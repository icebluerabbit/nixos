package config

import (
	flag "github.com/spf13/pflag"
)

// registerTorznab declares the Torznab indexer flags. The download links search
// mints route back to mousetrap so grabs keep running the policy; base-url is the
// absolute host those links point at.
func registerTorznab(fs *flag.FlagSet) {
	fs.String("torznab-apikey", "", "require this apikey on torznab requests; empty = no auth")
	fs.String("torznab-base-url", "", "absolute base URL for minted download links, e.g. http://mousetrap:8080")
	fs.Duration("torznab-link-ttl", defaultTorznabLinkTTL, "reject download links older than this; 0 = no expiry")
}
