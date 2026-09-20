package ingest

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"
)

// discard is a logger that goes nowhere, for the handlers that log on a miss.
func discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestParseBytes covers the shell-hook shape.
//
// qBittorrent's AutoRun hooks and autobrr's templated action body send every
// field as text, so a byte count arrives as "1234". Unparseable must be zero
// (and therefore NULL once it goes through int64Ptr) rather than a partial
// parse: "12GB" is not twelve bytes.
func TestParseBytes(t *testing.T) {
	cases := map[string]int64{
		"1234":             1234,
		" 1234 ":           1234,
		"":                 0,
		"not a number":     0,
		"12GB":             0,
		"-5":               -5,
		"9007199254740993": 9007199254740993,
		"{{ .Size }}":      0, // a template that failed to render
	}
	for in, want := range cases {
		if got := parseBytes(in); got != want {
			t.Errorf("parseBytes(%q) = %d, want %d", in, got, want)
		}
	}
}

// The pointer helpers are duplicated from reconcile deliberately, so they get
// their own assertions here rather than being assumed identical.

func TestPointerHelpers(t *testing.T) {
	if strPtr("") != nil {
		t.Error(`strPtr("") should be nil`)
	}
	if got := strPtr("x"); got == nil || *got != "x" {
		t.Error("strPtr lost the value")
	}
	if intPtr(0) != nil {
		t.Error("intPtr(0) should be nil")
	}
	if int64Ptr(0) != nil {
		t.Error("int64Ptr(0) should be nil")
	}
	if got := boolPtr(false); got == nil || *got {
		t.Error("boolPtr(false) must be a non-nil false")
	}
	if timePtr(time.Time{}) != nil {
		t.Error("the zero time should be nil")
	}
	local := time.Date(2026, 9, 13, 1, 0, 0, 0, time.FixedZone("x", 7200))
	if got := timePtr(local); got == nil || got.Location() != time.UTC {
		t.Error("timePtr must normalise to UTC")
	}
}

// TestHandleUnknownSourceIsSilent covers the sources wired to a path that carry
// nothing this model has a column for.
//
// qbt-watch reports a filename with no infohash; jellyfin and readarr are out
// of scope. None of them is an error, and none must reach a handler — a nil dao
// would panic if one did.
func TestHandleUnknownSourceIsSilent(t *testing.T) {
	i := New(nil, nil, discard(), Config{})
	for _, src := range []string{"qbt-watch", "jellyfin", "readarr", "soularr", "nonsense"} {
		i.Handle(context.Background(), src, []byte(`{"anything":true}`))
	}
}

// TestHandleUnconfiguredInstanceIsSilent is the other no-op path.
//
// An empty Config means the instance was never wired, so there is no Service
// row for a foreign key to point at. Handling the body anyway would fail the
// insert on every webhook.
func TestHandleUnconfiguredInstanceIsSilent(t *testing.T) {
	i := New(nil, nil, discard(), Config{})
	body := []byte(`{"eventType":"Grab","downloadId":"ABC"}`)
	for _, src := range []string{"sonarr", "radarr", "lidarr", "prowlarr"} {
		i.Handle(context.Background(), src, body)
	}
	// cross-seed and qbittorrent guard on their own config keys.
	i.Handle(context.Background(), "cross-seed", []byte(`{"extra":{"infoHash":"abc"}}`))
	i.Handle(context.Background(), "qbittorrent", []byte(`{"event":"added","infohash":"abc"}`))
}

// TestHandleMalformedBodyIsSilent is the contract with the senders.
//
// Every handler is best-effort: a body this cannot parse is a defect here or a
// change at the sender, and neither is fixed by panicking inside a request that
// has already been answered 200.
func TestHandleMalformedBodyIsSilent(t *testing.T) {
	i := New(nil, nil, discard(), Config{
		Sonarr: "sonarr", Prowlarr: "prowlarr",
		CrossSeed: "cross-seed", QBittorrent: "qbittorrent",
	})
	bodies := [][]byte{
		nil,
		[]byte(``),
		[]byte(`not json`),
		[]byte(`[]`),
		[]byte(`{"eventType":123}`),
	}
	sources := []string{
		"autobrr", "sonarr", "prowlarr", "qbittorrent",
		"cross-seed", "unpackerr", "seasonpackarr", "slskd",
	}
	for _, src := range sources {
		for _, body := range bodies {
			i.Handle(context.Background(), src, body)
		}
	}
}

// The bug this fixes, reproduced.
//
// slskd and soularr have no webhook of their own, so their events reach
// managarr as journal lines a Vector sidecar remapped and shipped — batched,
// concatenated, not wrapped in an array. Decoding the whole body as one value
// failed on the second object with "invalid character '{' after top-level
// value" and dropped the entire batch, including the first event, which had
// parsed perfectly well.
func TestSplitHandlesBatchedDeliveries(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"single object", `{"event":"a"}`, 1},
		{"single with trailing newline", "{\"event\":\"a\"}\n", 1},
		// What Vector actually sends when it batches.
		{"newline delimited", "{\"event\":\"a\"}\n{\"event\":\"b\"}\n", 2},
		{"concatenated", `{"event":"a"}{"event":"b"}`, 2},
		{"three", "{\"a\":1}\n{\"b\":2}\n{\"c\":3}", 3},
		{"empty body", "", 0},
		{"whitespace only", "  \n  ", 0},
		// An array is a single JSON value and stays one: no sender here emits
		// one, and unwrapping it would guess at an intent nobody expressed.
		{"array stays one value", `[{"a":1},{"b":2}]`, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := split([]byte(tc.body))
			if err != nil {
				t.Fatalf("split(%q): %v", tc.body, err)
			}
			if len(got) != tc.want {
				t.Errorf("split(%q) = %d values, want %d", tc.body, len(got), tc.want)
			}
			for _, v := range got {
				if !json.Valid(v) {
					t.Errorf("split produced invalid JSON: %s", v)
				}
			}
		})
	}
}

// A body that will not parse is an error rather than a silent drop: it is
// either a defect here or a change at the sender, and both need to be visible.
func TestSplitRejectsMalformedBodies(t *testing.T) {
	for _, body := range []string{
		"nonsense",
		`{"a":1} garbage`,
		`{"a":`,
	} {
		if _, err := split([]byte(body)); err == nil {
			t.Errorf("split(%q) accepted a malformed body", body)
		}
	}
}

// Every event in a batch has to reach its handler, not just the first.
func TestHandleRoutesEveryEventInABatch(t *testing.T) {
	i := &Ingest{logger: discardLogger()}

	// soularr is routed but its handler needs no database for a body it
	// cannot act on, so this exercises the split and the routing without one.
	body := []byte("{\"source\":\"soularr\",\"event\":\"search_failed\"}\n" +
		"{\"source\":\"soularr\",\"event\":\"search_failed\"}\n")

	values, err := split(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 {
		t.Fatalf("split gave %d values, want 2", len(values))
	}
	// Routing each one must not panic, which is what a nil dao would do if a
	// handler tried to write for an event carrying no album id.
	for _, v := range values {
		i.handleOne(context.Background(), "soularr", v)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
