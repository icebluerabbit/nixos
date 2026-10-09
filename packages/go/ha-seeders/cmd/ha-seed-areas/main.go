// Command ha-seed-areas merges declared areas into Home Assistant's area
// registry. The declaration arrives as JSON on stdin.
package main

import (
	"context"
	"os"

	"homelab/app"
	"homelab/ha-seeders/internal/seed"
)

func newApp() app.App {
	return app.App{
		Name:  "ha-seed-areas",
		Short: "Merge declared areas into Home Assistant's area registry",
		Long: "HA has no declarative configuration for areas, so the " +
			"declared set is merged into " + seed.AreaStoragePath +
			" before home-assistant.service starts. Append-only: an area " +
			"already there is left alone, because it may have been " +
			"renamed or given a floor in the UI. Reads the declaration as " +
			"JSON on stdin.",
		Run: func(_ context.Context, env *app.Env) error {
			var areas []seed.Area
			if err := seed.ReadSeed(os.Stdin, &areas); err != nil {
				return err
			}
			return seed.Areas(env.Logger, seed.AreaStoragePath, areas)
		},
	}
}

func main() {
	newApp().Main()
}
