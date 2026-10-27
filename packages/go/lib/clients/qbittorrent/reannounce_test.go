package qbittorrent

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestReannounce(t *testing.T) {
	var gotHashes, gotType string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathTorrentsReannounce {
			t.Errorf("path = %s, want %s", r.URL.Path, pathTorrentsReannounce)
		}
		gotType = r.Header.Get("Content-Type")
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		gotHashes = r.PostForm.Get("hashes")
		_, _ = io.WriteString(w, "Ok.")
	}))

	if err := c.Reannounce(context.Background(), []string{"aaaa", "bbbb"}); err != nil {
		t.Fatalf("Reannounce() error = %v", err)
	}

	// Form-encoded, not JSON: the WebUI API predates JSON bodies and
	// answers success while doing nothing when handed one.
	if gotType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", gotType)
	}
	if gotHashes != "aaaa|bbbb" {
		t.Errorf("hashes = %q, want aaaa|bbbb", gotHashes)
	}
}

func TestReannounceNoHashesCallsNothing(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the WebUI should not have been called")
	}))

	if err := c.Reannounce(context.Background(), nil); err != nil {
		t.Fatalf("Reannounce() error = %v", err)
	}
}

func TestReannounceReportsServerFailure(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "Forbidden")
	}))

	if err := c.Reannounce(context.Background(), []string{"aaaa"}); err == nil {
		t.Fatal("Reannounce() error = nil, want a failure")
	}
}

// An empty filter must not put "filter=" on the query: qBittorrent treats an
// unrecognised filter value as a mistake rather than as "everything".
func TestTorrentsFiltered(t *testing.T) {
	cases := []struct {
		filter string
		want   string
	}{
		{"", ""},
		{FilterActive, "filter=active"},
	}

	for _, tc := range cases {
		t.Run(tc.filter, func(t *testing.T) {
			var gotQuery string
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				_, _ = io.WriteString(w, `[{"hash":"aaaa","upspeed":4096}]`)
			}))

			got, err := c.TorrentsFiltered(context.Background(), tc.filter)
			if err != nil {
				t.Fatalf("TorrentsFiltered() error = %v", err)
			}
			if gotQuery != tc.want {
				t.Errorf("query = %q, want %q", gotQuery, tc.want)
			}
			if len(got) != 1 || got[0].UpSpeed != 4096 {
				t.Errorf("torrents = %+v, want one carrying upspeed 4096", got)
			}
		})
	}
}
