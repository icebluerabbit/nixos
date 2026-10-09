package reclaim

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"homelab/app"
)

// logEnv captures the log records so a test can assert what was reported.
func logEnv(buf *bytes.Buffer) *app.Env {
	return &app.Env{
		Logger: slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Stdout: io.Discard,
	}
}

func discardEnv() *app.Env {
	return &app.Env{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Stdout: io.Discard,
	}
}

// library builds a download directory: one subdirectory per album, each
// aged by setting its mtime.
type library struct {
	root string
}

func newLibrary(t *testing.T) *library {
	t.Helper()
	return &library{root: t.TempDir()}
}

// album creates a folder with the given files, aged by age.
func (l *library) album(t *testing.T, name string, age time.Duration, files ...string) string {
	t.Helper()
	dir := filepath.Join(l.root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		path := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(dir, when, when); err != nil {
		t.Fatal(err)
	}
	return dir
}

// lidarr is a stand-in for the Lidarr command API.
type lidarr struct {
	mu sync.Mutex
	// scanned records the folder path of every DownloadedAlbumsScan.
	scanned []string
	// keys records the API key each request carried.
	keys []string
	// result is what a settled command reports.
	result string
	// status overrides the settled status.
	status string
	// message is the command's message field.
	message string
	// pending is how many polls return "started" before the command
	// settles.
	pending int
	// nextID numbers the commands.
	nextID int
}

func (l *lidarr) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.keys = append(l.keys, r.Header.Get("X-Api-Key"))

		status := l.status
		if status == "" {
			status = "completed"
		}
		result := l.result
		if result == "" {
			result = "successful"
		}

		if r.Method == http.MethodPost {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode: %v", err)
			}
			path, _ := body["path"].(string)
			l.scanned = append(l.scanned, path)
			l.nextID++

			// A command that is still running comes back "started" and is
			// polled; one that is already done settles immediately.
			if l.pending > 0 {
				writeCommand(t, w, l.nextID, "started", "", "")
				return
			}
			writeCommand(t, w, l.nextID, status, result, l.message)
			return
		}

		if l.pending > 0 {
			l.pending--
			writeCommand(t, w, l.nextID, "started", "", "")
			return
		}
		writeCommand(t, w, l.nextID, status, result, l.message)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func writeCommand(t *testing.T, w http.ResponseWriter, id int, status, result, message string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"id": id, "status": status, "result": result, "message": message,
	}); err != nil {
		t.Errorf("encode: %v", err)
	}
}

func (l *lidarr) scannedPaths() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.scanned...)
}

// keyFile writes an API key the way the bound sops secret presents it.
func keyFile(t *testing.T, key string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "api-key")
	if err := os.WriteFile(path, []byte(key+"\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	return path
}

func fast() Options { return Options{PollInterval: time.Millisecond} }

func config(lib *library, url, key string) Config {
	return Config{
		LidarrURL:      url,
		KeyFile:        key,
		DownloadDir:    lib.root,
		MinAge:         2 * time.Hour,
		CommandTimeout: 120 * time.Second,
	}
}

// One command per album folder: the parent directory has no audio directly
// inside it and Lidarr answers "Failed to import" in milliseconds.
func TestRunScansEachAlbumFolder(t *testing.T) {
	lib := newLibrary(t)
	lib.album(t, "Artist - Album A", 3*time.Hour, "01.flac")
	lib.album(t, "Artist - Album B", 4*time.Hour, "disc1/01.mp3")

	l := &lidarr{}
	url := l.serve(t)

	var logs bytes.Buffer
	if err := Run(context.Background(), logEnv(&logs), config(lib, url, keyFile(t, "secret")), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	scanned := l.scannedPaths()
	if len(scanned) != 2 {
		t.Fatalf("scanned %v, want both album folders", scanned)
	}
	for _, path := range scanned {
		if filepath.Dir(path) != lib.root {
			t.Errorf("scanned %q, which is not an album folder", path)
		}
	}
	if !strings.Contains(logs.String(), `"imported":2`) {
		t.Errorf("summary missing from logs: %s", logs.String())
	}
}

// Longest-stranded first, so a capped run works through the oldest backlog
// rather than the same recent folders every time.
func TestRunTakesTheOldestFirst(t *testing.T) {
	lib := newLibrary(t)
	lib.album(t, "newer", 3*time.Hour, "01.flac")
	lib.album(t, "oldest", 30*24*time.Hour, "01.flac")
	lib.album(t, "middle", 5*24*time.Hour, "01.flac")

	l := &lidarr{}
	cfg := config(lib, l.serve(t), keyFile(t, "secret"))
	cfg.MaxFolders = 2

	var logs bytes.Buffer
	if err := Run(context.Background(), logEnv(&logs), cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	scanned := l.scannedPaths()
	if len(scanned) != 2 {
		t.Fatalf("scanned %d folders, want the cap of 2", len(scanned))
	}
	if filepath.Base(scanned[0]) != "oldest" || filepath.Base(scanned[1]) != "middle" {
		t.Errorf("order = %v, want oldest then middle", scanned)
	}
	if !strings.Contains(logs.String(), `"cap":2`) {
		t.Errorf("the cap was not reported: %s", logs.String())
	}
}

// A folder still being written to is left alone: slskd lands a multi-file
// album one file at a time.
func TestRunSkipsFreshFolders(t *testing.T) {
	lib := newLibrary(t)
	lib.album(t, "still landing", 10*time.Minute, "01.flac")

	l := &lidarr{}
	var logs bytes.Buffer
	if err := Run(context.Background(), logEnv(&logs), config(lib, l.serve(t), keyFile(t, "k")), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := l.scannedPaths(); len(got) != 0 {
		t.Errorf("scanned %v, want nothing", got)
	}
	if !strings.Contains(logs.String(), "too fresh") {
		t.Errorf("no skip was reported: %s", logs.String())
	}
}

// soularr already denylisted everything in failed_imports; nothing retries
// those, including this.
func TestRunSkipsFailedImports(t *testing.T) {
	lib := newLibrary(t)
	lib.album(t, failedImports, 10*24*time.Hour, "01.flac")
	lib.album(t, "good", 10*24*time.Hour, "01.flac")

	l := &lidarr{}
	if err := Run(context.Background(), discardEnv(), config(lib, l.serve(t), keyFile(t, "k")), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	scanned := l.scannedPaths()
	if len(scanned) != 1 || filepath.Base(scanned[0]) != "good" {
		t.Errorf("scanned %v, want only the good folder", scanned)
	}
}

// Artwork and nfo leftovers are not albums.
func TestRunSkipsFoldersWithoutAudio(t *testing.T) {
	lib := newLibrary(t)
	lib.album(t, "just artwork", 10*24*time.Hour, "cover.jpg", "album.nfo")

	l := &lidarr{}
	var logs bytes.Buffer
	if err := Run(context.Background(), logEnv(&logs), config(lib, l.serve(t), keyFile(t, "k")), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := l.scannedPaths(); len(got) != 0 {
		t.Errorf("scanned %v, want nothing", got)
	}
	if !strings.Contains(logs.String(), "no audio") {
		t.Errorf("no skip was reported: %s", logs.String())
	}
}

// A multi-disc rip nests its tracks, so the search has to go deeper than
// one level.
func TestHasAudioFindsNestedTracks(t *testing.T) {
	lib := newLibrary(t)
	nested := lib.album(t, "boxset", time.Hour, "disc1/disc2/01.OPUS")

	if !hasAudio(nested) {
		t.Error("a nested track was not found")
	}
	if hasAudio(lib.album(t, "empty", time.Hour, "readme.txt")) {
		t.Error("a folder with no audio was reported as having some")
	}
}

// Lidarr reports a scan that matched nothing as a completed command with
// result "unsuccessful". That is a normal outcome, not an error: the folder
// stays where it is and is counted.
func TestRunCountsUnmatchedFolders(t *testing.T) {
	lib := newLibrary(t)
	lib.album(t, "badly tagged", 10*24*time.Hour, "01.flac")

	l := &lidarr{result: "unsuccessful", message: "Failed to import"}
	var logs bytes.Buffer
	if err := Run(context.Background(), logEnv(&logs), config(lib, l.serve(t), keyFile(t, "k")), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := logs.String()
	if !strings.Contains(out, "no match") || !strings.Contains(out, "Failed to import") {
		t.Errorf("the outcome was not reported: %s", out)
	}
	if !strings.Contains(out, `"left_in_place":1`) {
		t.Errorf("summary = %s", out)
	}
}

// A command that is still running is polled until it settles.
func TestRunPollsUntilSettled(t *testing.T) {
	lib := newLibrary(t)
	lib.album(t, "slow", 10*24*time.Hour, "01.flac")

	l := &lidarr{pending: 3}
	var logs bytes.Buffer
	if err := Run(context.Background(), logEnv(&logs), config(lib, l.serve(t), keyFile(t, "k")), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(logs.String(), `"imported":1`) {
		t.Errorf("the settled command was not counted: %s", logs.String())
	}
}

// A command that never settles is abandoned with its id named, rather than
// holding the sweep open forever.
func TestRunTimesOutAStuckCommand(t *testing.T) {
	lib := newLibrary(t)
	lib.album(t, "stuck", 10*24*time.Hour, "01.flac")

	l := &lidarr{pending: 1 << 20} // never settles
	cfg := config(lib, l.serve(t), keyFile(t, "k"))
	cfg.CommandTimeout = 10 * time.Millisecond

	var logs bytes.Buffer
	if err := Run(context.Background(), logEnv(&logs), cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := logs.String()
	if !strings.Contains(out, "timed out after") {
		t.Errorf("no timeout was reported: %s", out)
	}
	if !strings.Contains(out, `"left_in_place":1`) {
		t.Errorf("summary = %s", out)
	}
}

// The key rides in the header on every call, and the trailing newline the
// secret file carries is stripped.
func TestRunSendsTheAPIKey(t *testing.T) {
	lib := newLibrary(t)
	lib.album(t, "album", 10*24*time.Hour, "01.flac")

	l := &lidarr{}
	if err := Run(context.Background(), discardEnv(), config(lib, l.serve(t), keyFile(t, "secret-key")), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.keys) == 0 {
		t.Fatal("no request was made")
	}
	for _, k := range l.keys {
		if k != "secret-key" {
			t.Errorf("key = %q, want secret-key", k)
		}
	}
}

// One folder that errors must not stop the rest of the sweep.
func TestRunContinuesAfterAnError(t *testing.T) {
	lib := newLibrary(t)
	lib.album(t, "a", 10*24*time.Hour, "01.flac")
	lib.album(t, "b", 11*24*time.Hour, "01.flac")

	var calls int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		first := calls == 1
		mu.Unlock()

		if first {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"id":0}`)
			return
		}
		writeCommand(t, w, 2, "completed", "successful", "")
	}))
	t.Cleanup(srv.Close)

	var logs bytes.Buffer
	if err := Run(context.Background(), logEnv(&logs), config(lib, srv.URL, keyFile(t, "k")), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := logs.String()
	if !strings.Contains(out, "scan failed") {
		t.Errorf("the failure was not reported: %s", out)
	}
	if !strings.Contains(out, `"attempted":2`) {
		t.Errorf("the sweep stopped early: %s", out)
	}
}

func TestRunReportsAnUnreadableKey(t *testing.T) {
	lib := newLibrary(t)
	cfg := config(lib, "http://127.0.0.1:1", filepath.Join(t.TempDir(), "absent"))

	err := Run(context.Background(), discardEnv(), cfg, fast())

	if err == nil || !strings.Contains(err.Error(), "absent") {
		t.Errorf("error = %v, want the key file named", err)
	}
}

func TestRunReportsAnUnreadableDownloadDir(t *testing.T) {
	cfg := Config{
		LidarrURL:   "http://127.0.0.1:1",
		KeyFile:     keyFile(t, "k"),
		DownloadDir: filepath.Join(t.TempDir(), "no-such-dir"),
		MinAge:      time.Hour,
	}

	err := Run(context.Background(), discardEnv(), cfg, fast())

	if err == nil || !strings.Contains(err.Error(), "no-such-dir") {
		t.Errorf("error = %v, want the directory named", err)
	}
}

func TestLoadRequiresTheEssentialSettings(t *testing.T) {
	for _, missing := range []string{EnvLidarrURL, EnvLidarrKeyFile, EnvDownloadDir} {
		t.Run(missing, func(t *testing.T) {
			t.Setenv(EnvLidarrURL, "http://lidarr:8686")
			t.Setenv(EnvLidarrKeyFile, "/run/secrets/key")
			t.Setenv(EnvDownloadDir, "/downloads")
			t.Setenv(missing, "")

			if _, err := Load(slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
				t.Errorf("Load succeeded with %s unset", missing)
			}
		})
	}
}

func TestLoadAppliesDefaultsAndTrailingSlash(t *testing.T) {
	t.Setenv(EnvLidarrURL, "http://lidarr:8686/")
	t.Setenv(EnvLidarrKeyFile, "/run/secrets/key")
	t.Setenv(EnvDownloadDir, "/downloads")
	t.Setenv(EnvMinAgeMinutes, "")
	t.Setenv(EnvMaxFolders, "")
	t.Setenv(EnvCommandTimeout, "")

	cfg, err := Load(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.LidarrURL != "http://lidarr:8686" {
		t.Errorf("LidarrURL = %q, want no trailing slash", cfg.LidarrURL)
	}
	if cfg.MinAge != DefaultMinAgeMinutes*time.Minute {
		t.Errorf("MinAge = %v", cfg.MinAge)
	}
	if cfg.MaxFolders != DefaultMaxFolders {
		t.Errorf("MaxFolders = %d", cfg.MaxFolders)
	}
	if cfg.CommandTimeout != DefaultCommandTimeout*time.Second {
		t.Errorf("CommandTimeout = %v", cfg.CommandTimeout)
	}
}

// A typo in a unit's environment falls back to the default rather than
// stopping the sweep.
func TestLoadToleratesNonIntegerSettings(t *testing.T) {
	t.Setenv(EnvLidarrURL, "http://lidarr:8686")
	t.Setenv(EnvLidarrKeyFile, "/run/secrets/key")
	t.Setenv(EnvDownloadDir, "/downloads")
	t.Setenv(EnvMinAgeMinutes, "ninety")

	var logs bytes.Buffer
	cfg, err := Load(slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.MinAge != DefaultMinAgeMinutes*time.Minute {
		t.Errorf("MinAge = %v, want the default", cfg.MinAge)
	}
	if !strings.Contains(logs.String(), "non-integer") {
		t.Errorf("the bad value was not reported: %s", logs.String())
	}
}

func TestOptionDefaults(t *testing.T) {
	got := Options{}.withDefaults()

	if got.PollInterval != pollInterval {
		t.Errorf("PollInterval = %v, want %v", got.PollInterval, pollInterval)
	}
	if got.Client == nil || got.Now == nil {
		t.Error("client or clock missing")
	}
}
