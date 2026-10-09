package importer_test

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
	"time"

	"github.com/google/go-cmp/cmp"

	"homelab/app"
	"homelab/beets-lidarr-import/internal/importer"
)

func testEnv(dryRun bool) *app.Env {
	return &app.Env{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Stdout: io.Discard,
		DryRun: dryRun,
	}
}

// request is one recorded call to the fake Lidarr.
type request struct {
	Method string
	Path   string
	Query  string
	Body   map[string]any
}

// fake is a Lidarr that answers a scripted sequence of command reads.
type fake struct {
	mu sync.Mutex
	// listing is the manualimport answer, as raw JSON so a test can send
	// shapes a Go literal could not express.
	listing string
	// queued is the answer to POST /command.
	queued string
	// statuses are the answers to GET /command/{id}, in order; the last
	// one repeats.
	statuses []string
	// status, when non-zero, fails the manualimport listing.
	listStatus int

	requests []request
}

func (f *fake) serve(t *testing.T) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		entry := request{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query().Encode()}
		if r.Method == http.MethodPost {
			raw, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(raw, &entry.Body); err != nil {
				t.Errorf("decode request body: %v", err)
			}
		}
		f.requests = append(f.requests, entry)

		switch {
		case r.URL.Path == "/api/v1/manualimport":
			if f.listStatus != 0 {
				w.WriteHeader(f.listStatus)
				return
			}
			_, _ = io.WriteString(w, f.listing)
		case r.URL.Path == "/api/v1/command" && r.Method == http.MethodPost:
			_, _ = io.WriteString(w, f.queued)
		case strings.HasPrefix(r.URL.Path, "/api/v1/command/"):
			if len(f.statuses) == 0 {
				t.Errorf("unexpected command read at %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			answer := f.statuses[0]
			if len(f.statuses) > 1 {
				f.statuses = f.statuses[1:]
			}
			_, _ = io.WriteString(w, answer)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

// item is one fully-resolved listing entry, as JSON.
func item(path string) string {
	return `{"path":"` + path + `","artist":{"id":34,"artistName":"MEOVV"},
	  "album":{"id":408,"title":"BITE NOW"},"albumReleaseId":1589,
	  "tracks":[{"id":10808}],"quality":{"quality":{"id":6,"name":"FLAC"}},
	  "rejections":[]}`
}

// setEnv points the tool at base and clears everything else.
func setEnv(t *testing.T, base string, extra map[string]string) {
	t.Helper()

	for _, key := range []string{
		importer.EnvURL, importer.EnvAPIKey, importer.EnvMode,
		importer.EnvTimeout, importer.EnvPoll,
	} {
		t.Setenv(key, "")
	}
	t.Setenv(importer.EnvURL, base)
	t.Setenv(importer.EnvAPIKey, "secret")
	// Polling at the default two seconds would make every test that waits
	// take two seconds.
	t.Setenv(importer.EnvPoll, "0.001")
	for key, value := range extra {
		t.Setenv(key, value)
	}
}

func run(t *testing.T, f *fake, extra map[string]string) error {
	t.Helper()

	setEnv(t, f.serve(t), extra)
	return importer.Run(context.Background(), testEnv(false),
		importer.Config{Album: "/rips/A/Album"})
}

func TestRunImportsWhatLidarrIdentified(t *testing.T) {
	f := &fake{
		listing:  "[" + item("/a/01.flac") + "," + item("/a/02.flac") + "]",
		queued:   `{"id":165885}`,
		statuses: []string{`{"status":"completed","result":"successful"}`},
	}

	if err := run(t, f, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(f.requests) != 3 {
		t.Fatalf("made %d requests, want 3: %+v", len(f.requests), f.requests)
	}
	posted := f.requests[1]
	if posted.Method != http.MethodPost || posted.Path != "/api/v1/command" {
		t.Errorf("second request = %s %s", posted.Method, posted.Path)
	}
	if posted.Body["name"] != "ManualImport" {
		t.Errorf("command name = %v, want ManualImport", posted.Body["name"])
	}
	files, _ := posted.Body["files"].([]any)
	if len(files) != 2 {
		t.Errorf("posted %d files, want 2", len(files))
	}
}

// DownloadedAlbumsScan is the path that applies the 80% threshold and rejects
// a correct rip whose release Lidarr's mirror lacks. Reaching for it again
// would undo the whole point of this tool.
func TestRunNeverScans(t *testing.T) {
	f := &fake{
		listing:  "[" + item("/a/01.flac") + "]",
		queued:   `{"id":1}`,
		statuses: []string{`{"status":"completed","result":"successful"}`},
	}

	if err := run(t, f, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, r := range f.requests {
		if name, _ := r.Body["name"].(string); strings.Contains(name, "Scan") {
			t.Errorf("sent command %q", name)
		}
	}
}

// Importing the resolved files alone would leave the album silently short,
// which is worse than not importing it.
func TestRunRefusesAPartlyPlacedAlbum(t *testing.T) {
	unresolved := `{"path":"/a/02.flac","artist":{"id":34},"album":{"id":408},
	  "albumReleaseId":1589,"tracks":[],
	  "rejections":[{"reason":"Unknown album"}]}`
	f := &fake{listing: "[" + item("/a/01.flac") + "," + unresolved + "]"}

	err := run(t, f, nil)
	if err == nil {
		t.Fatal("Run() error = nil, want a failure")
	}
	for _, want := range []string{"could not place 1 of 2", "02.flac: Unknown album"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
	// Nothing was sent: the files stay where beets put them.
	if len(f.requests) != 1 {
		t.Errorf("made %d requests, want 1", len(f.requests))
	}
}

func TestRunReportsAnUnsuccessfulImport(t *testing.T) {
	f := &fake{
		listing:  "[" + item("/a/01.flac") + "]",
		queued:   `{"id":7}`,
		statuses: []string{`{"status":"completed","result":"unsuccessful"}`},
	}

	err := run(t, f, nil)
	if err == nil || !strings.Contains(err.Error(), "imported nothing") {
		t.Fatalf("Run() error = %v, want it to report that nothing was imported", err)
	}
}

func TestRunReportsAFailedCommand(t *testing.T) {
	f := &fake{
		listing:  "[" + item("/a/01.flac") + "]",
		queued:   `{"id":7}`,
		statuses: []string{`{"status":"failed","exception":"disk full"}`},
	}

	err := run(t, f, nil)
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("Run() error = %v, want it to carry the exception", err)
	}
}

func TestRunReportsARejectedRequest(t *testing.T) {
	f := &fake{listing: "[" + item("/a/01.flac") + "]", queued: `{}`}

	err := run(t, f, nil)
	if err == nil || !strings.Contains(err.Error(), "rejected the request") {
		t.Fatalf("Run() error = %v, want it to report the rejection", err)
	}
}

func TestRunPollsUntilTheCommandFinishes(t *testing.T) {
	f := &fake{
		listing: "[" + item("/a/01.flac") + "]",
		queued:  `{"id":9}`,
		statuses: []string{
			`{"status":"queued"}`,
			`{"status":"started"}`,
			`{"status":"completed","result":"successful"}`,
		},
	}

	if err := run(t, f, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// listing, POST, then three reads.
	if len(f.requests) != 5 {
		t.Errorf("made %d requests, want 5: %+v", len(f.requests), f.requests)
	}
}

func TestRunGivesUpEventually(t *testing.T) {
	f := &fake{
		listing:  "[" + item("/a/01.flac") + "]",
		queued:   `{"id":9}`,
		statuses: []string{`{"status":"started"}`},
	}

	err := run(t, f, map[string]string{importer.EnvTimeout: "0.05"})
	if err == nil || !strings.Contains(err.Error(), "gave up waiting") {
		t.Fatalf("Run() error = %v, want it to give up", err)
	}
}

func TestRunHonoursTheImportMode(t *testing.T) {
	cases := map[string]string{
		// Copy plus Lidarr's copyUsingHardlinks keeps the rip as its
		// own tree at no extra disk cost; Move would take it away.
		"":     "Copy",
		"Move": "Move",
		"Auto": "Auto",
	}

	for set, want := range cases {
		t.Run("env="+set, func(t *testing.T) {
			f := &fake{
				listing:  "[" + item("/a/01.flac") + "]",
				queued:   `{"id":1}`,
				statuses: []string{`{"status":"completed","result":"successful"}`},
			}
			extra := map[string]string{}
			if set != "" {
				extra[importer.EnvMode] = set
			}
			if err := run(t, f, extra); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := f.requests[1].Body["importMode"]; got != want {
				t.Errorf("importMode = %v, want %v", got, want)
			}
		})
	}
}

func TestRunRequiresAnAlbum(t *testing.T) {
	f := &fake{}
	setEnv(t, f.serve(t), nil)

	err := importer.Run(context.Background(), testEnv(false), importer.Config{})
	if err == nil || !strings.Contains(err.Error(), "--album") {
		t.Fatalf("Run() error = %v, want it to name the missing flag", err)
	}
}

func TestRunRequiresTheEnvironment(t *testing.T) {
	for _, missing := range []string{importer.EnvURL, importer.EnvAPIKey} {
		t.Run(missing, func(t *testing.T) {
			f := &fake{}
			setEnv(t, f.serve(t), nil)
			t.Setenv(missing, "")

			err := importer.Run(context.Background(), testEnv(false),
				importer.Config{Album: "/a"})
			if err == nil || !strings.Contains(err.Error(), missing+" is not set") {
				t.Fatalf("Run() error = %v, want it to name %s", err, missing)
			}
		})
	}
}

func TestRunRejectsANonsenseTimeout(t *testing.T) {
	f := &fake{}
	setEnv(t, f.serve(t), map[string]string{importer.EnvTimeout: "soon"})

	err := importer.Run(context.Background(), testEnv(false),
		importer.Config{Album: "/a"})
	if err == nil || !strings.Contains(err.Error(), "not a number") {
		t.Fatalf("Run() error = %v, want it to reject the timeout", err)
	}
}

func TestRunReportsAnEmptyListing(t *testing.T) {
	f := &fake{listing: "[]"}

	err := run(t, f, nil)
	if err == nil || !strings.Contains(err.Error(), "no importable files") {
		t.Fatalf("Run() error = %v, want it to report the empty listing", err)
	}
}

func TestRunReportsAnUnreachableLidarr(t *testing.T) {
	setEnv(t, "http://lidarr.invalid:8686", nil)

	err := importer.Run(context.Background(), testEnv(false),
		importer.Config{Album: "/a"})
	if err == nil || !strings.Contains(err.Error(), "could not reach Lidarr") {
		t.Fatalf("Run() error = %v, want it to report the unreachable Lidarr", err)
	}
}

func TestRunDryRunSendsNoCommand(t *testing.T) {
	f := &fake{listing: "[" + item("/a/01.flac") + "]"}
	setEnv(t, f.serve(t), nil)

	err := importer.Run(context.Background(), testEnv(true),
		importer.Config{Album: "/rips/A/Album"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(f.requests) != 1 {
		t.Errorf("made %d requests under --dry-run, want 1 (the listing)", len(f.requests))
	}
}

// The URL is joined with the API path, so a trailing slash — however many —
// would produce "//api/v1", a 404 that reads like a wrong endpoint.
func TestLoadSettingsTrimsTrailingSlashes(t *testing.T) {
	setEnv(t, "http://lidarr:8686///", nil)

	got, err := importer.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if got.URL != "http://lidarr:8686" {
		t.Errorf("URL = %q, want http://lidarr:8686", got.URL)
	}
}

func TestLoadSettingsDefaults(t *testing.T) {
	setEnv(t, "http://lidarr:8686", nil)
	t.Setenv(importer.EnvPoll, "")

	got, err := importer.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}

	want := importer.Settings{
		URL:     "http://lidarr:8686",
		APIKey:  "secret",
		Mode:    importer.DefaultMode,
		Timeout: importer.DefaultTimeout,
		Poll:    importer.DefaultPoll,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("LoadSettings() (-want +got):\n%s", diff)
	}
}

// The environment carries plain seconds, which is what it has always carried
// here — a fractional value has to keep working.
func TestLoadSettingsReadsSeconds(t *testing.T) {
	setEnv(t, "http://lidarr:8686", map[string]string{
		importer.EnvTimeout: "12.5",
		importer.EnvPoll:    "0.25",
	})

	got, err := importer.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if got.Timeout != 12500*time.Millisecond {
		t.Errorf("Timeout = %s, want 12.5s", got.Timeout)
	}
	if got.Poll != 250*time.Millisecond {
		t.Errorf("Poll = %s, want 250ms", got.Poll)
	}
}

func TestLoadSettingsRejectsANonsensePoll(t *testing.T) {
	setEnv(t, "http://lidarr:8686", map[string]string{importer.EnvPoll: "often"})

	if _, err := importer.LoadSettings(); err == nil {
		t.Fatal("LoadSettings() error = nil, want a failure")
	}
}

func TestDefaultsAreEmpty(t *testing.T) {
	if got := importer.Defaults(); got != (importer.Config{}) {
		t.Errorf("Defaults() = %+v, want the zero config", got)
	}
}
