// Command soularr-reclaim hands stranded Soulseek album folders back to
// Lidarr, one per command.
package main

import (
	"context"

	"homelab/app"
	"homelab/soularr-reclaim/internal/reclaim"
)

// newApp wires the command line to the work. There are no flags: the unit
// configures this entirely through the environment, and internal/reclaim
// reads those names directly.
func newApp() app.App {
	return app.App{
		Name:  "soularr-reclaim",
		Short: "Hand stranded Soulseek album folders back to Lidarr",
		Long: "A soularr run only imports what it grabbed in that same run, " +
			"so an album stranded by a crash or an NFS stall sits in the " +
			"download directory forever. This walks that directory and " +
			"fires DownloadedAlbumsScan once per album folder. Nothing " +
			"here moves or deletes anything; Lidarr does the moving. " +
			"Configured by " + reclaim.EnvLidarrURL + ", " +
			reclaim.EnvLidarrKeyFile + ", " + reclaim.EnvDownloadDir +
			" and the RECLAIM_* settings.",
		Run: func(ctx context.Context, env *app.Env) error {
			cfg, err := reclaim.Load(env.Logger)
			if err != nil {
				return err
			}
			return reclaim.Run(ctx, env, cfg, reclaim.Options{})
		},
	}
}

func main() {
	newApp().Main()
}
