package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"homelab/app"
	"homelab/httpx"
)

// newClient points a Client at a handler.
func newClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := NewClient(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// pagedChanges serves n synthetic events with real keyset paging, so the walk
// runs against the same semantics a sidecar implements rather than a handler
// that ignores the cursor.
func pagedChanges(t *testing.T, n int, queries *[]string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if queries != nil {
			*queries = append(*queries, r.URL.RawQuery)
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

		start := 0
		if since := r.URL.Query().Get("since"); since != "" {
			i, err := strconv.Atoi(since)
			if err != nil {
				http.Error(w, "bad cursor", http.StatusBadRequest)
				return
			}
			start = i + 1
		}

		events := []Event{}
		for i := start; i < start+limit && i < n; i++ {
			events = append(events, Event{
				Source: "test", Kind: "thing",
				Ref:    fmt.Sprintf("item-%d", i),
				Cursor: strconv.Itoa(i),
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"events": events})
	}
}

func TestClientInfo(t *testing.T) {
	var gotPath string
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"sidecar":"managarr-crossseed","source":"cross-seed",
			"upstream":"/var/lib/cross-seed/cross-seed.db","capabilities":["changes"]}`))
	})

	got, err := c.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/info" {
		t.Errorf("path = %q, want /info", gotPath)
	}
	if got.Sidecar != "managarr-crossseed" || got.Source != "cross-seed" {
		t.Errorf("info = %+v", got)
	}
	if len(got.Capabilities) != 1 || got.Capabilities[0] != CapChanges {
		t.Errorf("capabilities = %v", got.Capabilities)
	}
}

func TestClientInfoError(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := c.Info(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}

func TestClientChangesPages(t *testing.T) {
	var queries []string
	c := newClient(t, pagedChanges(t, 25, &queries))

	var pages [][]Event
	cursor, err := c.Changes(context.Background(), "", 10, func(page []Event) bool {
		pages = append(pages, page)
		return true
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(pages) != 3 {
		t.Fatalf("got %d pages, want 3", len(pages))
	}
	if len(pages[0]) != 10 || len(pages[1]) != 10 || len(pages[2]) != 5 {
		t.Errorf("page sizes = %d/%d/%d, want 10/10/5",
			len(pages[0]), len(pages[1]), len(pages[2]))
	}

	// The cursor must come from the last event returned, not from the
	// requested page size. Advancing by the limit would skip rows whenever
	// a page came back short.
	if cursor != "24" {
		t.Errorf("resume cursor = %q, want 24", cursor)
	}

	// First request carries no since at all — an empty cursor means the
	// beginning, and sending since= would make a strict sidecar parse "".
	if strings.Contains(queries[0], "since") {
		t.Errorf("first query = %q, want no since", queries[0])
	}
	if !strings.Contains(queries[1], "since=9") {
		t.Errorf("second query = %q, want since=9", queries[1])
	}

	// A short final page ends the walk. An extra round trip would be racing
	// whatever the sidecar wrote since the walk began.
	if len(queries) != 3 {
		t.Errorf("made %d requests, want 3", len(queries))
	}
}

// An exact multiple has no short page to stop on, so the walk only ends when it
// sees an empty one.
func TestClientChangesExactMultiple(t *testing.T) {
	var queries []string
	c := newClient(t, pagedChanges(t, 20, &queries))

	var total int
	cursor, err := c.Changes(context.Background(), "", 10, func(page []Event) bool {
		total += len(page)
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 20 {
		t.Errorf("walked %d events, want 20", total)
	}
	if cursor != "19" {
		t.Errorf("cursor = %q, want 19", cursor)
	}
	if len(queries) != 3 {
		t.Errorf("made %d requests, want 3 (the third is the empty page)", len(queries))
	}
}

// Resuming must not re-read what the caller already has.
func TestClientChangesResumes(t *testing.T) {
	c := newClient(t, pagedChanges(t, 25, nil))

	var got []Event
	if _, err := c.Changes(context.Background(), "19", 10, func(page []Event) bool {
		got = append(got, page...)
		return true
	}); err != nil {
		t.Fatal(err)
	}

	if len(got) != 5 {
		t.Fatalf("got %d events, want the 5 after the cursor", len(got))
	}
	if got[0].Ref != "item-20" {
		t.Errorf("first event = %q, want item-20", got[0].Ref)
	}
}

// Stopping early must still report where to resume, or the caller re-reads the
// page it just stopped on.
func TestClientChangesStopReturnsCursor(t *testing.T) {
	var queries []string
	c := newClient(t, pagedChanges(t, 100, &queries))

	cursor, err := c.Changes(context.Background(), "", 10, func([]Event) bool {
		return false
	})
	if err != nil {
		t.Fatal(err)
	}
	if cursor != "9" {
		t.Errorf("cursor = %q, want 9", cursor)
	}
	if len(queries) != 1 {
		t.Errorf("made %d requests after stopping, want 1", len(queries))
	}
}

// Nothing new is not an error, and the cursor handed back must be the one
// passed in — otherwise an idle poll would rewind to the beginning.
func TestClientChangesEmpty(t *testing.T) {
	called := false
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"events":[]}`))
	})

	cursor, err := c.Changes(context.Background(), "abc", 10, func([]Event) bool {
		called = true
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("callback ran on an empty page")
	}
	if cursor != "abc" {
		t.Errorf("cursor = %q, want the one passed in", cursor)
	}
}

// A sidecar returning events without cursors cannot be paged. Stopping and
// saying so beats re-fetching the same page forever.
func TestClientChangesMissingCursor(t *testing.T) {
	requests := 0
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"events":[{"source":"test","kind":"thing"}]}`))
	})

	_, err := c.Changes(context.Background(), "", 10, func([]Event) bool { return true })
	if err == nil {
		t.Fatal("expected an error for a page with no cursor")
	}
	if requests != 1 {
		t.Errorf("made %d requests, want 1 — it must not loop", requests)
	}
}

// A sidecar whose cursor never advances would otherwise walk forever.
func TestClientChangesPageLimit(t *testing.T) {
	requests := 0
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"events":[{"kind":"thing","cursor":"stuck"}]}`))
	})

	_, err := c.Changes(context.Background(), "", 1, func([]Event) bool { return true })
	if err == nil {
		t.Fatal("expected an error once the page bound was hit")
	}
	if requests != maxChangePages {
		t.Errorf("made %d requests, want %d", requests, maxChangePages)
	}
}

func TestClientChangesDefaultPageSize(t *testing.T) {
	var queries []string
	c := newClient(t, pagedChanges(t, 1, &queries))

	if _, err := c.Changes(context.Background(), "", 0, func([]Event) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(queries[0], fmt.Sprintf("limit=%d", defaultChangesLimit)) {
		t.Errorf("query = %q, want the %d default", queries[0], defaultChangesLimit)
	}
}

// A failed page returns the cursor it started from, so a caller that stores the
// result does not skip what it never read.
func TestClientChangesErrorKeepsCursor(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	cursor, err := c.Changes(context.Background(), "abc", 10, func([]Event) bool { return true })
	if err == nil {
		t.Fatal("expected an error")
	}
	if cursor != "abc" {
		t.Errorf("cursor = %q, want the one passed in", cursor)
	}
}

func TestClientGet(t *testing.T) {
	var gotPath string
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"clientTorrents":[{"name":"A","infoHash":"aaa"}]}`))
	})

	var body struct {
		ClientTorrents []struct {
			Name     string `json:"name"`
			InfoHash string `json:"infoHash"`
		} `json:"clientTorrents"`
	}
	if err := c.Get(context.Background(), "/client-torrents", &body); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/client-torrents" {
		t.Errorf("path = %q", gotPath)
	}
	if len(body.ClientTorrents) != 1 || body.ClientTorrents[0].InfoHash != "aaa" {
		t.Errorf("body = %+v", body.ClientTorrents)
	}
}

func TestClientGetError(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	var out map[string]any
	if err := c.Get(context.Background(), "/nope", &out); err == nil {
		t.Fatal("expected an error")
	}
}

func TestNewClientRejectsBadConfig(t *testing.T) {
	if _, err := NewClient(httpx.Config{BaseURL: ""}); err == nil {
		t.Fatal("expected an error for an empty base URL")
	}
}

type decision struct {
	SearcheeName string `json:"searcheeName"`
	Decision     string `json:"decision"`
}

func TestDecodePayload(t *testing.T) {
	e := Event{
		Source:  "cross-seed",
		Kind:    "decision",
		Payload: json.RawMessage(`{"searcheeName":"Release.A","decision":"MATCH"}`),
	}

	got, err := DecodePayload[decision](e)
	if err != nil {
		t.Fatal(err)
	}
	if got.SearcheeName != "Release.A" || got.Decision != "MATCH" {
		t.Errorf("payload = %+v", got)
	}
}

func TestDecodePayloadErrors(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		if _, err := DecodePayload[decision](Event{Source: "x", Kind: "y"}); err == nil {
			t.Fatal("expected an error for an absent payload")
		}
	})
	t.Run("malformed", func(t *testing.T) {
		e := Event{Source: "x", Kind: "y", Payload: json.RawMessage(`{"decision":5}`)}
		if _, err := DecodePayload[decision](e); err == nil {
			t.Fatal("expected an error for a type mismatch")
		}
	})
}

// Cancelling the context must bring the listener down and return cleanly.
// ErrServerClosed is the end of a graceful shutdown, not a failure, and
// returning it would make every deploy look like a crash in the journal.
func TestRunShutsDownOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := Defaults()
	// Port 0: the kernel picks a free one, so this never collides with
	// whatever else is listening on the build machine.
	cfg.Addr = "127.0.0.1:0"
	cfg.ShutdownTimeout = 2 * time.Second

	env := &app.Env{Logger: discardLogger()}
	done := make(chan error, 1)
	go func() { done <- Run(ctx, env, cfg, &fake{info: Info{Source: "test"}}) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v, want nil on a graceful shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
}

func TestRunReportsListenFailure(t *testing.T) {
	cfg := Defaults()
	cfg.Addr = "127.0.0.1:-1"

	err := Run(context.Background(), &app.Env{Logger: discardLogger()}, cfg,
		&fake{info: Info{Source: "test"}})
	if err == nil {
		t.Fatal("expected an error for an unbindable address")
	}
}

// A malformed managarr URL has to fail at startup. A sidecar that came up and
// silently dropped every webhook would look perfectly healthy.
func TestRunRejectsBadManagarrURL(t *testing.T) {
	cfg := Defaults()
	cfg.Addr = "127.0.0.1:0"
	cfg.ManagarrURL = "://not a url"

	err := Run(context.Background(), &app.Env{Logger: discardLogger()}, cfg,
		&fake{info: Info{Source: "test"}})
	if err == nil {
		t.Fatal("expected an error for an unparseable managarr URL")
	}
}
