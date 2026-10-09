// Command qbt-watch imports .torrent files from watch/<category>/ into
// qBittorrent, filing each under the category its subdirectory names.
package main

import (
	"context"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/qbt-scripts/internal/watch"
)

// newApp wires the command line to the work. Split out of main only so that
// main_test can drive it: a flag name that does not match its config key
// parses fine and leaves the field empty, which here means scanning the
// wrong directory or none.
func newApp(cfg *watch.Config) app.App {
	return app.App{
		Name:  "qbt-watch",
		Short: "Import .torrent files from watch/<category>/ into qBittorrent",
		Long: "qBittorrent's own watch directory files everything into one " +
			"category. Here the subdirectory names the category, so a " +
			"hand-dropped torrent lands where an *arr grab would. An " +
			"imported file is moved to the backup directory, which is " +
			"what keeps the pass idempotent. --report-url, when given, " +
			"records each import as provenance: nothing else distinguishes " +
			"a hand drop from a cross-seed inject once the torrent is in " +
			"the client.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("api", cfg.API, "qBittorrent WebUI base URL")
			f.String("watch-dir", cfg.WatchDir,
				"directory holding one subdirectory per category")
			f.String("backup-dir", cfg.BackupDir,
				"directory an imported .torrent is moved to")
			f.String("categories", cfg.Categories,
				"comma-separated categories to scan")
			f.String("report-url", cfg.ReportURL,
				"POST each import here; empty disables reporting")
		},
		Run: func(ctx context.Context, env *app.Env) error {
			return watch.Run(ctx, env, *cfg)
		},
	}
}

func main() {
	cfg := watch.Defaults()
	newApp(&cfg).Main()
}
