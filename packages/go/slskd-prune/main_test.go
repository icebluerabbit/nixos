package main

import (
	"bytes"
	"context"
	"testing"

	"homelab/app"
	"homelab/slskd-prune/internal/prune"
)

// run drives the command with args, replacing the work so the test asserts on
// configuration alone — the pruning itself is covered in the internal package.
func run(t *testing.T, cfg *prune.Config, args ...string) {
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
// parses, the field stays at its default, and the tool prunes the wrong window.
func TestFlagsReachConfig(t *testing.T) {
	t.Parallel()

	cfg := prune.Defaults()
	run(t, &cfg,
		"--base", "http://slskd.test:5030",
		"--retention-days", "30",
		"--workers", "3",
	)

	want := prune.Config{
		Base:          "http://slskd.test:5030",
		RetentionDays: 30,
		Workers:       3,
	}
	if cfg != want {
		t.Errorf("config = %+v, want %+v", cfg, want)
	}
}

// Defaults survive when a flag is not given, so the container unit can pass
// only the flags it cares about.
func TestUnsetFlagsKeepDefaults(t *testing.T) {
	t.Parallel()

	cfg := prune.Defaults()
	defaults := prune.Defaults()

	run(t, &cfg, "--retention-days", "14")

	if cfg.Base != defaults.Base {
		t.Errorf("Base = %q, want the default %q", cfg.Base, defaults.Base)
	}
	if cfg.Workers != defaults.Workers {
		t.Errorf("Workers = %d, want the default %d", cfg.Workers, defaults.Workers)
	}
	if cfg.RetentionDays != 14 {
		t.Errorf("RetentionDays = %d, want 14", cfg.RetentionDays)
	}
}

// A positional argument is a mistake, not an input: everything here is flags.
func TestPositionalArgumentIsRejected(t *testing.T) {
	t.Parallel()

	cfg := prune.Defaults()
	a := newApp(&cfg)
	a.Args = []string{"14"}
	a.Stdout = &bytes.Buffer{}
	a.Stderr = &bytes.Buffer{}
	a.Run = func(context.Context, *app.Env) error { return nil }

	err := a.Execute(context.Background())
	if !app.IsUsageError(err) {
		t.Errorf("err = %v, want a usage error", err)
	}
}
