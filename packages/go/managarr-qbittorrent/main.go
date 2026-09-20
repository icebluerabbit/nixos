// Command managarr-qbittorrent serves qBittorrent's state and completions to
// managarr.
//
// The one sidecar here that adds a capability rather than a cursor.
//
// qBittorrent keeps no history. Its API reports what it holds right now, so a
// torrent added and removed between two hourly passes leaves nothing behind —
// no record that it existed, what it was, or where its bytes went. Every other
// service in this stack persists its own past and can be backfilled from it;
// this one cannot, which makes a missed event here real data loss rather than
// latency.
//
// What it does have is one event: an external program run on completion. That
// program posts to /events here, and this fills in the file list, the trackers
// and the sizes that the command line leaves out — none of which are reachable
// once a *arr has imported the torrent and removed it.
//
// The work is lib/sidecar/qbittorrentsvc's.
package main

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/clients/qbittorrent"
	"homelab/httpx"
	"homelab/sidecar"
	"homelab/sidecar/qbittorrentsvc"
)

type config struct {
	// Squashed: without this the embedded struct is a field literally named
	// "Config", which collides with the framework's own --config flag (a file
	// path, a string) and fails every start with "expected a map or struct,
	// got string".
	sidecar.Config `json:",squash"`
	// UpstreamURL is the WebUI, on the loopback inside this container.
	UpstreamURL string `json:"upstream_url"`
}

func defaults() config {
	return config{
		Config:      sidecar.Defaults(),
		UpstreamURL: "http://127.0.0.1:8080",
	}
}

// newApp wires the command line to the work. Split out of main only so
// main_test can drive it: the flag names and their mapping onto config's keys
// are the one thing here that can be wrong, and a typo in either half would be
// silent until the unit ran.
func newApp(cfg *config) app.App {
	return app.App{
		Name:  "managarr-qbittorrent",
		Short: "Serve qBittorrent's state and completions to managarr",
		Long: "qBittorrent keeps no history, so a torrent removed between two " +
			"passes leaves nothing behind. This runs beside it, reports new " +
			"torrents by added_on, and enriches the completion hook with the " +
			"file list and trackers that vanish when the torrent does.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("addr", cfg.Addr, "listen address")
			f.String("upstream-url", cfg.UpstreamURL, "qBittorrent WebUI base URL, on the loopback")
			f.String("managarr-url", cfg.ManagarrURL, "managarr base URL; empty disables relaying")
			f.Duration("shutdown-timeout", cfg.ShutdownTimeout, "grace period for in-flight requests")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			c, err := qbittorrent.New(httpx.Config{BaseURL: cfg.UpstreamURL, Logger: env.Logger})
			if err != nil {
				return fmt.Errorf("qbittorrent client: %w", err)
			}
			return sidecar.Run(ctx, env, cfg.Config, qbittorrentsvc.New(c, cfg.UpstreamURL))
		},
	}
}

func main() {
	cfg := defaults()
	newApp(&cfg).Main()
}
