// Command managarr-seasonpackarr relays seasonpackarr's webhook to managarr.
//
// seasonpackarr posts when it tries to satisfy a season pack from episodes already held. Whether it matched and what it matched against exist only here — a successful pack looks, downstream, like any other hardlink.
//
// A pure relay: seasonpackarr has no history to walk and no API worth reading, so
// there is nothing to enrich and nothing to page. What the sidecar still buys
// is uniformity — managarr ingests one envelope from every service rather than
// one shape per service — and a /healthz and /info like every other sidecar,
// which is what makes the fleet enumerable.
//
// The body is forwarded unparsed. A relay that understood its payload would
// need a release every time a template changed.
//
// The work is lib/sidecar/relaysvc's.
package main

import (
	"context"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/sidecar"
	"homelab/sidecar/relaysvc"
)

func defaults() sidecar.Config { return sidecar.Defaults() }

// newApp wires the command line to the work. Split out of main only so
// main_test can drive it: the flag names and their mapping onto Config's keys
// are the one thing here that can be wrong, and a typo in either half would be
// silent until the unit ran.
func newApp(cfg *sidecar.Config) app.App {
	return app.App{
		Name:  "managarr-seasonpackarr",
		Short: "Relay seasonpackarr's webhook to managarr",
		Long: "A pure relay. seasonpackarr keeps no history and has no API worth " +
			"reading, so this normalises its webhook into the managarr " +
			"envelope and forwards the body unparsed.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("addr", cfg.Addr, "listen address")
			f.String("managarr-url", cfg.ManagarrURL, "managarr base URL; empty disables relaying")
			f.Duration("shutdown-timeout", cfg.ShutdownTimeout, "grace period for in-flight requests")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			return sidecar.Run(ctx, env, *cfg, relaysvc.New("seasonpackarr", "name"))
		},
	}
}

func main() {
	cfg := defaults()
	newApp(&cfg).Main()
}
