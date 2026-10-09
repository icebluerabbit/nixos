// Command jellyfin-refresh refreshes one Jellyfin library path from an *arr
// CustomScript hook.
package main

import (
	"context"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/jellyfin-refresh/internal/refresh"
)

// newApp wires the command line to the work. Split out of main only so that
// main_test can drive it: the flag names and their mapping onto Config's keys
// are the one thing here that can be wrong, and a typo in either half would
// otherwise be silent until an import failed to show up.
func newApp(cfg *refresh.Config) app.App {
	return app.App{
		Name:  "jellyfin-refresh",
		Short: "Refresh one Jellyfin library path from an *arr hook",
		Long: "Radarr, Sonarr and Lidarr authenticate their built-in " +
			"Emby/Jellyfin connection with the X-Emby-Token header and the " +
			"api_key query parameter, both of which Jellyfin 12 removed: the " +
			"library update returns 401 and an imported file stays invisible " +
			"until the next scheduled scan. This posts the same update over " +
			"the Authorization: MediaBrowser scheme Jellyfin 12 accepts, " +
			"naming the path so only that directory is refreshed. The *arr " +
			"hands its event to a hook through the environment and each one " +
			"names those variables differently, so the names are configuration " +
			"— given as flags by hand, or, because a modern Radarr custom " +
			"script may take no arguments, as HOMELAB_JELLYFIN_REFRESH_* in " +
			"the environment the hook inherits.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("base", cfg.Base, "Jellyfin base URL")
			f.String("api-key", cfg.APIKey, "Jellyfin API key")
			f.String("api-key-file", cfg.APIKeyFile,
				"file holding the Jellyfin API key")
			f.String("event-env", cfg.EventEnv,
				"environment variable holding the *arr event type")
			f.String("path-env", cfg.PathEnv,
				"comma-separated environment variables that may hold the "+
					"changed media path, in order of preference")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			return refresh.Run(ctx, env, *cfg)
		},
	}
}

func main() {
	cfg := refresh.Defaults()
	newApp(&cfg).Main()
}
