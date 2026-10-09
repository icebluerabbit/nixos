package main

import (
	"bytes"
	"context"
	"testing"

	"homelab/app"
	"homelab/topology/internal/render"
)

// run drives the command with args, replacing the work so the test asserts on
// configuration alone — rendering is covered in the internal packages.
func run(t *testing.T, cfg *render.Config, args ...string) {
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
// parses, the field stays at its default, and `just topology` writes the
// wrong view into the wrong marker.
func TestFlagsReachConfig(t *testing.T) {
	t.Parallel()

	cfg := render.Defaults()
	run(t, &cfg,
		"--view", "logical",
		"--host", "desktop",
		"--into", "README.md",
		"--marker", "topology-logical",
	)

	want := render.Config{
		View:   "logical",
		Host:   "desktop",
		Into:   "README.md",
		Marker: "topology-logical",
	}
	if cfg != want {
		t.Errorf("config = %+v, want %+v", cfg, want)
	}
}

// Defaults survive when a flag is not given, which is what makes
// `topology --view physical --into README.md --marker topology-physical`
// enough for the Justfile recipe.
func TestUnsetFlagsKeepDefaults(t *testing.T) {
	t.Parallel()

	cfg := render.Defaults()
	defaults := render.Defaults()

	run(t, &cfg, "--into", "README.md")

	if cfg.View != defaults.View {
		t.Errorf("view = %q, want %q", cfg.View, defaults.View)
	}
	if cfg.Host != defaults.Host {
		t.Errorf("host = %q, want %q", cfg.Host, defaults.Host)
	}
	if cfg.Marker != defaults.Marker {
		t.Errorf("marker = %q, want %q", cfg.Marker, defaults.Marker)
	}
}
