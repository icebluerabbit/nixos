// Command qui-provision registers the qBittorrent instance in qui.
//
// It takes three positional arguments rather than flags, which is why it
// does not run through lib/app: that bootstrap rejects positional arguments,
// and the container unit passes these three positionally.
//
//	qui-provision <api-base> <qbt-url> <name>
//
// main is thin by construction; the work is in internal/provision.
package main

import (
	"context"
	"fmt"
	"os"

	"homelab/qui-provision/internal/provision"
)

// usage is what a caller that got the arguments wrong is told.
const usage = "usage: qui-provision <api-base> <qbt-url> <name>"

// exitUsage matches the convention across this workspace: 2 says the command
// line was wrong, as opposed to 1 for work that ran and failed.
const exitUsage = 2

func main() {
	args := os.Args[1:]
	if len(args) != 3 {
		fmt.Fprintln(os.Stderr, usage) //nolint:errcheck // reporting to a failed stderr has nowhere to go
		os.Exit(exitUsage)
	}

	cfg := provision.Config{APIBase: args[0], QbtURL: args[1], Name: args[2]}
	if err := provision.Run(context.Background(), os.Stdout, cfg, provision.Options{}); err != nil {
		fmt.Fprintf(os.Stderr, "qui-provision: %v\n", err) //nolint:errcheck // as above
		os.Exit(1)
	}
}
