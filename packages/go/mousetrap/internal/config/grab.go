package config

import (
	flag "github.com/spf13/pflag"
)

// defaultGrabUnsatMargin holds this many slots below the unsatisfied limit.
const defaultGrabUnsatMargin = 5

// registerGrab declares the Layer-2 grab-gate flags — pure policy thresholds.
// Delivery (client URL, categories, paused) lives in registerClient, not here.
// dry-run defaults on: nothing is grabbed until explicitly armed.
func registerGrab(fs *flag.FlagSet) {
	fs.Int("grab-unsat-margin", defaultGrabUnsatMargin, "hold this many below the unsatisfied limit")
	fs.Duration("grab-inflight-window", defaultGrabInFlightWindow, "how long a grab counts as in-flight")
	fs.Int64("grab-buffer-min", 0, "non-freeleech grabs require buffer >= this many bytes; 0 = off")
	fs.Bool("grab-dry-run", true, "decide and log only; add nothing (arm by setting false)")
}
