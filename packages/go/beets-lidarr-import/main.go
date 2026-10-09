// Command beets-lidarr-import hands one finished album from beets to Lidarr.
package main

import (
	"context"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/beets-lidarr-import/internal/importer"
)

// newApp wires the command line to the work. Split out of main only so that
// main_test can drive it.
//
// Only the album is a flag. Lidarr's URL, key and import mode arrive through
// the environment under their own names, which the beets unit already sets
// and the hook inherits; the key in particular must not be a flag, because
// argv is readable by anything that can see /proc.
func newApp(cfg *importer.Config) app.App {
	return app.App{
		Name:  "beets-lidarr-import",
		Short: "Hand one finished album from beets to Lidarr",
		Long: "beets writes a tagged rip into its output directory and calls " +
			"this from its hook plugin. Lidarr then links the files into the " +
			"artist folder and names them, so a rip and a torrent land " +
			"identically and Lidarr stays the only writer into the library. " +
			"It asks Lidarr what it identified and imports exactly that, " +
			"rather than asking for a folder scan: an unattended scan scores " +
			"the files against the releases Lidarr holds and rejects a " +
			"correct rip whose release its metadata mirror does not carry. " +
			"Reads " + importer.EnvURL + ", " + importer.EnvAPIKey + " and " +
			"optionally " + importer.EnvMode + ", " + importer.EnvTimeout +
			" and " + importer.EnvPoll + " from the environment.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("album", cfg.Album, "album directory beets has written")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			return importer.Run(ctx, env, *cfg)
		},
	}
}

func main() {
	cfg := importer.Defaults()
	newApp(&cfg).Main()
}
