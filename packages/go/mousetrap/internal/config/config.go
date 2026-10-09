// Package config declares mousetrap's command line and the defaults behind it.
// It is the only place besides main that touches pflag; the config shape itself
// lives in package types. Flag registration is split by domain: mam.go,
// client.go, webhook.go, torznab.go, accept.go, grab.go, wedge.go, shop.go.
//
// Flags are declared, not bound. lib/config resolves them onto types.Config by
// key — a flag's name with dashes turned into underscores — after layering the
// environment on top of the defaults, so a value can arrive from either. That
// is also why every flag's default must equal the matching field in Defaults:
// the two are the same fact stated in the two places the resolver reads.
package config

import (
	"time"

	flag "github.com/spf13/pflag"

	libconfig "homelab/config"
	"homelab/mousetrap/internal/types"
)

// Process defaults. The bind address is every interface inside the container's
// own netns, and the state directory is the bind mount the unit provides.
const (
	// DefaultListen binds every interface: the container has one.
	DefaultListen = "0.0.0.0"
	// DefaultPort is the HTTP port the unit publishes.
	DefaultPort = "8080"
	// DefaultStateDir holds the SQLite ledgers and the rotated mam_id.
	DefaultStateDir = "/var/lib/mousetrap"
)

// Defaults returns the configuration a run with no flags and no environment
// gets. Every flag declaration repeats the value that lands here.
func Defaults() types.Config {
	return types.Config{
		Listen:   DefaultListen,
		Port:     DefaultPort,
		StateDir: DefaultStateDir,
		MAM:      types.MAMConfig{APIBase: defaultMAMAPIBase},
		Torznab:  types.TorznabConfig{LinkTTL: defaultTorznabLinkTTL},
		Grab: types.GrabConfig{
			UnsatMargin:    defaultGrabUnsatMargin,
			InFlightWindow: defaultGrabInFlightWindow,
			DryRun:         true,
		},
		Shop: types.ShopConfig{
			DryRun:             true,
			VIPCost:            defaultShopVIPCost,
			VIPReserveMultiple: defaultShopVIPReserveMultiple,
			UploadChunkGiB:     defaultShopUploadChunkGiB,
			UploadPointsPerGiB: defaultShopUploadPointsPerGiB,
		},
	}
}

// Flags declares every mousetrap flag on fs. It is what main hands lib/app.
func Flags(fs *flag.FlagSet) {
	fs.String("listen", DefaultListen, "HTTP bind address")
	fs.String("port", DefaultPort, "HTTP bind port")
	fs.String("state-dir", DefaultStateDir, "local-disk state directory")

	registerMAM(fs)
	registerClient(fs)
	registerWebhook(fs)
	registerTorznab(fs)
	registerAccept(fs)
	registerGrab(fs)
	registerWedge(fs)
	registerShop(fs)
}

// Parse registers every flag into a fresh FlagSet, parses args, and resolves
// the configuration. It is the flag layer alone — no environment, no file — so
// tests drive the command line exactly as the container passes it.
func Parse(args []string) (types.Config, error) {
	fs := flag.NewFlagSet("mousetrap", flag.ContinueOnError)
	Flags(fs)
	if err := fs.Parse(args); err != nil {
		return types.Config{}, err
	}

	cfg := Defaults()
	if err := libconfig.Load(&cfg, libconfig.Options{Flags: fs}); err != nil {
		return types.Config{}, err
	}
	return cfg, nil
}

// Durations are named here rather than inline so Defaults and the flag
// declaration cannot drift apart.
const (
	defaultGrabInFlightWindow = 30 * time.Minute
	defaultTorznabLinkTTL     = time.Hour
)
