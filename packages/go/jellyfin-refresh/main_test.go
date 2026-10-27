package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"homelab/app"
	"homelab/jellyfin-refresh/internal/refresh"
)

// run drives the command with args, replacing the work so the test asserts on
// configuration alone — the refresh itself is covered in the internal package.
func run(t *testing.T, cfg *refresh.Config, args ...string) {
	t.Helper()

	a := newApp(cfg)
	a.Args = args
	a.Stdout = &bytes.Buffer{}
	a.Stderr = &bytes.Buffer{}
	a.Run = func(context.Context, *app.Env) error { return nil }

	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute(%v) error = %v", args, err)
	}
}

// Every flag must reach its Config field. lib/app proves the general mapping
// from a flag name to a config key; what it cannot know is whether this tool's
// flag names match this tool's json tags. A mismatch is silent — the flag
// parses, the field keeps its zero value, and the hook refreshes nothing.
func TestFlagsReachConfig(t *testing.T) {
	t.Parallel()

	cfg := refresh.Defaults()
	run(t, &cfg,
		"--base", "http://jellyfin.test:8096",
		"--api-key-file", "/run/secrets/jellyfin-apikey",
		"--event-env", "radarr_eventtype",
		"--path-env", "radarr_movie_path,radarr_moviefile_path",
	)

	want := refresh.Config{
		Base:       "http://jellyfin.test:8096",
		APIKeyFile: "/run/secrets/jellyfin-apikey",
		EventEnv:   "radarr_eventtype",
		PathEnv:    "radarr_movie_path,radarr_moviefile_path",
	}
	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("config mismatch (-want +got):\n%s", diff)
	}
}

// Every setting must also arrive through the environment, because that is how
// the hook is actually configured: a modern Radarr custom script takes no
// arguments at all, so the *arr unit exports HOMELAB_JELLYFIN_REFRESH_* and the
// hook inherits them. A key that only worked as a flag would leave the deployed
// hook unauthenticated.
func TestConfigComesFromTheEnvironment(t *testing.T) {
	t.Setenv("HOMELAB_JELLYFIN_REFRESH_BASE", "http://jellyfin.test:8096")
	t.Setenv("HOMELAB_JELLYFIN_REFRESH_API_KEY", "abc123")
	t.Setenv("HOMELAB_JELLYFIN_REFRESH_EVENT_ENV", "radarr_eventtype")
	t.Setenv("HOMELAB_JELLYFIN_REFRESH_PATH_ENV",
		"radarr_movie_path,radarr_moviefile_path")

	cfg := refresh.Defaults()
	run(t, &cfg)

	want := refresh.Config{
		Base:     "http://jellyfin.test:8096",
		APIKey:   "abc123",
		EventEnv: "radarr_eventtype",
		PathEnv:  "radarr_movie_path,radarr_moviefile_path",
	}
	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("config mismatch (-want +got):\n%s", diff)
	}
}

// A positional argument is a mistake, not an input: everything here is flags,
// and an *arr passes its event in the environment.
func TestPositionalArgumentIsRejected(t *testing.T) {
	t.Parallel()

	cfg := refresh.Defaults()
	a := newApp(&cfg)
	a.Args = []string{"/mnt/media/library/movies"}
	a.Stdout = &bytes.Buffer{}
	a.Stderr = &bytes.Buffer{}
	a.Run = func(context.Context, *app.Env) error { return nil }

	err := a.Execute(context.Background())
	if !app.IsUsageError(err) {
		t.Errorf("err = %v, want a usage error", err)
	}
}
