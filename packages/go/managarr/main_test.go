package main

import (
	"context"
	"strings"
	"testing"

	"homelab/app"
	"homelab/managarr/internal/server"
)

// The flag names and their mapping onto Config's keys are the one thing in
// main.go that can be wrong, and a typo in either half stays silent until the
// unit runs.
func TestFlagsAppearInHelp(t *testing.T) {
	t.Parallel()

	cfg := server.Defaults()
	a := newApp(&cfg)

	var out strings.Builder
	a.Stdout = &out
	a.Stderr = &out
	a.Args = []string{"--help"}

	if err := a.Execute(t.Context()); err != nil {
		t.Fatalf("--help: %v", err)
	}

	help := out.String()
	for _, flag := range []string{
		"--addr", "--shutdown-timeout", "--db-dsn",
		"--qbittorrent-url", "--sonarr-url", "--radarr-url",
		"--lidarr-url", "--slskd-url",
	} {
		if !strings.Contains(help, flag) {
			t.Errorf("%s missing from --help:\n%s", flag, help)
		}
	}
}

// Every flag must actually land in Config.
//
// This exists because it did not. The struct was tagged `koanf:"db_dsn"` while
// homelab/config unmarshals with the `json` tag, so every field silently kept
// its zero value: the unit was started with a correct --db-dsn on its command
// line and pgx still connected to its own localhost default, crash-looping a
// hundred times before anyone read the error. A tag mismatch produces no
// warning anywhere, so the only defence is asserting the round trip.
func TestFlagsReachConfig(t *testing.T) {
	t.Parallel()

	cfg := server.Defaults()
	a := newApp(&cfg)

	// Run is replaced so the app parses configuration and stops, rather than
	// opening a database and binding a port.
	a.Run = func(context.Context, *app.Env) error { return nil }

	var out strings.Builder
	a.Stdout = &out
	a.Stderr = &out
	a.Args = []string{
		"--addr", ":1234",
		"--db-dsn", "postgres://u@example:5432/db",
		"--qbittorrent-url", "http://qbit:8080",
		"--sonarr-url", "http://sonarr:8989",
		"--radarr-url", "http://radarr:7878",
		"--lidarr-url", "http://lidarr:8686",
		"--slskd-url", "http://slskd:5030",
	}

	if err := a.Execute(t.Context()); err != nil {
		t.Fatalf("execute: %v", err)
	}

	for _, tc := range []struct{ name, got, want string }{
		{"addr", cfg.Addr, ":1234"},
		{"db_dsn", cfg.DBDSN, "postgres://u@example:5432/db"},
		{"qbittorrent_url", cfg.QBittorrentURL, "http://qbit:8080"},
		{"sonarr_url", cfg.SonarrURL, "http://sonarr:8989"},
		{"radarr_url", cfg.RadarrURL, "http://radarr:7878"},
		{"lidarr_url", cfg.LidarrURL, "http://lidarr:8686"},
		{"slskd_url", cfg.SlskdURL, "http://slskd:5030"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// A unit that passes no source flags at all must still be usable: it records
// webhooks and simply has nothing to reconcile.
func TestDefaults(t *testing.T) {
	t.Parallel()

	cfg := server.Defaults()
	if cfg.Addr == "" || cfg.ShutdownTimeout == 0 {
		t.Fatalf("incomplete defaults: %+v", cfg)
	}
}
