package config

import (
	flag "github.com/spf13/pflag"
)

// registerWedge declares the Personal Freeleech wedge flags. Disabled by default.
func registerWedge(fs *flag.FlagSet) {
	fs.Bool("wedge-enable", false, "enable Personal Freeleech wedge spending")
	fs.Int("wedge-reserve", 0, "keep at least this many wedges unspent")
	fs.StringArray("wedge-proactive-media-type", nil, "always wedge this MAM media type above the proactive size (repeatable)")
	fs.Int64("wedge-proactive-min-size", 0, "proactive wedge size threshold in bytes")
	fs.Bool("wedge-require", false, "skip a proactive-wedge release when no wedge is spendable instead of grabbing it at full ratio")
	fs.Bool("wedge-rescue", false, "wedge a grab that would otherwise be a buffer hold")
	fs.Int64("wedge-rescue-min-size", 0, "rescue wedge size threshold in bytes")
}
