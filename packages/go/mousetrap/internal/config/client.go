package config

import (
	flag "github.com/spf13/pflag"
)

// registerClient declares the download-client flags (qBittorrent today). Kept
// client-agnostic in name so a second client slots in without touching grab policy.
func registerClient(fs *flag.FlagSet) {
	fs.String("client-url", "", "download client WebUI base URL")
	fs.StringToString("client-category", nil, "mediaType=category (repeatable), e.g. Ebook=books")
	fs.Bool("client-paused", false, "add torrents paused")
}
