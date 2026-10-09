// Command ha-seed-persons merges declared device trackers into Home
// Assistant's existing people. The declaration arrives as JSON on stdin.
package main

import (
	"context"
	"os"

	"homelab/app"
	"homelab/ha-seeders/internal/seed"
)

func newApp() app.App {
	return app.App{
		Name:  "ha-seed-persons",
		Short: "Merge declared device trackers into Home Assistant's people",
		Long: "People are created by onboarding and SSO, never here: one " +
			"fabricated in " + seed.PersonStoragePath + " would have no " +
			"credentials and no way to log in. This only adds device " +
			"trackers to people that already exist, and does nothing at " +
			"all when the registry is absent. Reads the declaration as " +
			"JSON on stdin.",
		Run: func(_ context.Context, env *app.Env) error {
			var persons []seed.Person
			if err := seed.ReadSeed(os.Stdin, &persons); err != nil {
				return err
			}
			return seed.Persons(env.Logger, seed.PersonStoragePath, persons)
		},
	}
}

func main() {
	newApp().Main()
}
