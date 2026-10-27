// Command managarr-slskd serves slskd's event log to managarr.
//
// The event log is the authoritative record of the Soulseek side, and the only
// one. /transfers is a live view that expires a transfer once it finishes, so it
// shows what is in flight and nothing else; the event log is persisted and is
// the only place slskd states the mapping between a peer's remote path and the
// local file it wrote.
//
// That mapping is what closes the music loop. A library file shares an inode
// with the download that produced it — beets imports by move — and the download
// names the peer and the remote path, which names the search result, which names
// the search. Without this log the chain starts at a file with no provenance.
//
// The work is lib/sidecar/slskdsvc's.
package main

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/clients/slskd"
	"homelab/httpx"
	"homelab/sidecar"
	"homelab/sidecar/slskdsvc"
)

type config struct {
	// Squashed: without this the embedded struct is a field literally named
	// "Config", which collides with the framework's own --config flag (a file
	// path, a string) and fails every start with "expected a map or struct,
	// got string".
	sidecar.Config `json:",squash"`
	// UpstreamURL is slskd, on the loopback inside this container.
	UpstreamURL string `json:"upstream_url"`
	// PageSize bounds one page of the event log. Small on purpose: every
	// download event embeds its entire transfer object, so the full dump is
	// tens of megabytes.
	PageSize int `json:"page_size"`
}

func defaults() config {
	return config{
		Config:      sidecar.Defaults(),
		UpstreamURL: "http://127.0.0.1:5030",
		PageSize:    500,
	}
}

// newApp wires the command line to the work. Split out of main only so
// main_test can drive it: the flag names and their mapping onto config's keys
// are the one thing here that can be wrong, and a typo in either half would be
// silent until the unit ran.
func newApp(cfg *config) app.App {
	return app.App{
		Name:  "managarr-slskd",
		Short: "Serve slskd's event log to managarr",
		Long: "slskd's event log is the only persisted record of the Soulseek " +
			"side — /transfers expires a transfer as soon as it finishes — " +
			"and the only place the remote path is mapped to the local file.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("addr", cfg.Addr, "listen address")
			f.String("upstream-url", cfg.UpstreamURL, "slskd base URL, on the loopback")
			f.String("managarr-url", cfg.ManagarrURL, "managarr base URL; empty disables relaying")
			f.Int("page-size", cfg.PageSize, "events per upstream request")
			f.Duration("shutdown-timeout", cfg.ShutdownTimeout, "grace period for in-flight requests")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			// No credentials: slskd runs with authentication.disabled and is
			// gated at Caddy, so its API is open on the loopback.
			c, err := slskd.New(httpx.Config{BaseURL: cfg.UpstreamURL, Logger: env.Logger})
			if err != nil {
				return fmt.Errorf("slskd client: %w", err)
			}
			return sidecar.Run(ctx, env, cfg.Config,
				slskdsvc.New(c, cfg.UpstreamURL, cfg.PageSize))
		},
	}
}

func main() {
	cfg := defaults()
	newApp(&cfg).Main()
}
