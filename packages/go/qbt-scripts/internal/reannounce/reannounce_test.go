package reannounce_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"homelab/app"
	"homelab/clients/qbittorrent"
	"homelab/qbt-scripts/internal/reannounce"
)

func testEnv(dryRun bool) *app.Env {
	return &app.Env{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Stdout: io.Discard,
		DryRun: dryRun,
	}
}

// torrent is one row of the fake client's list.
type torrent struct {
	hash     string
	progress float64
	trackers []qbittorrent.Tracker
}

// fake is a stand-in qBittorrent WebUI that records which torrents were
// re-announced, so a test asserts on the selection rather than on a count.
type fake struct {
	mu         sync.Mutex
	torrents   []torrent
	reannounce []string
	// listStatus, when non-zero, fails the torrent listing.
	listStatus int
	// trackerStatus, when non-zero, fails the tracker listing.
	trackerStatus int
}

// serve starts the fake and returns its base URL.
func (f *fake) serve(t *testing.T) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		switch r.URL.Path {
		case "/api/v2/torrents/info":
			if f.listStatus != 0 {
				w.WriteHeader(f.listStatus)
				return
			}
			rows := make([]map[string]any, 0, len(f.torrents))
			for _, tor := range f.torrents {
				rows = append(rows, map[string]any{
					"hash": tor.hash, "progress": tor.progress,
				})
			}
			writeJSON(t, w, rows)

		case "/api/v2/torrents/trackers":
			if f.trackerStatus != 0 {
				w.WriteHeader(f.trackerStatus)
				return
			}
			hash := r.URL.Query().Get("hash")
			for _, tor := range f.torrents {
				if tor.hash == hash {
					writeJSON(t, w, tor.trackers)
					return
				}
			}
			writeJSON(t, w, []qbittorrent.Tracker{})

		case "/api/v2/torrents/reannounce":
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm() error = %v", err)
			}
			f.reannounce = append(f.reannounce, r.PostForm.Get("hashes"))
			_, _ = io.WriteString(w, "Ok.")

		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

// tracker is shorthand for one tracker row.
func tracker(url string, status int) qbittorrent.Tracker {
	return qbittorrent.Tracker{URL: url, Status: status}
}

// The failure test is the whole selection rule, and every part of it has been
// wrong in a real client at some point: qBittorrent 5.x split the old
// catch-all status 4 into 4/5/6, and the DHT/PeX/LSD rows sit in the same
// list as real trackers while never announcing.
func TestAllFailed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		trackers []qbittorrent.Tracker
		want     bool
	}{
		{"no trackers at all", nil, false},
		{
			"only pseudo-trackers",
			[]qbittorrent.Tracker{
				tracker("** [DHT] **", 0),
				tracker("** [PeX] **", 0),
				tracker("** [LSD] **", 0),
			},
			false,
		},
		{
			"one working tracker",
			[]qbittorrent.Tracker{tracker("https://t.test/announce", qbittorrent.TrackerWorking)},
			false,
		},
		{
			"not contacted is not a failure",
			[]qbittorrent.Tracker{tracker("https://t.test/announce", qbittorrent.TrackerNotContacted)},
			false,
		},
		{
			"updating is not a failure",
			[]qbittorrent.Tracker{tracker("https://t.test/announce", qbittorrent.TrackerUpdating)},
			false,
		},
		{
			"not working",
			[]qbittorrent.Tracker{tracker("https://t.test/announce", qbittorrent.TrackerNotWorking)},
			true,
		},
		{
			"tracker error",
			[]qbittorrent.Tracker{tracker("https://t.test/announce", qbittorrent.TrackerError)},
			true,
		},
		{
			"unreachable, the status 6 that == 4 would miss",
			[]qbittorrent.Tracker{tracker("https://t.test/announce", 6)},
			true,
		},
		{
			"pseudo-trackers alongside a failed real one",
			[]qbittorrent.Tracker{
				tracker("** [DHT] **", 0),
				tracker("https://t.test/announce", qbittorrent.TrackerError),
			},
			true,
		},
		{
			"one of two real trackers still works",
			[]qbittorrent.Tracker{
				tracker("https://a.test/announce", qbittorrent.TrackerError),
				tracker("https://b.test/announce", qbittorrent.TrackerWorking),
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := reannounce.AllFailed(tc.trackers); got != tc.want {
				t.Errorf("AllFailed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRunReannouncesOnlyStalledTorrents(t *testing.T) {
	t.Parallel()

	f := &fake{torrents: []torrent{
		{
			hash:     "aaaa",
			progress: 0,
			trackers: []qbittorrent.Tracker{tracker("https://t.test/a", qbittorrent.TrackerError)},
		},
		{
			// Downloading: it has talked to someone, so its tracker
			// list is not what is holding it up.
			hash:     "bbbb",
			progress: 0.5,
			trackers: []qbittorrent.Tracker{tracker("https://t.test/b", qbittorrent.TrackerError)},
		},
		{
			hash:     "cccc",
			progress: 0,
			trackers: []qbittorrent.Tracker{tracker("https://t.test/c", qbittorrent.TrackerWorking)},
		},
		{
			hash:     "dddd",
			progress: 0,
			trackers: []qbittorrent.Tracker{tracker("https://t.test/d", 6)},
		},
	}}

	cfg := reannounce.Config{API: f.serve(t)}
	if err := reannounce.Run(context.Background(), testEnv(false), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if diff := cmp.Diff([]string{"aaaa", "dddd"}, f.reannounce); diff != "" {
		t.Errorf("reannounced (-want +got):\n%s", diff)
	}
}

func TestRunDryRunAnnouncesNothing(t *testing.T) {
	t.Parallel()

	f := &fake{torrents: []torrent{{
		hash:     "aaaa",
		trackers: []qbittorrent.Tracker{tracker("https://t.test/a", qbittorrent.TrackerError)},
	}}}

	cfg := reannounce.Config{API: f.serve(t)}
	if err := reannounce.Run(context.Background(), testEnv(true), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(f.reannounce) != 0 {
		t.Errorf("reannounced %v under --dry-run, want none", f.reannounce)
	}
}

// A listing that fails has to surface: the unit is a oneshot and a silent
// success would let a wedged WebUI look like a quiet pass forever.
func TestRunReportsListingFailure(t *testing.T) {
	t.Parallel()

	f := &fake{listStatus: http.StatusInternalServerError}
	cfg := reannounce.Config{API: f.serve(t)}
	if err := reannounce.Run(context.Background(), testEnv(false), cfg); err == nil {
		t.Fatal("Run() error = nil, want a failure")
	}
}

func TestRunReportsTrackerFailure(t *testing.T) {
	t.Parallel()

	f := &fake{
		torrents:      []torrent{{hash: "aaaa"}},
		trackerStatus: http.StatusInternalServerError,
	}
	cfg := reannounce.Config{API: f.serve(t)}
	if err := reannounce.Run(context.Background(), testEnv(false), cfg); err == nil {
		t.Fatal("Run() error = nil, want a failure")
	}
}

func TestRunRejectsUnusableAPI(t *testing.T) {
	t.Parallel()

	err := reannounce.Run(context.Background(), testEnv(false), reannounce.Config{API: ""})
	if err == nil {
		t.Fatal("Run() error = nil, want a failure")
	}
	if !strings.Contains(err.Error(), "BaseURL") {
		t.Errorf("Run() error = %v, want it to name the missing base URL", err)
	}
}

func TestDefaultsAreEmpty(t *testing.T) {
	t.Parallel()

	if got := reannounce.Defaults(); got != (reannounce.Config{}) {
		t.Errorf("Defaults() = %+v, want the zero config", got)
	}
}
