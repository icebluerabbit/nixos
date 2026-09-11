package server_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"homelab/clients/stashbox"
	"homelab/secret"
	"homelab/stashtrap/internal/policy"
	"homelab/stashtrap/internal/server"
	"homelab/stashtrap/internal/store"
	"homelab/stashtrap/internal/sync"
)

// syncHarness wires a server to a Syncer backed by a stub stash-box, so the
// endpoint is exercised end to end rather than against a fake syncer.
func syncHarness(t *testing.T, block chan struct{}) http.Handler {
	t.Helper()

	stash := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if block != nil {
			<-block
		}
		var body struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case contains(body.Query, "queryPerformers"):
			io.WriteString(w, `{"data":{"queryPerformers":{"count":0,"performers":[]}}}`) //nolint:errcheck // test stub
		case contains(body.Query, "queryStudios"):
			io.WriteString(w, `{"data":{"queryStudios":{"count":0,"studios":[]}}}`) //nolint:errcheck // test stub
		default:
			io.WriteString(w, `{"data":{"queryScenes":{"count":0,"scenes":[]}}}`) //nolint:errcheck // test stub
		}
	}))
	t.Cleanup(stash.Close)

	client, err := stashbox.New(stashbox.Config{
		Endpoint:    stash.URL,
		APIKey:      secret.New("k"),
		MinInterval: time.Microsecond,
	})
	if err != nil {
		t.Fatalf("stashbox.New: %v", err)
	}
	t.Cleanup(client.Close)

	st, err := store.Open(filepath.Join(t.TempDir(), "stashtrap.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	syncer, err := sync.New(sync.Deps{
		Client:   client,
		Store:    st,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Endpoint: stash.URL,
	})
	if err != nil {
		t.Fatalf("sync.New: %v", err)
	}

	srv, err := server.New(server.Deps{
		Store:   st,
		Decider: policy.New(),
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Syncer:  syncer,
	}, prometheus.NewRegistry())
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	return srv.Handler()
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// The timer's curl must not wait for a crawl that runs for minutes.
func TestSyncReturnsImmediately(t *testing.T) {
	t.Parallel()

	h := syncHarness(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/sync", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
}

// A second timer tick while a fill is running is refused rather than queued:
// two crawls would race the same tables and share one rate limit.
func TestSyncRejectsConcurrent(t *testing.T) {
	t.Parallel()

	block := make(chan struct{})
	h := syncHarness(t, block)

	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/sync", http.NoBody))
	if first.Code != http.StatusAccepted {
		t.Fatalf("first status = %d", first.Code)
	}

	// The crawl is parked inside the stub until block closes.
	deadline := time.Now().Add(2 * time.Second)
	var second *httptest.ResponseRecorder
	for time.Now().Before(deadline) {
		second = httptest.NewRecorder()
		h.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/sync", http.NoBody))
		if second.Code == http.StatusConflict {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(block)

	if second == nil || second.Code != http.StatusConflict {
		t.Fatalf("second status = %v, want 409", second.Code)
	}
}

// Without a key there is no syncer, and the route must not exist at all —
// a 404 says "not configured" where a 500 would say "broken".
func TestSyncAbsentWithoutSyncer(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/sync", http.NoBody))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
