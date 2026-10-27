// Command managarr-autobrr serves autobrr's announce history to managarr.
//
// autobrr is the only record of what was ANNOUNCED. Every release that appeared
// on every tracker's IRC channel passes through it, and it keeps the ones it
// rejected alongside the ones it grabbed — so it answers "was this ever offered
// to me, and why did I not take it", which nothing downstream can.
//
// The rejections are most of the corpus and exist nowhere else: a release the
// filter declined never reaches a *arr, a torrent client or a tracker's history.
//
// The work is lib/sidecar/autobrrsvc's.
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/clients/autobrr"
	"homelab/httpx"
	"homelab/sidecar"
	"homelab/sidecar/autobrrsvc"
)

type config struct {
	// Squashed: without this the embedded struct is a field literally named
	// "Config", which collides with the framework's own --config flag (a file
	// path, a string) and fails every start with "expected a map or struct,
	// got string".
	sidecar.Config `json:",squash"`
	// UpstreamURL is autobrr, on the loopback inside this container.
	UpstreamURL string `json:"upstream_url"`
	// APIKey comes from the environment, never a flag: a key on a command
	// line is readable from the process table by anything in the container.
	APIKey string `json:"api_key"`
}

func defaults() config {
	return config{
		Config:      sidecar.Defaults(),
		UpstreamURL: "http://127.0.0.1:7474",
	}
}

// newApp wires the command line to the work. Split out of main only so
// main_test can drive it: the flag names and their mapping onto config's keys
// are the one thing here that can be wrong, and a typo in either half would be
// silent until the unit ran.
func newApp(cfg *config) app.App {
	return app.App{
		Name:  "managarr-autobrr",
		Short: "Serve autobrr's announce history to managarr",
		Long: "autobrr is the only record of what was announced, including the " +
			"releases it rejected — which are most of the corpus and exist " +
			"nowhere else. This reads that history with a cursor autobrr's " +
			"newest-first offset paging cannot provide.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("addr", cfg.Addr, "listen address")
			f.String("upstream-url", cfg.UpstreamURL, "autobrr base URL, on the loopback")
			f.String("managarr-url", cfg.ManagarrURL, "managarr base URL; empty disables relaying")
			f.Duration("shutdown-timeout", cfg.ShutdownTimeout, "grace period for in-flight requests")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			if cfg.APIKey == "" {
				return fmt.Errorf("no API key: set HOMELAB_MANAGARR_AUTOBRR_API_KEY")
			}
			c, err := autobrr.New(httpx.Config{
				BaseURL: cfg.UpstreamURL,
				Logger:  env.Logger,
				Headers: http.Header{"X-API-Token": {cfg.APIKey}},
			})
			if err != nil {
				return fmt.Errorf("autobrr client: %w", err)
			}
			return sidecar.Run(ctx, env, cfg.Config, autobrrsvc.New(c, cfg.UpstreamURL))
		},
	}
}

func main() {
	cfg := defaults()
	newApp(&cfg).Main()
}
