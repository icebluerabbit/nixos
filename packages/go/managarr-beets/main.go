// Command managarr-beets serves beets' library to managarr.
//
// beets is the only thing in this stack that has read the files themselves.
// Lidarr knows what it asked for and what it filed; beets knows what the bytes
// actually say — which pressing, which label, which country, what the bitrate
// really is rather than what the release name claimed.
//
// It is also where the music loop closes. mb_releasegroupid is Lidarr's
// foreignAlbumId under a different name, so a tagged file resolves to the album
// that was searched for by shared identifier rather than by comparing titles.
//
// The work is lib/sidecar/beetssvc's.
package main

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/clients/beets"
	"homelab/httpx"
	"homelab/sidecar"
	"homelab/sidecar/beetssvc"
)

type config struct {
	// Squashed: without this the embedded struct is a field literally named
	// "Config", which collides with the framework's own --config flag (a file
	// path, a string) and fails every start with "expected a map or struct,
	// got string".
	sidecar.Config `json:",squash"`
	// UpstreamURL is the beets web plugin, on the loopback inside this
	// container.
	UpstreamURL string `json:"upstream_url"`
}

func defaults() config {
	return config{
		Config:      sidecar.Defaults(),
		UpstreamURL: "http://127.0.0.1:8337",
	}
}

// newApp wires the command line to the work. Split out of main only so
// main_test can drive it: the flag names and their mapping onto config's keys
// are the one thing here that can be wrong, and a typo in either half would be
// silent until the unit ran.
func newApp(cfg *config) app.App {
	return app.App{
		Name:  "managarr-beets",
		Short: "Serve beets' library to managarr",
		Long: "beets holds the tag layer — the MusicBrainz ids resolved per " +
			"track and the audio properties it probed. Its mb_releasegroupid " +
			"is what Lidarr calls foreignAlbumId, which is what joins a " +
			"tagged file to the album that was searched for.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("addr", cfg.Addr, "listen address")
			f.String("upstream-url", cfg.UpstreamURL, "beets web plugin base URL, on the loopback")
			f.String("managarr-url", cfg.ManagarrURL, "managarr base URL; empty disables relaying")
			f.Duration("shutdown-timeout", cfg.ShutdownTimeout, "grace period for in-flight requests")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			// The web plugin has no authentication of its own; it is
			// reachable only on the loopback inside this container.
			c, err := beets.New(httpx.Config{BaseURL: cfg.UpstreamURL, Logger: env.Logger})
			if err != nil {
				return fmt.Errorf("beets client: %w", err)
			}
			return sidecar.Run(ctx, env, cfg.Config, beetssvc.New(c, cfg.UpstreamURL))
		},
	}
}

func main() {
	cfg := defaults()
	newApp(&cfg).Main()
}
