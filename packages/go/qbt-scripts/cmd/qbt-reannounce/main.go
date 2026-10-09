// Command qbt-reannounce forces a tracker announce for zero-progress
// torrents whose trackers have all errored.
package main

import (
	"context"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/qbt-scripts/internal/reannounce"
)

// newApp wires the command line to the work. Split out of main only so that
// main_test can drive it: a flag name that does not match its config key
// parses fine and leaves the field empty, which here means talking to no
// qBittorrent at all.
func newApp(cfg *reannounce.Config) app.App {
	return app.App{
		Name:  "qbt-reannounce",
		Short: "Re-announce zero-progress torrents whose trackers have all errored",
		Long: "A torrent added while its tracker was unreachable keeps the " +
			"announce interval it was given and sits at zero progress " +
			"until that interval elapses. This walks the torrent list, " +
			"finds the ones that have downloaded nothing and whose every " +
			"real tracker reports a failure, and asks the client to " +
			"announce them again. DHT, PeX and LSD rows are not trackers " +
			"and are ignored.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("api", cfg.API, "qBittorrent WebUI base URL")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			return reannounce.Run(ctx, env, *cfg)
		},
	}
}

func main() {
	cfg := reannounce.Defaults()
	newApp(&cfg).Main()
}
