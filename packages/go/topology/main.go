// Command topology renders network diagrams from the infra registries.
package main

import (
	"context"
	"os"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/topology/internal/render"
)

// newApp wires the command line to the work. Split out of main only so that
// main_test can drive it: the flag names and their mapping onto Config's keys
// are the one thing here that can be wrong, and a typo in either half would
// otherwise be silent until `just topology` wrote the wrong section.
func newApp(cfg *render.Config) app.App {
	return app.App{
		Name:  "topology",
		Short: "Render network diagrams from the infra registries",
		Long: "Reads `config.infra` as JSON on stdin:\n\n" +
			"  nix eval .#nixosConfigurations.server.config.infra --json | topology\n\n" +
			"Views:\n" +
			"  physical   what is plugged into what, derived from the port-profile join\n" +
			"  logical    the server's own VLANs, bridges and containers past the trunk",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("view", cfg.View, "which diagram to render")
			f.String("host", cfg.Host, "host the logical view describes")
			f.String("into", cfg.Into, "markdown file to splice into; stdout if unset")
			f.String("marker", cfg.Marker, "name of the BEGIN/END marker pair in --into")
		},
		Run: func(_ context.Context, env *app.Env) error {
			return render.Run(env, *cfg, os.Stdin)
		},
	}
}

func main() {
	cfg := render.Defaults()
	newApp(&cfg).Main()
}
