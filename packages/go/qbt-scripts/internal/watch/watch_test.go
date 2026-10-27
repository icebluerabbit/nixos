package watch_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"homelab/app"
	"homelab/qbt-scripts/internal/watch"
)

func testEnv(dryRun bool) *app.Env {
	return &app.Env{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Stdout: io.Discard,
		DryRun: dryRun,
	}
}

// added records one accepted upload, decoded from the multipart body so the
// test asserts on what qBittorrent would actually see.
type added struct {
	Filename string
	Category string
	AutoTMM  string
	Content  string
}

// fake is a stand-in qBittorrent WebUI.
type fake struct {
	mu    sync.Mutex
	added []added
	// reject makes the add answer "refused" for these filenames, which is
	// what a torrent already in the client produces.
	reject map[string]bool
	// fail makes the add return a server error for these filenames.
	fail map[string]bool
	// legacyOK answers with the pre-5.1 bare "Ok." instead of JSON.
	legacyOK bool
}

func (f *fake) serve(t *testing.T) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/torrents/add" {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		entry := readUpload(t, r)

		f.mu.Lock()
		defer f.mu.Unlock()

		switch {
		case f.fail[entry.Filename]:
			w.WriteHeader(http.StatusInternalServerError)
		case f.reject[entry.Filename]:
			_, _ = io.WriteString(w, `{"success_count":0}`)
		case f.legacyOK:
			f.added = append(f.added, entry)
			_, _ = io.WriteString(w, "Ok.")
		default:
			f.added = append(f.added, entry)
			_, _ = io.WriteString(w, `{"success_count":1}`)
		}
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

// readUpload decodes the multipart body the client sent.
func readUpload(t *testing.T, r *http.Request) added {
	t.Helper()

	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse Content-Type: %v", err)
	}
	reader := multipart.NewReader(r.Body, params["boundary"])

	var entry added
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part %q: %v", part.FormName(), err)
		}
		switch part.FormName() {
		case "category":
			entry.Category = string(body)
		case "autoTMM":
			entry.AutoTMM = string(body)
		case "torrents":
			entry.Filename = part.FileName()
			entry.Content = string(body)
			if got := part.Header.Get("Content-Type"); got != "application/x-bittorrent" {
				t.Errorf("file part Content-Type = %q, want application/x-bittorrent", got)
			}
		default:
			t.Errorf("unexpected form field %q", part.FormName())
		}
	}
	return entry
}

// drop writes one .torrent into watch/<category>/.
func drop(t *testing.T, watchDir, category, name, content string) {
	t.Helper()

	dir := filepath.Join(watchDir, category)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// listDir returns the sorted names in dir, or nothing when it is absent.
func listDir(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestCategoryList(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"movies", []string{"movies"}},
		{"movies,tv,music", []string{"movies", "tv", "music"}},
		// A trailing comma must not mint a category named "", which
		// would glob the watch root itself.
		{"movies,,tv,", []string{"movies", "tv"}},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := watch.Config{Categories: tc.in}.CategoryList()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("CategoryList() (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRunImportsAndBacksUp(t *testing.T) {
	t.Parallel()

	watchDir := t.TempDir()
	backupDir := filepath.Join(t.TempDir(), "export")
	drop(t, watchDir, "movies", "a.torrent", "d8:announce...")
	drop(t, watchDir, "tv", "b.torrent", "d8:announce!!!")
	// Not a .torrent: left where it is.
	drop(t, watchDir, "tv", "notes.txt", "ignore me")

	f := &fake{}
	cfg := watch.Config{
		API:        f.serve(t),
		WatchDir:   watchDir,
		BackupDir:  backupDir,
		Categories: "movies,tv",
	}
	if err := watch.Run(context.Background(), testEnv(false), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := []added{
		{Filename: "a.torrent", Category: "movies", AutoTMM: "true", Content: "d8:announce..."},
		{Filename: "b.torrent", Category: "tv", AutoTMM: "true", Content: "d8:announce!!!"},
	}
	if diff := cmp.Diff(want, f.added); diff != "" {
		t.Errorf("uploads (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]string{"a.torrent", "b.torrent"}, listDir(t, backupDir)); diff != "" {
		t.Errorf("backup directory (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{}, listDir(t, filepath.Join(watchDir, "movies"))); diff != "" {
		t.Errorf("movies watch directory (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"notes.txt"}, listDir(t, filepath.Join(watchDir, "tv"))); diff != "" {
		t.Errorf("tv watch directory (-want +got):\n%s", diff)
	}
}

// The pre-5.1 answer is a bare string. Treating it as a rejection would leave
// every imported file in the watch directory to be re-added forever.
func TestRunAcceptsLegacyOkAnswer(t *testing.T) {
	t.Parallel()

	watchDir, backupDir := t.TempDir(), t.TempDir()
	drop(t, watchDir, "movies", "a.torrent", "x")

	f := &fake{legacyOK: true}
	cfg := watch.Config{
		API: f.serve(t), WatchDir: watchDir, BackupDir: backupDir,
		Categories: "movies",
	}
	if err := watch.Run(context.Background(), testEnv(false), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if diff := cmp.Diff([]string{"a.torrent"}, listDir(t, backupDir)); diff != "" {
		t.Errorf("backup directory (-want +got):\n%s", diff)
	}
}

// A rejected or failed add leaves the file in place, so the next pass retries
// it. Neither stops the rest of the drop.
func TestRunLeavesUnimportedFilesAlone(t *testing.T) {
	t.Parallel()

	watchDir, backupDir := t.TempDir(), t.TempDir()
	drop(t, watchDir, "movies", "bad.torrent", "x")
	drop(t, watchDir, "movies", "dupe.torrent", "x")
	drop(t, watchDir, "movies", "good.torrent", "x")

	f := &fake{
		fail:   map[string]bool{"bad.torrent": true},
		reject: map[string]bool{"dupe.torrent": true},
	}
	cfg := watch.Config{
		API: f.serve(t), WatchDir: watchDir, BackupDir: backupDir,
		Categories: "movies",
	}
	if err := watch.Run(context.Background(), testEnv(false), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if diff := cmp.Diff([]string{"good.torrent"}, listDir(t, backupDir)); diff != "" {
		t.Errorf("backup directory (-want +got):\n%s", diff)
	}
	left := listDir(t, filepath.Join(watchDir, "movies"))
	if diff := cmp.Diff([]string{"bad.torrent", "dupe.torrent"}, left); diff != "" {
		t.Errorf("watch directory (-want +got):\n%s", diff)
	}
}

func TestRunDryRunImportsNothing(t *testing.T) {
	t.Parallel()

	watchDir, backupDir := t.TempDir(), t.TempDir()
	drop(t, watchDir, "movies", "a.torrent", "x")

	f := &fake{}
	cfg := watch.Config{
		API: f.serve(t), WatchDir: watchDir, BackupDir: backupDir,
		Categories: "movies",
	}
	if err := watch.Run(context.Background(), testEnv(true), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(f.added) != 0 {
		t.Errorf("uploaded %v under --dry-run, want none", f.added)
	}
	if diff := cmp.Diff([]string{"a.torrent"}, listDir(t, filepath.Join(watchDir, "movies"))); diff != "" {
		t.Errorf("watch directory (-want +got):\n%s", diff)
	}
}

func TestRunReportsProvenance(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		reports []map[string]string
	)
	recorder := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode report: %v", err)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		reports = append(reports, body)
	}))
	t.Cleanup(recorder.Close)

	watchDir, backupDir := t.TempDir(), t.TempDir()
	drop(t, watchDir, "movies", "a.torrent", "x")

	f := &fake{}
	cfg := watch.Config{
		API: f.serve(t), WatchDir: watchDir, BackupDir: backupDir,
		Categories: "movies", ReportURL: recorder.URL,
	}
	if err := watch.Run(context.Background(), testEnv(false), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := []map[string]string{
		{"event": watch.Event, "name": "a.torrent", "category": "movies"},
	}
	if diff := cmp.Diff(want, reports); diff != "" {
		t.Errorf("reports (-want +got):\n%s", diff)
	}
}

// The import already succeeded by the time the ping is sent, so a recorder
// that is down must not turn it into a failure.
func TestRunSurvivesAnUnreachableRecorder(t *testing.T) {
	t.Parallel()

	watchDir, backupDir := t.TempDir(), t.TempDir()
	drop(t, watchDir, "movies", "a.torrent", "x")

	f := &fake{}
	cfg := watch.Config{
		API: f.serve(t), WatchDir: watchDir, BackupDir: backupDir,
		Categories: "movies",
		// Reserved as invalid by RFC 2606, so nothing resolves it.
		ReportURL: "http://recorder.invalid/record",
	}
	if err := watch.Run(context.Background(), testEnv(false), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if diff := cmp.Diff([]string{"a.torrent"}, listDir(t, backupDir)); diff != "" {
		t.Errorf("backup directory (-want +got):\n%s", diff)
	}
}

func TestRunCreatesTheBackupDirectory(t *testing.T) {
	t.Parallel()

	watchDir := t.TempDir()
	backupDir := filepath.Join(t.TempDir(), "nested", "export")

	f := &fake{}
	cfg := watch.Config{
		API: f.serve(t), WatchDir: watchDir, BackupDir: backupDir,
		Categories: "movies",
	}
	if err := watch.Run(context.Background(), testEnv(false), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if _, err := os.Stat(backupDir); err != nil {
		t.Errorf("Stat(%s) error = %v, want the directory to exist", backupDir, err)
	}
}

// A directory ending in .torrent matches the glob and cannot be read. One
// unreadable entry must not stop the files behind it.
func TestRunSkipsUnreadableEntries(t *testing.T) {
	t.Parallel()

	watchDir, backupDir := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(watchDir, "movies", "a.torrent"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	drop(t, watchDir, "movies", "b.torrent", "x")

	f := &fake{}
	cfg := watch.Config{
		API: f.serve(t), WatchDir: watchDir, BackupDir: backupDir,
		Categories: "movies",
	}
	if err := watch.Run(context.Background(), testEnv(false), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if diff := cmp.Diff([]string{"b.torrent"}, listDir(t, backupDir)); diff != "" {
		t.Errorf("backup directory (-want +got):\n%s", diff)
	}
}

// A category name carrying glob metacharacters makes the scan itself fail,
// which is a configuration mistake rather than a bad torrent.
func TestRunReportsABadCategoryPattern(t *testing.T) {
	t.Parallel()

	f := &fake{}
	cfg := watch.Config{
		API: f.serve(t), WatchDir: t.TempDir(), BackupDir: t.TempDir(),
		Categories: "[",
	}
	if err := watch.Run(context.Background(), testEnv(false), cfg); err == nil {
		t.Fatal("Run() error = nil, want a failure")
	}
}

func TestRunRejectsUnusableAPI(t *testing.T) {
	t.Parallel()

	err := watch.Run(context.Background(), testEnv(false), watch.Config{BackupDir: t.TempDir()})
	if err == nil {
		t.Fatal("Run() error = nil, want a failure")
	}
	if !strings.Contains(err.Error(), "BaseURL") {
		t.Errorf("Run() error = %v, want it to name the missing base URL", err)
	}
}

func TestRunReportsAnUncreatableBackupDirectory(t *testing.T) {
	t.Parallel()

	// A regular file where the directory should be: MkdirAll cannot
	// proceed, and importing with nowhere to move the file to would
	// re-add it on every pass.
	blocked := filepath.Join(t.TempDir(), "export")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatalf("write %s: %v", blocked, err)
	}

	f := &fake{}
	cfg := watch.Config{API: f.serve(t), WatchDir: t.TempDir(), BackupDir: blocked}
	if err := watch.Run(context.Background(), testEnv(false), cfg); err == nil {
		t.Fatal("Run() error = nil, want a failure")
	}
}

func TestDefaultsAreEmpty(t *testing.T) {
	t.Parallel()

	if got := watch.Defaults(); got != (watch.Config{}) {
		t.Errorf("Defaults() = %+v, want the zero config", got)
	}
}
