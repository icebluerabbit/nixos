// Command managarr-crossseed serves cross-seed's decision log to managarr.
//
// cross-seed records every candidate it considered and what it decided —
// MATCH, SIZE_MISMATCH, FILE_TREE_MISMATCH, INFO_HASH_ALREADY_EXISTS — in its
// own SQLite database, and exposes none of it. Its HTTP API is /api/ping,
// /api/announce and /api/webhook, and the webhook fires only on a match. So the
// rejections, which are four fifths of the record and the only answer to "why
// was this not cross-seeded", are reachable only from the file.
//
// This runs beside that file, inside cross-seed's own container, and serves it
// over the managarr sidecar contract. The alternative was bind-mounting another
// service's database into a consumer, which needs a matching uid and makes the
// consumer depend on a schema upstream is free to change.
//
// Nothing here writes to cross-seed. There is no endpoint that could.
package main

import (
	"context"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/managarr-crossseed/internal/service"
	"homelab/sidecar"
)

// config is this sidecar's command line: the shared contract flags plus the one
// thing only this sidecar has.
type config struct {
	// Squashed: without this the embedded struct is a field literally named
	// "Config", which collides with the framework's own --config flag (a file
	// path, a string) and fails every start with "expected a map or struct,
	// got string".
	sidecar.Config `json:",squash"`
	// DBPath is cross-seed's live SQLite database. Read, never written.
	DBPath string `json:"db_path"`
}

func defaults() config {
	return config{
		Config: sidecar.Defaults(),
		DBPath: "/var/lib/cross-seed/cross-seed.db",
	}
}

// newApp wires the command line to the work. Split out of main only so
// main_test can drive it: the flag names and their mapping onto config's keys
// are the one thing here that can be wrong, and a typo in either half would be
// silent until the unit ran.
func newApp(cfg *config) app.App {
	return app.App{
		Name:  "managarr-crossseed",
		Short: "Serve cross-seed's decision log to managarr",
		Long: "cross-seed keeps every candidate it considered and its verdict " +
			"in SQLite and exposes none of it: its API has no decisions " +
			"route, and its webhook fires only on a match. This serves that " +
			"database read-only, from a fresh snapshot per request, so " +
			"managarr needs no bind mount and no knowledge of the schema.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("addr", cfg.Addr, "listen address")
			f.String("db-path", cfg.DBPath, "cross-seed's SQLite database; read, never written")
			f.String("managarr-url", cfg.ManagarrURL, "managarr base URL; empty disables relaying")
			f.Duration("shutdown-timeout", cfg.ShutdownTimeout, "grace period for in-flight requests")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			return sidecar.Run(ctx, env, cfg.Config, service.New(cfg.DBPath))
		},
	}
}

func main() {
	cfg := defaults()
	newApp(&cfg).Main()
}
