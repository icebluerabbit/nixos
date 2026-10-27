// Command managarr-bazarr serves Bazarr's subtitle history to managarr.
//
// Bazarr is the only source of subtitle state, and the only source of an
// ABSENCE: which languages are still wanted cannot be observed on disk, because
// a missing subtitle leaves nothing behind. Its history is also the only record
// of which provider a subtitle came from and what it scored.
//
// No webhook to receive. Bazarr notifies through Apprise providers rather than
// a plain HTTP callback, so this reports /events unsupported rather than
// accepting bodies that will never arrive.
//
// The work is lib/sidecar/bazarrsvc's.
package main

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/clients/bazarr"
	"homelab/httpx"
	"homelab/sidecar"
	"homelab/sidecar/bazarrsvc"
)

type config struct {
	// Squashed: without this the embedded struct is a field literally named
	// "Config", which collides with the framework's own --config flag (a file
	// path, a string) and fails every start with "expected a map or struct,
	// got string".
	sidecar.Config `json:",squash"`
	// UpstreamURL is Bazarr, on the loopback inside this container.
	UpstreamURL string `json:"upstream_url"`
	// APIKey comes from the environment, never a flag: a key on a command
	// line is readable from the process table by anything in the container.
	APIKey string `json:"api_key"`
}

func defaults() config {
	return config{
		Config:      sidecar.Defaults(),
		UpstreamURL: "http://127.0.0.1:6767",
	}
}

// newApp wires the command line to the work. Split out of main only so
// main_test can drive it: the flag names and their mapping onto config's keys
// are the one thing here that can be wrong, and a typo in either half would be
// silent until the unit ran.
func newApp(cfg *config) app.App {
	return app.App{
		Name:  "managarr-bazarr",
		Short: "Serve Bazarr's subtitle history to managarr",
		Long: "Bazarr is the only source of subtitle state and the only source " +
			"of an absence — a language still wanted leaves nothing on disk " +
			"to observe. This reads its history with a cursor and serves the " +
			"blacklist it keeps alongside.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("addr", cfg.Addr, "listen address")
			f.String("upstream-url", cfg.UpstreamURL, "Bazarr base URL, on the loopback")
			f.String("managarr-url", cfg.ManagarrURL, "managarr base URL; empty disables relaying")
			f.Duration("shutdown-timeout", cfg.ShutdownTimeout, "grace period for in-flight requests")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			if cfg.APIKey == "" {
				return fmt.Errorf("no API key: set HOMELAB_MANAGARR_BAZARR_API_KEY")
			}
			c, err := bazarr.New(
				httpx.Config{BaseURL: cfg.UpstreamURL, Logger: env.Logger}, cfg.APIKey)
			if err != nil {
				return fmt.Errorf("bazarr client: %w", err)
			}
			return sidecar.Run(ctx, env, cfg.Config, bazarrsvc.New(c, cfg.UpstreamURL))
		},
	}
}

func main() {
	cfg := defaults()
	newApp(&cfg).Main()
}
