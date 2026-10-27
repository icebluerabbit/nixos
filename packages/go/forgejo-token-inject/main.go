// Command forgejo-token-inject splices the forgejo API token into
// code-server's settings.json at preStart.
//
// It takes two positional arguments rather than flags, which is why it does
// not run through lib/app: that bootstrap rejects positional arguments, and
// the container's preStart passes the two paths positionally.
//
//	forgejo-token-inject <settings.json> <token-file>
//
// main is thin by construction; the work is in internal/inject.
package main

import (
	"fmt"
	"os"

	"homelab/forgejo-token-inject/internal/inject"
)

// usage is what a caller that got the arguments wrong is told.
const usage = "usage: forgejo-token-inject <settings.json> <token-file>"

// exitUsage matches the convention across this workspace: 2 says the command
// line was wrong, as opposed to 1 for work that ran and failed.
const exitUsage = 2

func main() {
	args := os.Args[1:]
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, usage) //nolint:errcheck // reporting to a failed stderr has nowhere to go
		os.Exit(exitUsage)
	}

	if _, err := inject.Run(args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "forgejo-token-inject: %v\n", err) //nolint:errcheck // as above
		os.Exit(1)
	}
}
