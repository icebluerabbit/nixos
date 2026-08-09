package config

import (
	"time"

	flag "github.com/spf13/pflag"

	"mousetrap/types"
)

// registerGrab binds the Layer-2 grab-gate flags — pure policy thresholds.
// Delivery (client URL, categories, paused) lives in registerClient, not here.
// dry-run defaults on: nothing is grabbed until explicitly armed.
func registerGrab(fs *flag.FlagSet, c *types.GrabConfig) {
	fs.IntVar(&c.UnsatMargin, "grab-unsat-margin", 5, "hold this many below the unsatisfied limit")
	fs.DurationVar(&c.InFlightWindow, "grab-inflight-window", 30*time.Minute, "how long a grab counts as in-flight")
	fs.Int64Var(&c.BufferMinBytes, "grab-buffer-min", 0, "non-freeleech grabs require buffer >= this many bytes; 0 = off")
	fs.BoolVar(&c.DryRun, "grab-dry-run", true, "decide and log only; add nothing (arm by setting false)")
}
