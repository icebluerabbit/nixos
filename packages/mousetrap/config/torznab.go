package config

import (
	"time"

	flag "github.com/spf13/pflag"

	"mousetrap/types"
)

// registerTorznab binds the Torznab indexer flags. The download links search mints
// route back to mousetrap so grabs keep running the policy; base-url is the absolute
// host those links point at.
func registerTorznab(fs *flag.FlagSet, c *types.TorznabConfig) {
	fs.StringVar(&c.APIKey, "torznab-apikey", "", "require this apikey on torznab requests; empty = no auth")
	fs.StringVar(&c.BaseURL, "torznab-base-url", "", "absolute base URL for minted download links, e.g. http://mousetrap:8080")
	fs.DurationVar(&c.LinkTTL, "torznab-link-ttl", time.Hour, "reject download links older than this; 0 = no expiry")
}
