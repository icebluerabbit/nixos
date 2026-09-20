package relaysvc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"homelab/sidecar"
)

func TestInfo(t *testing.T) {
	got := New("unpackerr", "path").Info()

	if got.Sidecar != "managarr-unpackerr" || got.Source != "unpackerr" {
		t.Errorf("info = %+v", got)
	}
	// Events only: there is nothing retrospective to walk.
	if len(got.Capabilities) != 1 || got.Capabilities[0] != sidecar.CapEvents {
		t.Errorf("capabilities = %v, want [events]", got.Capabilities)
	}
	// Upstream is empty, and honestly so: a relay reads nothing, so there is
	// no address it could be pointed at wrongly.
	if got.Upstream != "" {
		t.Errorf("upstream = %q, want empty", got.Upstream)
	}
}

func TestChangesUnsupported(t *testing.T) {
	_, err := New("soularr").Changes(context.Background(), "", 10)

	if !errors.Is(err, sidecar.ErrUnsupported) {
		t.Errorf("err = %v, want ErrUnsupported", err)
	}
}

// The body must survive byte for byte. A relay that re-encoded its payload
// would drop every field it does not know about, which is all of them.
func TestEnrichForwardsTheBodyUnchanged(t *testing.T) {
	const body = `{"path":"/mnt/x","app":"sonarr","event":"extracted",` +
		`"data":{"files":["a.mkv"],"bytes":123},"unknown_future_field":true}`

	events, err := New("unpackerr", "path").Enrich(context.Background(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if string(events[0].Payload) != body {
		t.Errorf("payload =\n%s\nwant\n%s", events[0].Payload, body)
	}
	if events[0].Source != "unpackerr" || events[0].Kind != KindWebhook {
		t.Errorf("event = %s/%s", events[0].Source, events[0].Kind)
	}
}

func TestEnrichPicksTheFirstPresentRefKey(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		body string
		want string
	}{
		{"first key wins", []string{"a", "b"}, `{"a":"one","b":"two"}`, "one"},
		{"falls through a missing key", []string{"a", "b"}, `{"b":"two"}`, "two"},
		{"falls through an empty value", []string{"a", "b"}, `{"a":"","b":"two"}`, "two"},
		{"no key matches", []string{"a"}, `{"z":"x"}`, ""},
		{"no keys configured", nil, `{"a":"one"}`, ""},
		// Only strings. An object or a number is not an identifier, and
		// coercing one would put something in Ref that matches nothing.
		{"ignores a non-string", []string{"a", "b"}, `{"a":{"n":1},"b":"two"}`, "two"},
		{"ignores a number", []string{"a", "b"}, `{"a":5,"b":"two"}`, "two"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			events, err := New("seasonpackarr", tc.keys...).
				Enrich(context.Background(), []byte(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			if events[0].Ref != tc.want {
				t.Errorf("ref = %q, want %q", events[0].Ref, tc.want)
			}
		})
	}
}

// A body with no identifier is still relayed. managarr can correlate from the
// payload; refusing it would lose the event entirely.
func TestEnrichRelaysWithoutARef(t *testing.T) {
	events, err := New("soularr", "album_id").
		Enrich(context.Background(), []byte(`{"event":"denylist"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Ref != "" {
		t.Errorf("events = %+v", events)
	}
}

// An unparseable body will fail on managarr's side too, and failing here says
// which service sent it.
func TestEnrichRejectsNonObjects(t *testing.T) {
	for name, body := range map[string]string{
		"not json": "nonsense",
		"an array": `[{"a":1}]`,
		"a string": `"hello"`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := New("unpackerr").Enrich(context.Background(), []byte(body))
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

// An empty object is a valid body — a notification whose fields were all empty
// is still a notification that fired.
func TestEnrichAcceptsAnEmptyObject(t *testing.T) {
	events, err := New("unpackerr", "path").Enrich(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if !json.Valid(events[0].Payload) {
		t.Error("payload is not valid JSON")
	}
}

func TestImplementsContract(t *testing.T) {
	var _ sidecar.Service = (*Service)(nil)
}
