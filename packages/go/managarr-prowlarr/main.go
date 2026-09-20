// Command managarr-prowlarr serves Prowlarr's search history to managarr.
//
// Prowlarr is the only record of the search side of the stack. Every query a
// *arr sent to every indexer, what came back and how long it took live here and
// nowhere else — the *arr keeps what it grabbed, not what it asked — and that
// history is the only evidence for a release that was offered and never taken.
//
// Runs inside Prowlarr's own container and talks to it over the loopback, so
// the API key never crosses the VLAN.
//
// The work is lib/sidecar/prowlarrsvc's.
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/clients/prowlarr"
	"homelab/httpx"
	"homelab/sidecar"
	"homelab/sidecar/prowlarrsvc"
)

// config is the shared contract flags plus what only this sidecar needs.
type config struct {
	// Squashed: without this the embedded struct is a field literally named
	// "Config", which collides with the framework's own --config flag (a file
	// path, a string) and fails every start with "expected a map or struct,
	// got string".
	sidecar.Config `json:",squash"`
	// UpstreamURL is Prowlarr, on the loopback inside this container.
	UpstreamURL string `json:"upstream_url"`
	// APIKey comes from the environment, never a flag: a key on a command
	// line is readable from the process table by anything in the container.
	APIKey string `json:"api_key"`
}

func defaults() config {
	return config{
		Config:      sidecar.Defaults(),
		UpstreamURL: "http://127.0.0.1:9696",
	}
}

// newApp wires the command line to the work. Split out of main only so
// main_test can drive it: the flag names and their mapping onto config's keys
// are the one thing here that can be wrong, and a typo in either half would be
// silent until the unit ran.
func newApp(cfg *config) app.App {
	return app.App{
		Name:  "managarr-prowlarr",
		Short: "Serve Prowlarr's search history to managarr",
		Long: "Runs inside Prowlarr's container and reads its history with a " +
			"cursor it does not natively offer, across all three event " +
			"types — grabs, searches and feed polls.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("addr", cfg.Addr, "listen address")
			f.String("upstream-url", cfg.UpstreamURL, "Prowlarr base URL, on the loopback")
			f.String("managarr-url", cfg.ManagarrURL, "managarr base URL; empty disables relaying")
			f.Duration("shutdown-timeout", cfg.ShutdownTimeout, "grace period for in-flight requests")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			if cfg.APIKey == "" {
				return fmt.Errorf("no API key: set HOMELAB_MANAGARR_PROWLARR_API_KEY")
			}
			c, err := prowlarr.New(httpx.Config{
				BaseURL: cfg.UpstreamURL,
				Logger:  env.Logger,
				Headers: http.Header{"X-Api-Key": {cfg.APIKey}},
			})
			if err != nil {
				return fmt.Errorf("prowlarr client: %w", err)
			}
			return sidecar.Run(ctx, env, cfg.Config, prowlarrsvc.New(c, cfg.UpstreamURL))
		},
	}
}

func main() {
	cfg := defaults()
	newApp(&cfg).Main()
}
