// Command ha-seed-config-entries merges declared config entries into Home
// Assistant's registry. The declaration arrives as JSON on stdin, with
// $VAR references expanded from the environment so a secret never has to be
// written into the Nix store.
package main

import (
	"context"
	"os"
	"time"

	"homelab/app"
	"homelab/ha-seeders/internal/seed"
)

func newApp() app.App {
	return app.App{
		Name:  "ha-seed-config-entries",
		Short: "Merge declared config entries into Home Assistant's registry",
		Long: "HA has no declarative configuration for integrations, so " +
			"the declared entries are merged into " + seed.EntryStoragePath +
			" before home-assistant.service starts. An existing entry has " +
			"its data keys, identity fields and subentries reconciled, so " +
			"a re-seed repairs one that drifted. Reads the declaration as " +
			"JSON on stdin, expanding $VAR from the environment.",
		Run: func(_ context.Context, env *app.Env) error {
			var entries []seed.Entry
			if err := seed.ReadSeedExpanded(os.Stdin, os.LookupEnv, &entries); err != nil {
				return err
			}
			return seed.Entries(env.Logger, seed.EntryStoragePath, entries, time.Now())
		},
	}
}

func main() {
	newApp().Main()
}
