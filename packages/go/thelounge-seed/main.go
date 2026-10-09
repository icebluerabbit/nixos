// Command thelounge-seed merges declared IRC networks into thelounge's
// per-user JSON files.
package main

import (
	"context"

	"homelab/app"
	"homelab/thelounge-seed/internal/seed"
)

// newApp wires the command line to the work. There are no flags: the unit
// configures this through the environment, and internal/seed reads those
// names directly.
func newApp() app.App {
	return app.App{
		Name:  "thelounge-seed",
		Short: "Merge declared IRC networks into thelounge's user files",
		Long: "thelounge owns users/<name>.json and rewrites it at runtime, " +
			"so the declared networks are merged back in on every boot. " +
			"Only the networks key is replaced; sessions and client " +
			"settings are preserved. Reads " + seed.EnvSeedFile + " for the " +
			"declaration and " + seed.EnvUsersDir + " for the state " +
			"directory (default " + seed.DefaultUsersDir + ").",
		Run: func(_ context.Context, env *app.Env) error {
			cfg, err := seed.Load()
			if err != nil {
				return err
			}
			return seed.Run(env, cfg)
		},
	}
}

func main() {
	newApp().Main()
}
