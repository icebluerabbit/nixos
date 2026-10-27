// Command nixos-notify reports a host lifecycle event to MQTT, which Home
// Assistant consumes through an mqtt trigger.
//
// It takes positional arguments, a variable number of them, which is why it
// does not run through lib/app: that bootstrap rejects positional arguments,
// and the units pass these positionally.
//
//	nixos-notify <mosquitto_pub> <host> <port> <topic> <event> [arg] [arg2]
//
// main is thin by construction; the work is in internal/notify.
package main

import (
	"fmt"
	"os"

	"homelab/nixos-notify/internal/notify"
)

// usage is what a caller that got the arguments wrong is told.
const usage = "usage: nixos-notify <mosquitto_pub> <host> <port> <topic> <event> [arg] [arg2]"

// exitUsage matches the convention across this workspace: 2 says the command
// line was wrong, as opposed to 1 for work that ran and failed.
const exitUsage = 2

// minArgs is the publisher, the broker's host and port, the topic and the
// event.
const minArgs = 5

func main() {
	args := os.Args[1:]
	if len(args) < minArgs {
		fmt.Fprintln(os.Stderr, usage) //nolint:errcheck // reporting to a failed stderr has nowhere to go
		os.Exit(exitUsage)
	}

	cfg := notify.Config{
		Pub:   args[0],
		Host:  args[1],
		Port:  args[2],
		Topic: args[3],
		Event: args[4],
	}
	if len(args) > minArgs {
		cfg.Arg = args[minArgs]
	}
	if len(args) > minArgs+1 {
		cfg.Arg2 = args[minArgs+1]
	}

	if err := notify.Run(notify.OSSystem{}, os.Stderr, cfg, notify.Options{}); err != nil {
		fmt.Fprintf(os.Stderr, "nixos-notify: %v\n", err) //nolint:errcheck // as above
		os.Exit(1)
	}
}
