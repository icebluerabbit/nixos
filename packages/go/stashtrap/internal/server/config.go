package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"homelab/app"
	"homelab/clients/stashbox"
	"homelab/secret"
)

// Config is what the tool takes on the command line.
type Config struct {
	// Addr is the listen address for the announce endpoint. It binds all
	// interfaces by default: the container perimeter is the firewall, and
	// autobrr reaches this across the bridge.
	Addr string `json:"addr"`
	// DBPath is the SQLite file. Local disk, never NFS.
	DBPath string `json:"db_path"`
	// ShutdownTimeout bounds the wait for in-flight requests at shutdown.
	// One announce is a parse and an insert, so a second is generous; the
	// unit's own TimeoutStopSec is the real backstop.
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`

	// StashboxEndpoint is the stash-box GraphQL URL whose favourites make up
	// the catalogue. Empty disables the catalogue and POST /sync.
	StashboxEndpoint string `json:"stashbox_endpoint"`
	// StashboxKey is the API key, given literally.
	StashboxKey string `json:"stashbox_key"`
	// StashboxKeyFile is a file holding the API key — a sops secret path, or
	// a systemd credential.
	StashboxKeyFile string `json:"stashbox_key_file"`
	// StashboxMinInterval is the floor between stash-box requests, so a bulk
	// crawl paces itself instead of discovering the service's limit.
	StashboxMinInterval time.Duration `json:"stashbox_min_interval"`

	// Platforms are taken wholesale, on top of the followed performers and
	// studios.
	//
	// A platform is not a studio: "OnlyFans" is an alias of seventy-five
	// studios and identifies none of them, and the performer is a handle
	// nobody follows, so these releases can never match on merit. Whether
	// they are worth the disk is a judgement about disk, which is why it is
	// a flag and not a rule.
	Platforms []string `json:"platforms"`
}

// Defaults are what a unit that passes no flags gets.
func Defaults() Config {
	return Config{
		Addr:                ":8080",
		DBPath:              "/var/lib/stashtrap/stashtrap.db",
		ShutdownTimeout:     5 * time.Second,
		StashboxEndpoint:    "https://stashdb.org/graphql",
		StashboxMinInterval: time.Second,
	}
}

// Stashbox resolves the stash-box client configuration, or reports that none
// was given — a deployment with no key runs without a catalogue rather than
// refusing to start.
func (c Config) Stashbox() (stashbox.Config, bool, error) {
	if c.StashboxEndpoint == "" || (c.StashboxKey == "" && c.StashboxKeyFile == "") {
		return stashbox.Config{}, false, nil
	}
	key, err := secret.Flag("stashbox-key", c.StashboxKey, c.StashboxKeyFile)
	if err != nil {
		return stashbox.Config{}, false, err
	}
	return stashbox.Config{
		Endpoint:    c.StashboxEndpoint,
		APIKey:      key,
		MinInterval: c.StashboxMinInterval,
	}, true, nil
}

// Validate reports a configuration that cannot work.
func (c Config) Validate() error {
	switch {
	case c.Addr == "":
		return errors.New("stashtrap: --addr is required")
	case c.DBPath == "":
		return errors.New("stashtrap: --db is required")
	}
	return nil
}

// Serve runs the HTTP server until ctx is cancelled.
//
// A daemon under lib/app blocks here; the graceful shutdown matters because an
// answer autobrr never reads is a release it treats as failed.
func Serve(ctx context.Context, env *app.Env, srv *Server, cfg Config) error {
	httpSrv := &http.Server{
		Addr:    cfg.Addr,
		Handler: srv.Handler(),
		// Generous: the body can carry a base64 .torrent, and autobrr is
		// on the same bridge rather than the open internet.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		env.Logger.Info("listening", "addr", cfg.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- fmt.Errorf("stashtrap: serve: %w", err)
			return
		}
		errc <- nil
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("stashtrap: shutdown: %w", err)
		}
		return nil
	}
}
