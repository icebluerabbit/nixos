package config

import (
	flag "github.com/spf13/pflag"

	"mousetrap/types"
)

// registerWedge binds the Personal Freeleech wedge flags. Disabled by default.
func registerWedge(fs *flag.FlagSet, c *types.WedgeConfig) {
	fs.BoolVar(&c.Enable, "wedge-enable", false, "enable Personal Freeleech wedge spending")
	fs.IntVar(&c.Reserve, "wedge-reserve", 0, "keep at least this many wedges unspent")
	fs.StringArrayVar(&c.ProactiveMediaTypes, "wedge-proactive-media-type", nil, "always wedge this MAM media type above the proactive size (repeatable)")
	fs.Int64Var(&c.ProactiveMinSizeBytes, "wedge-proactive-min-size", 0, "proactive wedge size threshold in bytes")
	fs.BoolVar(&c.RescueEnable, "wedge-rescue", false, "wedge a grab that would otherwise be a buffer hold")
	fs.Int64Var(&c.RescueMinSizeBytes, "wedge-rescue-min-size", 0, "rescue wedge size threshold in bytes")
}
