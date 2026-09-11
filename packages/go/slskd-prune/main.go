// Command slskd-prune deletes slskd searches older than a retention window.
package main

import (
	"context"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/slskd-prune/internal/prune"
)

// newApp wires the command line to the work. Split out of main only so that
// main_test can drive it: the flag names and their mapping onto Config's keys
// are the one thing here that can be wrong, and a typo in either half would
// otherwise be silent until the unit ran.
func newApp(cfg *prune.Config) app.App {
	return app.App{
		Name:  "slskd-prune",
		Short: "Delete slskd searches older than a retention window",
		Long: "slskd has retention settings for transfers, files, logs and the " +
			"share cache, but none for searches, so search.db grows without " +
			"bound and the web UI's search view — which loads every record " +
			"in one response — stalls. This is that missing retention pass. " +
			"Deleting frees pages for reuse but does not shrink search.db; " +
			"that needs a VACUUM with slskd stopped.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("base", cfg.Base, "slskd base URL")
			f.Int("retention-days", cfg.RetentionDays,
				"delete searches that started more than this many days ago")
			f.Int("workers", cfg.Workers, "concurrent delete requests")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			return prune.Run(ctx, env, *cfg)
		},
	}
}

func main() {
	cfg := prune.Defaults()
	newApp(&cfg).Main()
}
