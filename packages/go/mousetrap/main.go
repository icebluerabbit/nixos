// Command mousetrap is the MyAnonaMouse grab authority. See README.md for the
// requirements. main is thin: declare the command line, hand off to the server.
package main

import (
	"context"

	"homelab/app"
	"homelab/mousetrap/internal/config"
	"homelab/mousetrap/internal/server"
	"homelab/mousetrap/internal/types"
)

// newApp wires the command line to the work. Split out of main only so that
// main_test can drive it: mousetrap takes some sixty flags and every one of
// them has to reach its field, which a typo in either the flag name or the
// json key would break silently.
func newApp(cfg *types.Config) app.App {
	return app.App{
		Name:  "mousetrap",
		Short: "MyAnonaMouse grab authority",
		Long: "mousetrap is the sole MAM grab authority: it takes autobrr " +
			"announces and Torznab searches, applies acceptance and " +
			"grab policy against the live account state, hands accepted " +
			"releases to the download client, and records every decision " +
			"in its SQLite ledgers.",
		Config: cfg,
		Flags:  config.Flags,
		Run: func(ctx context.Context, env *app.Env) error {
			return server.Run(ctx, env, *cfg)
		},
	}
}

func main() {
	cfg := config.Defaults()
	newApp(&cfg).Main()
}
