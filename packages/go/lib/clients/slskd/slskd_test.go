package slskd_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"homelab/clients/slskd"
	"homelab/httpx"
)

// listBody is a trimmed capture of a real /api/v0/searches response. The
// seven-digit fraction on the timestamps is what slskd actually sends, and is
// the reason StartedAt is decoded rather than compared as a string.
const listBody = `[
  {"endedAt":"2026-07-28T06:01:00.3195054Z","fileCount":8591,
   "id":"e14070fe-ffaf-4afb-86e2-0e0b605acdf9","isComplete":true,
   "responseCount":250,"responses":[],"searchText":"aespa",
   "startedAt":"2026-07-28T06:00:58.4012303Z","token":48,
   "state":"Completed, ResponseLimitReached"},
  {"endedAt":"2026-09-08T21:19:17.0000000Z","fileCount":8,
   "id":"0374bfde-8af8-11f1-a737-62f5a45b2089","isComplete":true,
   "responseCount":3,"responses":[],"searchText":"Twisted Nails of Faith",
   "startedAt":"2026-09-08T21:19:10.0000000Z","token":49,
   "state":"Completed, TimedOut"}
]`

func newClient(t *testing.T, handler http.Handler) *slskd.Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := slskd.New(httpx.Config{BaseURL: srv.URL, RetryMax: -1})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestSearchesDecodesHistory(t *testing.T) {
	t.Parallel()

	var seenPath string
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(listBody))
	}))

	got, err := c.Searches(context.Background())
	if err != nil {
		t.Fatalf("Searches: %v", err)
	}

	if seenPath != "/api/v0/searches" {
		t.Errorf("path = %q, want /api/v0/searches", seenPath)
	}

	want := []slskd.Search{
		{
			ID:            "e14070fe-ffaf-4afb-86e2-0e0b605acdf9",
			SearchText:    "aespa",
			StartedAt:     time.Date(2026, 7, 28, 6, 0, 58, 401230300, time.UTC),
			EndedAt:       time.Date(2026, 7, 28, 6, 1, 0, 319505400, time.UTC),
			IsComplete:    true,
			FileCount:     8591,
			ResponseCount: 250,
			State:         "Completed, ResponseLimitReached",
		},
		{
			ID:            "0374bfde-8af8-11f1-a737-62f5a45b2089",
			SearchText:    "Twisted Nails of Faith",
			StartedAt:     time.Date(2026, 9, 8, 21, 19, 10, 0, time.UTC),
			EndedAt:       time.Date(2026, 9, 8, 21, 19, 17, 0, time.UTC),
			IsComplete:    true,
			FileCount:     8,
			ResponseCount: 3,
			State:         "Completed, TimedOut",
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Searches (-want +got):\n%s", diff)
	}
}

// A non-retryable status comes back as a *httpx.StatusError, which is the
// contract callers discriminate on. 404 rather than 500 on purpose: httpx
// retries 5xx and reports the exhausted sequence as a transport error instead.
func TestSearchesReportsStatus(t *testing.T) {
	t.Parallel()

	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))

	_, err := c.Searches(context.Background())
	var se *httpx.StatusError
	if !errors.As(err, &se) {
		t.Fatalf("err = %v, want a *httpx.StatusError", err)
	}
	if se.Code != http.StatusNotFound {
		t.Errorf("Code = %d, want 404", se.Code)
	}
	if !httpx.IsNotFound(err) {
		t.Error("IsNotFound = false, want true")
	}
}

func TestDeleteSearchAddressesTheID(t *testing.T) {
	t.Parallel()

	var seenMethod, seenPath string
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod, seenPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))

	if err := c.DeleteSearch(context.Background(), "e14070fe-ffaf"); err != nil {
		t.Fatalf("DeleteSearch: %v", err)
	}
	if seenMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", seenMethod)
	}
	// Encoded exactly once — a pre-escaped id would arrive as %2D-mangled.
	if want := "/api/v0/searches/e14070fe-ffaf"; seenPath != want {
		t.Errorf("path = %q, want %q", seenPath, want)
	}
}

func TestDeleteSearchRejectsEmptyID(t *testing.T) {
	t.Parallel()

	called := false
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	if err := c.DeleteSearch(context.Background(), ""); !errors.Is(err, slskd.ErrNoSearchID) {
		t.Errorf("err = %v, want ErrNoSearchID", err)
	}
	if called {
		t.Error("an empty id reached the server")
	}
}

func TestNewRejectsEmptyBaseURL(t *testing.T) {
	t.Parallel()

	if _, err := slskd.New(httpx.Config{}); !errors.Is(err, httpx.ErrNoBaseURL) {
		t.Errorf("err = %v, want ErrNoBaseURL", err)
	}
}
