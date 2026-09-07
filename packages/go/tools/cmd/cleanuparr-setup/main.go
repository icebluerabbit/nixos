// Command cleanuparr-setup completes Cleanuparr's first-run setup and exports
// its API key for configuratarr.
package main

import (
	"context"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/tools/internal/cleanuparrsetup"
)

// newApp wires the command line to the work. Split out of main only so that
// main_test can drive it: the flag names and their mapping onto Config's keys
// are the one thing here that can be wrong, and a typo in either half would
// otherwise be silent until the unit ran.
func newApp(cfg *cleanuparrsetup.Config) app.App {
	return app.App{
		Name:  "cleanuparr-setup",
		Short: "Complete Cleanuparr first-run setup and export its API key",
		Long: "Each credential takes either --x or --x-file, so the host decides " +
			"whether the value is a secret. Prefer the file form for the " +
			"password: a literal lands in argv, which is world-readable " +
			"through /proc.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("base", cfg.Base, "Cleanuparr base URL")
			f.String("key-file", cfg.KeyFile, "where to write CLEANUPARR_API_KEY=…")
			f.String("admin-user", "", "admin account name")
			f.String("admin-user-file", "", "file holding the admin account name")
			f.String("admin-password", "", "admin password (prefer --admin-password-file)")
			f.String("admin-password-file", "", "file holding the admin password")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			return cleanuparrsetup.Run(ctx, env, *cfg)
		},
	}
}

func main() {
	cfg := cleanuparrsetup.Defaults()
	newApp(&cfg).Main()
}
