package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) (*Server, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv, err := New(Deps{
		Logger: logger,
		Now:    func() time.Time { return time.Unix(0, 0).UTC() },
	}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv, &logs
}

// The corpus is the product, so the body has to survive the round trip
// byte-for-byte — including payloads that are not JSON at all, which is what
// qBittorrent's substituted token line is.
func TestRecordsBodyVerbatim(t *testing.T) {
	srv, logs := newTestServer(t)

	body := `{"eventType":"Download","downloadId":"ABC123","episodeFile":{"path":"/mnt/media/library/tv/x.mkv"}}`
	req := httptest.NewRequest(http.MethodPost, "/hook/sonarr", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var rec2 map[string]any
	if err := json.Unmarshal(firstLine(t, logs.Bytes()), &rec2); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	if got := rec2["body"]; got != body {
		t.Errorf("body = %q, want %q", got, body)
	}
	if got := rec2["source"]; got != "sonarr" {
		t.Errorf("source = %q, want sonarr", got)
	}
}

// Every sender must see a 2xx. A non-2xx makes Sonarr disable the connection
// and autobrr retry, and either one corrupts the corpus this phase exists to
// collect.
func TestAlwaysAcceptsKnownSources(t *testing.T) {
	srv, _ := newTestServer(t)

	for _, src := range Sources {
		// A body that is not JSON, because several senders do not send JSON.
		req := httptest.NewRequest(http.MethodPost, "/hook/"+src, strings.NewReader("not json at all"))
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", src, rec.Code)
		}
	}
}

// An unknown path is a misconfigured sender. It must fail at the sender rather
// than be silently absorbed, or the gap only shows up later as missing data.
func TestUnknownSourceIs404(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/hook/whoops", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// A body past the cap is truncated rather than refused: a recorded prefix is
// worth more than a retry storm.
func TestOversizeBodyIsTruncatedNotRejected(t *testing.T) {
	srv, logs := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/hook/radarr",
		io.LimitReader(neverEnding{}, maxBodyBytes+4096))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var line map[string]any
	if err := json.Unmarshal(firstLine(t, logs.Bytes()), &line); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	if got := line["body_bytes"].(float64); int(got) != maxBodyBytes {
		t.Errorf("body_bytes = %d, want %d", int(got), maxBodyBytes)
	}
}

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func firstLine(t *testing.T, b []byte) []byte {
	t.Helper()
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		return b[:i]
	}
	return b
}

type neverEnding struct{}

func (neverEnding) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

// The route the managarr-* sidecars relay to.
//
// One path for all thirteen: the envelope names its own source, which is the
// point of normalising at the edge, so a path per sender would be redundant.
func TestIngestSidecarRoutesByEnvelope(t *testing.T) {
	type got struct {
		source string
		body   string
	}
	var seen []got

	s := newServer(t, Deps{
		Logger: discardLogger(),
		Ingest: func(_ context.Context, source string, body []byte) {
			seen = append(seen, got{source, string(body)})
		},
	})

	body := `{"events":[
		{"source":"qbittorrent","kind":"completed","ref":"abc","payload":{"infohash":"abc"}},
		{"source":"sonarr","kind":"webhook","ref":"DEF","payload":{"eventType":"Grab"}}
	]}`
	rec := post(t, s, "/ingest/sidecar", body)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body)
	}
	if len(seen) != 2 {
		t.Fatalf("ingested %d events, want 2", len(seen))
	}
	if seen[0].source != "qbittorrent" || seen[1].source != "sonarr" {
		t.Errorf("sources = %q, %q", seen[0].source, seen[1].source)
	}
	// The payload is passed through untouched — the server reads the envelope
	// and nothing below it.
	if !strings.Contains(seen[0].body, `"infohash":"abc"`) {
		t.Errorf("payload was altered: %s", seen[0].body)
	}
}

// An event with no source cannot reach a handler. Skipped rather than failing
// the batch: the others in it are still good.
func TestIngestSidecarSkipsSourcelessEvents(t *testing.T) {
	var count int
	s := newServer(t, Deps{
		Logger: discardLogger(),
		Ingest: func(context.Context, string, []byte) { count++ },
	})

	rec := post(t, s, "/ingest/sidecar", `{"events":[
		{"kind":"completed","payload":{"a":1}},
		{"source":"slskd","kind":"event","payload":{"b":2}}
	]}`)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if count != 1 {
		t.Errorf("ingested %d events, want only the one with a source", count)
	}
}

// A body this cannot parse still answers 202, for the same reason every /hook/
// path answers 200: the sender is a sidecar that logs and moves on, the event
// is already gone, and retrying would only repeat the failure.
func TestIngestSidecarAcceptsUndecodableBodies(t *testing.T) {
	var count int
	s := newServer(t, Deps{
		Logger: discardLogger(),
		Ingest: func(context.Context, string, []byte) { count++ },
	})

	rec := post(t, s, "/ingest/sidecar", "not json")
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
	if count != 0 {
		t.Errorf("ingested %d events from an unparseable body", count)
	}
}

// An event carrying no payload has nothing to persist. Not an error — a
// sidecar may relay an envelope whose body it could not build.
func TestIngestSidecarSkipsEmptyPayloads(t *testing.T) {
	var count int
	s := newServer(t, Deps{
		Logger: discardLogger(),
		Ingest: func(context.Context, string, []byte) { count++ },
	})

	rec := post(t, s, "/ingest/sidecar", `{"events":[{"source":"beets","kind":"item"}]}`)
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
	if count != 0 {
		t.Errorf("ingested %d events with no payload", count)
	}
}

func TestIngestSidecarRejectsNonPost(t *testing.T) {
	s := newServer(t, Deps{Logger: discardLogger()})

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ingest/sidecar", http.NoBody))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// newServer builds a server with explicit deps, for the tests that need to
// observe what reached Ingest rather than what reached the log.
func newServer(t *testing.T, deps Deps) *Server {
	t.Helper()
	s, err := New(deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// post sends a body and returns the recorded response.
func post(t *testing.T, s *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	s.Handler().ServeHTTP(rec, r)
	return rec
}
