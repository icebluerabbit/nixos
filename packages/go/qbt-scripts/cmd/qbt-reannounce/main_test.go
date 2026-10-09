package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"homelab/app"
	"homelab/qbt-scripts/internal/reannounce"
)

// Every flag must reach its config field. lib/app proves the general mapping
// from a flag name to a config key; what it cannot know is whether this
// tool's flag names match this tool's json tags. A mismatch is silent — the
// flag parses, the field keeps its zero value, and the unit talks to nothing.
func TestFlagsReachConfig(t *testing.T) {
	t.Parallel()

	cfg := reannounce.Defaults()
	a := newApp(&cfg)
	a.Args = []string{"--api", "http://127.0.0.1:8080"}
	a.Stdout = &bytes.Buffer{}
	a.Stderr = &bytes.Buffer{}
	a.Run = func(context.Context, *app.Env) error { return nil }

	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := reannounce.Config{API: "http://127.0.0.1:8080"}
	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("config (-want +got):\n%s", diff)
	}
}
