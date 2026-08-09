// Package config populates the runtime configuration from command-line flags
// (pflag) into a types.Config. It is the only place besides main that touches
// pflag; the config shape itself lives in package types. Flag registration is
// split by domain: mam.go, client.go, webhook.go, accept.go, grab.go, wedge.go.
package config

import (
	"os"

	flag "github.com/spf13/pflag"

	"mousetrap/types"
)

// Load defines the flags, parses os.Args, and returns the resolved configuration.
func Load() types.Config {
	return Parse(os.Args[1:])
}

// Parse registers every flag into a fresh FlagSet, parses args, and returns the
// resolved configuration. Separate from Load so tests can pass explicit args.
func Parse(args []string) types.Config {
	fs := flag.NewFlagSet("mousetrap", flag.ExitOnError)

	var c types.Config
	fs.StringVar(&c.Listen, "listen", "0.0.0.0", "HTTP bind address")
	fs.StringVar(&c.Port, "port", "8080", "HTTP bind port")
	fs.StringVar(&c.StateDir, "state-dir", "/var/lib/mousetrap", "local-disk state directory")

	registerMAM(fs, &c.MAM)
	registerClient(fs, &c.Client)
	registerWebhook(fs, &c.Webhook)
	registerTorznab(fs, &c.Torznab)
	registerAccept(fs, &c.Accept)
	registerGrab(fs, &c.Grab)
	registerWedge(fs, &c.Wedge)
	registerShop(fs, &c.Shop)

	_ = fs.Parse(args) // ExitOnError: --help and parse errors exit the process
	return c
}
