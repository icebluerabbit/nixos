package config

import (
	flag "github.com/spf13/pflag"

	"mousetrap/types"
)

// registerClient binds the download-client flags (qBittorrent today). Kept
// client-agnostic in name so a second client slots in without touching grab policy.
func registerClient(fs *flag.FlagSet, c *types.ClientConfig) {
	fs.StringVar(&c.URL, "client-url", "", "download client WebUI base URL")
	fs.StringToStringVar(&c.Categories, "client-category", nil, "kind=category (repeatable), e.g. ebook=books")
	fs.BoolVar(&c.AddPaused, "client-paused", false, "add torrents paused")
}
