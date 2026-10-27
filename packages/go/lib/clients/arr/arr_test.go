package arr_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"homelab/clients/arr"
	"homelab/httpx"
)

func newClient(t *testing.T, kind arr.Kind, h http.Handler) *arr.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := arr.New(kind, httpx.Config{BaseURL: srv.URL, RetryMax: -1}, "key")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// Sonarr and Lidarr honour exactly one parent id and silently ignore any
// others, so every parent needs its own request. The fake here mimics that:
// it answers for the first id only, which is what makes a batched
// implementation return a plausible-looking undercount instead of an error.
//
// This is not hypothetical. Batching passed review, deployed, and reported 44
// files across 939 series.
func TestFilesRequestsOneParentEach(t *testing.T) {
	t.Parallel()

	var urls []string
	c := newClient(t, arr.Sonarr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urls = append(urls, r.URL.RequestURI())
		if len(r.URL.RequestURI()) > 4096 {
			w.WriteHeader(http.StatusRequestURITooLong)
			return
		}
		// Answer for the first seriesId only, as Sonarr does.
		ids := r.URL.Query()["seriesId"]
		if len(ids) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`[{"id":1,"path":"/x.mkv","seriesId":` + ids[0] + `}]`))
	}))

	ids := make([]int64, 939)
	for i := range ids {
		ids[i] = int64(i + 1)
	}

	files, err := c.Files(t.Context(), ids)
	if err != nil {
		t.Fatalf("Files: %v", err)
	}

	if len(urls) != len(ids) {
		t.Errorf("made %d requests for %d parents, want one each", len(urls), len(ids))
	}
	for _, u := range urls {
		if n := strings.Count(u, "seriesId="); n != 1 {
			t.Errorf("request carries %d ids, want exactly 1: %s", n, u)
		}
	}
	if len(files) != len(ids) {
		t.Errorf("got %d files for %d parents, want one each", len(files), len(ids))
	}
}

// Every app needs its own filter key, and getting one wrong is a 400 that only
// shows up against a live instance:
//
//	GET /api/v3/moviefile -> 400 "movieId or movieFileIds must be provided"
func TestFilesFilterKeyPerKind(t *testing.T) {
	t.Parallel()

	for kind, want := range map[arr.Kind]struct{ collection, param string }{
		arr.Sonarr: {"episodefile", "seriesId"},
		arr.Radarr: {"moviefile", "movieId"},
		arr.Lidarr: {"trackfile", "artistId"},
	} {
		var got string
		c := newClient(t, kind, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.URL.RequestURI()
			if r.URL.Query().Get(want.param) == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`[]`))
		}))

		if _, err := c.Files(t.Context(), []int64{7}); err != nil {
			t.Fatalf("%s Files: %v", kind, err)
		}
		if !strings.Contains(got, want.collection) {
			t.Errorf("%s used the wrong collection: %s", kind, got)
		}
		if !strings.Contains(got, want.param+"=7") {
			t.Errorf("%s missing %s filter: %s", kind, want.param, got)
		}
	}
}

// Lidarr is v1 while the others are v3, and the wrong version 404s rather than
// failing in any way that names the problem.
func TestAPIVersionPerKind(t *testing.T) {
	t.Parallel()

	for kind, want := range map[arr.Kind]string{
		arr.Sonarr: "/api/v3/series",
		arr.Radarr: "/api/v3/movie",
		arr.Lidarr: "/api/v1/artist",
	} {
		var got string
		c := newClient(t, kind, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.URL.Path
			_, _ = w.Write([]byte(`[]`))
		}))
		if _, err := c.Parents(t.Context()); err != nil {
			t.Fatalf("%s Parents: %v", kind, err)
		}
		if got != want {
			t.Errorf("%s parents path = %q, want %q", kind, got, want)
		}
	}
}
