// Command stashtrap receives autobrr announces, records them, and answers
// whether the release should be grabbed.
//
// It is an external filter, not an acquirer: autobrr asks, stashtrap decides,
// autobrr grabs. Phase one records everything and accepts nothing — the corpus
// is what the matching in phase two gets designed against.
package main

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/clients/stashbox"
	"homelab/stashtrap/internal/policy"
	"homelab/stashtrap/internal/server"
	"homelab/stashtrap/internal/store"
	"homelab/stashtrap/internal/sync"
)

// newApp wires the command line to the work. Split out of main only so
// main_test can drive it: the flag names and their mapping onto Config's keys
// are the one thing here that can be wrong, and a typo in either half would
// otherwise be silent until the unit ran.
func newApp(cfg *server.Config) app.App {
	return app.App{
		Name:  "stashtrap",
		Short: "Record autobrr announces and decide whether to grab them",
		Long: "Runs as autobrr's external filter: it answers 250 to accept a " +
			"release and 200 to reject it, and records every announce it " +
			"was asked about. A release is accepted when it names a followed " +
			"performer or studio, or comes from a platform given to --platforms.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("addr", cfg.Addr, "listen address for the announce endpoint")
			f.String("db-path", cfg.DBPath, "SQLite database path (local disk, never NFS)")
			f.Duration("shutdown-timeout", cfg.ShutdownTimeout, "grace period for in-flight requests")
			f.String("stashbox-endpoint", cfg.StashboxEndpoint, "stash-box GraphQL endpoint whose favourites form the catalogue")
			f.String("stashbox-key", "", "stash-box API key (prefer --stashbox-key-file)")
			f.String("stashbox-key-file", "", "file holding the stash-box API key")
			f.Duration("stashbox-min-interval", cfg.StashboxMinInterval, "floor between stash-box requests")
			f.StringSlice("platforms", cfg.Platforms, "platforms to accept wholesale, which the catalogue cannot recognise (e.g. onlyfans)")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			return run(ctx, env, *cfg)
		},
	}
}

func run(ctx context.Context, env *app.Env, cfg server.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer st.Close() //nolint:errcheck // the process is exiting; nothing can act on it

	// A deployment without a stash-box key runs without a catalogue: the
	// announce path does not depend on it, and refusing to start would take
	// the filter's gate down over a credential it never needed.
	var syncer *sync.Syncer
	if sbCfg, ok, err := cfg.Stashbox(); err != nil {
		return err
	} else if ok {
		client, err := stashbox.New(sbCfg)
		if err != nil {
			return fmt.Errorf("stashtrap: %w", err)
		}
		defer client.Close()

		if syncer, err = sync.New(sync.Deps{
			Client:   client,
			Store:    st,
			Logger:   env.Logger,
			Endpoint: sbCfg.Endpoint,
			Registry: env.Metrics,
		}); err != nil {
			return fmt.Errorf("stashtrap: %w", err)
		}
	} else {
		env.Logger.Warn("no stash-box key given; catalogue sync disabled")
	}

	srv, err := server.New(server.Deps{
		Store:    st,
		Decider:  policy.New(cfg.Platforms...),
		Logger:   env.Logger,
		Syncer:   syncer,
		Endpoint: cfg.StashboxEndpoint,
	}, env.Metrics)
	if err != nil {
		return fmt.Errorf("stashtrap: %w", err)
	}

	return server.Serve(ctx, env, srv, cfg)
}

func main() {
	cfg := server.Defaults()
	newApp(&cfg).Main()
}
