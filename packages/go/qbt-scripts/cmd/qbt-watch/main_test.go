package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"homelab/app"
	"homelab/qbt-scripts/internal/watch"
)

// Every flag must reach its config field. lib/app proves the general mapping
// from a flag name to a config key; what it cannot know is whether this
// tool's flag names match this tool's json tags. A mismatch is silent — the
// flag parses, the field keeps its zero value, and the unit scans nothing or
// moves imports into the working directory.
func TestFlagsReachConfig(t *testing.T) {
	t.Parallel()

	cfg := watch.Defaults()
	a := newApp(&cfg)
	a.Args = []string{
		"--api", "http://127.0.0.1:8080",
		"--watch-dir", "/var/lib/qbittorrent/watch",
		"--backup-dir", "/var/lib/qbittorrent/export",
		"--categories", "movies,tv",
		"--report-url", "http://10.10.30.5:9000/event",
	}
	a.Stdout = &bytes.Buffer{}
	a.Stderr = &bytes.Buffer{}
	a.Run = func(context.Context, *app.Env) error { return nil }

	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := watch.Config{
		API:        "http://127.0.0.1:8080",
		WatchDir:   "/var/lib/qbittorrent/watch",
		BackupDir:  "/var/lib/qbittorrent/export",
		Categories: "movies,tv",
		ReportURL:  "http://10.10.30.5:9000/event",
	}
	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("config (-want +got):\n%s", diff)
	}
}

// --report-url is optional: the unit omits it when no recorder is
// configured, and the tool must then simply not report.
func TestReportURLIsOptional(t *testing.T) {
	t.Parallel()

	cfg := watch.Defaults()
	a := newApp(&cfg)
	a.Args = []string{
		"--api", "http://127.0.0.1:8080",
		"--watch-dir", "/watch", "--backup-dir", "/export",
		"--categories", "movies",
	}
	a.Stdout = &bytes.Buffer{}
	a.Stderr = &bytes.Buffer{}
	a.Run = func(context.Context, *app.Env) error { return nil }

	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if cfg.ReportURL != "" {
		t.Errorf("ReportURL = %q, want it empty", cfg.ReportURL)
	}
}
