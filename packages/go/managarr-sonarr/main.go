// Command managarr-sonarr serves Sonarr's history and notifications to managarr.
//
// Runs inside Sonarr's own container and talks to it over the loopback, so the
// API key never crosses the VLAN — it lives only where the service that owns it
// already has it.
//
// Two things it does that managarr calling Sonarr directly cannot:
//
//   - A since-cursor. Sonarr's /history takes page, pageSize and a sort and
//     nothing else, so without one every reconcile re-walks the whole history.
//   - Webhook enrichment. A Grab names the release and the download id but not
//     what the download client called it, and the queue record that knows is
//     gone within minutes.
//
// The work is lib/sidecar/arrsvc's: Sonarr, Radarr and Lidarr are one API under
// different nouns, so the three binaries differ only by the Kind below.
package main

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/clients/arr"
	"homelab/httpx"
	"homelab/sidecar"
	"homelab/sidecar/arrsvc"
)

// config is the shared contract flags plus what only an *arr sidecar needs.
type config struct {
	// Squashed: without this the embedded struct is a field literally named
	// "Config", which collides with the framework's own --config flag (a file
	// path, a string) and fails every start with "expected a map or struct,
	// got string".
	sidecar.Config `json:",squash"`
	// UpstreamURL is Sonarr, on the loopback inside this container.
	UpstreamURL string `json:"upstream_url"`
	// APIKey comes from the environment, never a flag: a key on a command
	// line is readable from the process table by anything in the container.
	APIKey string `json:"api_key"`
}

func defaults() config {
	return config{
		Config:      sidecar.Defaults(),
		UpstreamURL: "http://127.0.0.1:8989",
	}
}

// newApp wires the command line to the work. Split out of main only so
// main_test can drive it: the flag names and their mapping onto config's keys
// are the one thing here that can be wrong, and a typo in either half would be
// silent until the unit ran.
func newApp(cfg *config) app.App {
	return app.App{
		Name:  "managarr-sonarr",
		Short: "Serve Sonarr's history and notifications to managarr",
		Long: "Runs inside Sonarr's container, reads its history with a cursor " +
			"it does not natively offer, and enriches its notifications with " +
			"the queue detail that expires minutes after a grab.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("addr", cfg.Addr, "listen address")
			f.String("upstream-url", cfg.UpstreamURL, "Sonarr base URL, on the loopback")
			f.String("managarr-url", cfg.ManagarrURL, "managarr base URL; empty disables relaying")
			f.Duration("shutdown-timeout", cfg.ShutdownTimeout, "grace period for in-flight requests")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			if cfg.APIKey == "" {
				return fmt.Errorf("no API key: set HOMELAB_MANAGARR_SONARR_API_KEY")
			}
			c, err := arr.New(arr.Sonarr,
				httpx.Config{BaseURL: cfg.UpstreamURL, Logger: env.Logger}, cfg.APIKey)
			if err != nil {
				return fmt.Errorf("sonarr client: %w", err)
			}
			return sidecar.Run(ctx, env, cfg.Config, arrsvc.New(c, cfg.UpstreamURL))
		},
	}
}

func main() {
	cfg := defaults()
	newApp(&cfg).Main()
}
