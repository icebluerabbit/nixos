package provision

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fastOptions keep the polling behaviour but not its wall clock: the real
// defaults wait two minutes, which is right in a unit and wrong in a test.
func fastOptions(client *http.Client) Options {
	return Options{Attempts: 3, Interval: time.Millisecond, Client: client}
}

// quiStub is a stand-in for qui's instance API. It records what was posted.
type quiStub struct {
	mu        sync.Mutex
	instances []map[string]any
	posted    []map[string]any
	// listFailures is how many GETs fail before the listener "comes up".
	listFailures int
	// createStatus overrides the response to a POST.
	createStatus int
	createBody   string
}

func (q *quiStub) handler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(instancesPath, func(w http.ResponseWriter, r *http.Request) {
		q.mu.Lock()
		defer q.mu.Unlock()

		switch r.Method {
		case http.MethodGet:
			if q.listFailures > 0 {
				q.listFailures--
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(q.instances); err != nil {
				t.Errorf("encode instances: %v", err)
			}
		case http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode post body: %v", err)
			}
			q.posted = append(q.posted, body)
			if q.createStatus != 0 {
				w.WriteHeader(q.createStatus)
				_, _ = io.WriteString(w, q.createBody)
				return
			}
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func (q *quiStub) postCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.posted)
}

// serve starts the stub and returns its address.
func serve(t *testing.T, q *quiStub) string {
	t.Helper()
	srv := httptest.NewServer(q.handler(t))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestRunRegistersTheInstance(t *testing.T) {
	q := &quiStub{}
	api := serve(t, q)

	var out bytes.Buffer
	cfg := Config{APIBase: api, QbtURL: "http://10.10.130.5:8080", Name: "qbittorrent"}
	if err := Run(context.Background(), &out, cfg, fastOptions(nil)); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if q.postCount() != 1 {
		t.Fatalf("posted %d instances, want 1", q.postCount())
	}
	posted := q.posted[0]
	if posted["name"] != "qbittorrent" || posted["host"] != "http://10.10.130.5:8080" {
		t.Errorf("posted = %v", posted)
	}
	// qBittorrent trusts this subnet, so the instance carries no credentials.
	if posted["username"] != "" || posted["password"] != "" {
		t.Errorf("credentials were sent: %v", posted)
	}
	if !strings.Contains(out.String(), "registered 'qbittorrent'") {
		t.Errorf("output = %q", out.String())
	}
}

// The unit runs on every start, so a host already registered must not be
// registered twice.
func TestRunIsIdempotentOnHost(t *testing.T) {
	q := &quiStub{instances: []map[string]any{
		{"host": "http://10.10.130.5:8080", "name": "whatever it was called"},
	}}
	api := serve(t, q)

	var out bytes.Buffer
	cfg := Config{APIBase: api, QbtURL: "http://10.10.130.5:8080", Name: "qbittorrent"}
	if err := Run(context.Background(), &out, cfg, fastOptions(nil)); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if q.postCount() != 0 {
		t.Errorf("posted %d instances, want none", q.postCount())
	}
	if !strings.Contains(out.String(), "already present") {
		t.Errorf("output = %q", out.String())
	}
}

// Identity is the host, not the name: a different name for the same
// qBittorrent is still the same instance.
func TestRunMatchesOnHostNotName(t *testing.T) {
	q := &quiStub{instances: []map[string]any{
		{"host": "http://other:8080", "name": "qbittorrent"},
	}}
	api := serve(t, q)

	cfg := Config{APIBase: api, QbtURL: "http://10.10.130.5:8080", Name: "qbittorrent"}
	if err := Run(context.Background(), io.Discard, cfg, fastOptions(nil)); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if q.postCount() != 1 {
		t.Errorf("a different host should have been registered, posts = %d", q.postCount())
	}
}

// qui.service is Type=simple, so the listener is routinely not up when this
// unit starts. That is the normal case, not a fault.
func TestRunWaitsForTheListener(t *testing.T) {
	q := &quiStub{listFailures: 2}
	api := serve(t, q)

	cfg := Config{APIBase: api, QbtURL: "http://10.10.130.5:8080", Name: "qbittorrent"}
	if err := Run(context.Background(), io.Discard, cfg, fastOptions(nil)); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if q.postCount() != 1 {
		t.Errorf("posted %d instances after the listener came up, want 1", q.postCount())
	}
}

// Fail-open: an API that never answers must not wedge the container.
func TestRunGivesUpOpen(t *testing.T) {
	var out bytes.Buffer
	cfg := Config{APIBase: "http://127.0.0.1:1", QbtURL: "http://qbt:8080", Name: "qbittorrent"}

	if err := Run(context.Background(), &out, cfg, fastOptions(nil)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "never became ready") {
		t.Errorf("output = %q", out.String())
	}
}

// Same when qui is up but keeps refusing the listing.
func TestRunGivesUpOpenOnPersistentRefusal(t *testing.T) {
	q := &quiStub{listFailures: 1000}
	api := serve(t, q)

	var out bytes.Buffer
	cfg := Config{APIBase: api, QbtURL: "http://qbt:8080", Name: "qbittorrent"}
	if err := Run(context.Background(), &out, cfg, fastOptions(nil)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "never became ready") {
		t.Errorf("output = %q", out.String())
	}
}

// A refused create is reported with its status and body, and tolerated: qui
// is up and said no, which is not something to wedge a container over.
func TestRunToleratesRefusedCreate(t *testing.T) {
	q := &quiStub{createStatus: http.StatusConflict, createBody: `{"error":"exists"}`}
	api := serve(t, q)

	var out bytes.Buffer
	cfg := Config{APIBase: api, QbtURL: "http://qbt:8080", Name: "qbittorrent"}
	if err := Run(context.Background(), &out, cfg, fastOptions(nil)); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "create failed HTTP 409") || !strings.Contains(got, "exists") {
		t.Errorf("output = %q", got)
	}
}

// A connection that breaks after qui has already answered is a real fault:
// retrying the same call would not fix it, so it is reported.
func TestRunReportsTransportFailureAfterListing(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, "[]")
			return
		}
		// The listing succeeded; now the listener goes away mid-run.
		srv.CloseClientConnections()
	}))
	t.Cleanup(srv.Close)

	cfg := Config{APIBase: srv.URL, QbtURL: "http://qbt:8080", Name: "qbittorrent"}
	err := Run(context.Background(), io.Discard, cfg, fastOptions(nil))

	if err == nil {
		t.Fatal("Run tolerated a transport failure on create")
	}
}

// A cancelled context stops the wait rather than sitting out the full
// retry budget.
func TestRunStopsWaitingWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	cfg := Config{APIBase: "http://127.0.0.1:1", QbtURL: "http://qbt:8080", Name: "qbittorrent"}

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, &out, cfg, Options{Attempts: 1000, Interval: time.Hour})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run: %v", err)
		}
		if !strings.Contains(out.String(), "never became ready") {
			t.Errorf("output = %q", out.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run ignored its cancelled context")
	}
}

// The defaults are what the unit runs with, so they are asserted rather than
// left to whatever Options happens to be zero-valued.
func TestOptionDefaults(t *testing.T) {
	got := Options{}.withDefaults()

	if got.Attempts != DefaultAttempts || got.Interval != DefaultInterval {
		t.Errorf("polling = %d x %v, want %d x %v",
			got.Attempts, got.Interval, DefaultAttempts, DefaultInterval)
	}
	if got.Client == nil || got.Client.Timeout != DefaultTimeout {
		t.Errorf("client timeout = %v, want %v", got.Client, DefaultTimeout)
	}
}

// An empty list is a valid answer, not a reason to keep waiting.
func TestRunTreatsEmptyListAsReady(t *testing.T) {
	q := &quiStub{instances: []map[string]any{}}
	api := serve(t, q)

	cfg := Config{APIBase: api, QbtURL: "http://qbt:8080", Name: "qbittorrent"}
	if err := Run(context.Background(), io.Discard, cfg, fastOptions(nil)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if q.postCount() != 1 {
		t.Errorf("posted %d instances, want 1", q.postCount())
	}
}
