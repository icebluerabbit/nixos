package refresh

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"homelab/app"
	"homelab/secret"
)

// testEnv is the Env a hook would get, with the logs thrown away.
func testEnv() *app.Env {
	return &app.Env{Logger: slog.New(slog.DiscardHandler)}
}

// recorded is what the fake Jellyfin saw.
type recorded struct {
	method string
	path   string
	auth   string
	body   mediaUpdated
}

// fakeJellyfin serves the two routes this tool calls and records the request.
// status is what /Library/Media/Updated answers with; 0 means 204, which is
// what Jellyfin returns.
func fakeJellyfin(t *testing.T, status int) (*httptest.Server, *recorded) {
	t.Helper()

	got := &recorded{}
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			got.method = r.Method
			got.path = r.URL.Path
			got.auth = r.Header.Get("Authorization")

			if status != 0 {
				w.WriteHeader(status)
				return
			}

			if r.URL.Path == pathSystemInfo {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"Version":"12.1.0"}`))
				return
			}

			_ = json.NewDecoder(r.Body).Decode(&got.body)
			w.WriteHeader(http.StatusNoContent)
		}))
	t.Cleanup(server.Close)

	return server, got
}

// A configuration missing either half of what it needs is refused before any
// request goes out, so a mistyped unit fails loudly instead of posting an
// update Jellyfin cannot act on.
func TestValidate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		cfg  Config
		want error
	}{
		"no base": {
			cfg:  Config{PathEnv: "radarr_movie_path"},
			want: ErrNoBase,
		},
		"no path env": {
			cfg:  Config{Base: "http://jellyfin.test:8096"},
			want: ErrNoPathEnv,
		},
		"complete": {
			cfg: Config{
				Base:    "http://jellyfin.test:8096",
				PathEnv: "radarr_movie_path",
			},
			want: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := tc.cfg.Validate(); !errors.Is(err, tc.want) {
				t.Errorf("Validate() error = %v, want %v", err, tc.want)
			}
		})
	}
}

// The first variable that actually carries a path wins. An *arr exports the
// variables for events it is not sending, empty, so "set" is not enough.
func TestSelectPath(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"radarr_movie_path":     "",
		"radarr_moviefile_path": "/mnt/media/library/movies/Heat (1995)/heat.mkv",
	}
	lookup := func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}

	names := []string{"radarr_movie_path", "radarr_moviefile_path"}
	if got, want := SelectPath(lookup, names), env["radarr_moviefile_path"]; got != want {
		t.Errorf("SelectPath() = %q, want %q", got, want)
	}

	if got := SelectPath(lookup, []string{"sonarr_series_path"}); got != "" {
		t.Errorf("SelectPath() on an unset variable = %q, want empty", got)
	}
}

// The path variables arrive as one comma-separated string, because the
// environment is the only route a Radarr custom script has.
func TestPathEnvNames(t *testing.T) {
	t.Parallel()

	cfg := Config{PathEnv: " radarr_movie_path , radarr_moviefile_path ,"}
	want := []string{"radarr_movie_path", "radarr_moviefile_path"}

	if diff := cmp.Diff(want, cfg.PathEnvNames()); diff != "" {
		t.Errorf("PathEnvNames() mismatch (-want +got):\n%s", diff)
	}
	if got := (Config{}).PathEnvNames(); len(got) != 0 {
		t.Errorf("PathEnvNames() on an empty value = %v, want none", got)
	}
}

// A file is reduced to its directory, so the subtitles and artwork imported
// beside it are refreshed too; a directory and a path that does not exist are
// passed through.
func TestDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "movie.mkv")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if got := Directory(file); got != dir {
		t.Errorf("Directory(file) = %q, want %q", got, dir)
	}
	if got := Directory(dir); got != dir {
		t.Errorf("Directory(dir) = %q, want %q", got, dir)
	}

	missing := filepath.Join(dir, "gone", "movie.mkv")
	if got := Directory(missing); got != missing {
		t.Errorf("Directory(missing) = %q, want it unchanged", got)
	}
}

// The whole point of the tool: an import event becomes one path update carrying
// the header Jellyfin 12 accepts.
func TestRunRefreshesThePath(t *testing.T) {
	server, got := fakeJellyfin(t, 0)

	movie := t.TempDir()
	t.Setenv("radarr_eventtype", "Download")
	t.Setenv("radarr_movie_path", movie)

	cfg := Config{
		Base:     server.URL,
		APIKey:   "abc123",
		EventEnv: "radarr_eventtype",
		PathEnv:  "radarr_movie_path",
	}
	if err := Run(context.Background(), testEnv(), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got.method != http.MethodPost || got.path != pathMediaUpdated {
		t.Errorf("request = %s %s, want POST %s",
			got.method, got.path, pathMediaUpdated)
	}
	if want := `MediaBrowser Token="abc123"`; got.auth != want {
		t.Errorf("Authorization = %q, want %q", got.auth, want)
	}

	want := mediaUpdated{Updates: []update{{Path: movie, UpdateType: updateTypeCreated}}}
	if diff := cmp.Diff(want, got.body); diff != "" {
		t.Errorf("body mismatch (-want +got):\n%s", diff)
	}
}

// The key may arrive as a file instead, which is how a sops secret or a systemd
// credential reaches a hook.
func TestRunReadsTheKeyFromAFile(t *testing.T) {
	server, got := fakeJellyfin(t, 0)

	keyFile := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(keyFile, []byte("filekey\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("radarr_movie_path", t.TempDir())

	cfg := Config{
		Base:       server.URL,
		APIKeyFile: keyFile,
		PathEnv:    "radarr_movie_path",
	}
	if err := Run(context.Background(), testEnv(), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if want := `MediaBrowser Token="filekey"`; got.auth != want {
		t.Errorf("Authorization = %q, want %q", got.auth, want)
	}
}

// A Test event has no path, so it proves the credential instead — otherwise the
// operator's Test button would report the failure this tool exists to fix.
func TestRunAnswersATestEvent(t *testing.T) {
	server, got := fakeJellyfin(t, 0)

	t.Setenv("radarr_eventtype", TestEvent)

	cfg := Config{
		Base:     server.URL,
		APIKey:   "abc123",
		EventEnv: "radarr_eventtype",
		PathEnv:  "radarr_movie_path",
	}
	if err := Run(context.Background(), testEnv(), cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got.method != http.MethodGet || got.path != pathSystemInfo {
		t.Errorf("request = %s %s, want GET %s",
			got.method, got.path, pathSystemInfo)
	}
}

// A rejected key is reported as a failure of this tool's own check, so the
// *arr's log says which half is wrong.
func TestRunReportsARejectedCredential(t *testing.T) {
	server, _ := fakeJellyfin(t, http.StatusUnauthorized)

	t.Setenv("radarr_eventtype", TestEvent)

	cfg := Config{
		Base:     server.URL,
		APIKey:   "stale",
		EventEnv: "radarr_eventtype",
		PathEnv:  "radarr_movie_path",
	}
	if err := Run(context.Background(), testEnv(), cfg); err == nil {
		t.Fatal("Run() error = nil, want a credential failure")
	}
}

// An event whose path variables are all unset is an error, not a silent
// no-op: something changed and Jellyfin was not told.
func TestRunWithoutAPathFails(t *testing.T) {
	server, _ := fakeJellyfin(t, 0)

	t.Setenv("radarr_movie_path", "")

	cfg := Config{
		Base:    server.URL,
		APIKey:  "abc123",
		PathEnv: "radarr_movie_path",
	}
	err := Run(context.Background(), testEnv(), cfg)
	if !errors.Is(err, ErrNoPath) {
		t.Errorf("Run() error = %v, want ErrNoPath", err)
	}
}

// A refresh Jellyfin refuses is surfaced with the path in it.
func TestRunReportsAFailedRefresh(t *testing.T) {
	server, _ := fakeJellyfin(t, http.StatusInternalServerError)

	t.Setenv("radarr_movie_path", t.TempDir())

	cfg := Config{
		Base:    server.URL,
		APIKey:  "abc123",
		PathEnv: "radarr_movie_path",
	}
	if err := Run(context.Background(), testEnv(), cfg); err == nil {
		t.Error("Run() error = nil, want a refresh failure")
	}
}

// --dry-run stops before the update, so a hook can be tried by hand against
// the live server without moving anything.
func TestRunDryRunPostsNothing(t *testing.T) {
	server, got := fakeJellyfin(t, 0)

	t.Setenv("radarr_movie_path", t.TempDir())

	cfg := Config{
		Base:    server.URL,
		APIKey:  "abc123",
		PathEnv: "radarr_movie_path",
	}
	env := testEnv()
	env.DryRun = true

	if err := Run(context.Background(), env, cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got.method != "" {
		t.Errorf("request = %s %s, want none", got.method, got.path)
	}
}

// Both halves of the credential pair at once is a host mistake worth refusing:
// the two values disagreeing silently would be worse.
func TestRunRejectsAnAmbiguousCredential(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Base:       "http://jellyfin.test:8096",
		APIKey:     "abc123",
		APIKeyFile: "/run/secrets/jellyfin-apikey",
		PathEnv:    "radarr_movie_path",
	}
	err := Run(context.Background(), testEnv(), cfg)
	if !errors.Is(err, secret.ErrAmbiguous) {
		t.Errorf("Run() error = %v, want secret.ErrAmbiguous", err)
	}
}

// A configuration that cannot work never reaches the network.
func TestRunValidatesFirst(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), testEnv(), Config{})
	if !errors.Is(err, ErrNoBase) {
		t.Errorf("Run() error = %v, want ErrNoBase", err)
	}
}
