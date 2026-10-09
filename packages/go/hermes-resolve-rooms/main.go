// Command hermes-resolve-rooms resolves declared Matrix room aliases to room
// IDs and appends them to the Hermes .env.
//
// It takes positional arguments, a variable number of them, which is why it
// does not run through lib/app: that bootstrap rejects positional arguments,
// and the container unit passes the aliases positionally.
//
//	hermes-resolve-rooms <homeserver> <env-file> <home-alias|-> [alias...]
//
// main is thin by construction; the work is in internal/resolve.
package main

import (
	"context"
	"fmt"
	"os"

	"homelab/hermes-resolve-rooms/internal/resolve"
)

// usage is what a caller that got the arguments wrong is told.
const usage = "usage: hermes-resolve-rooms <homeserver> <env-file> <home-alias|-> [alias...]"

// exitUsage matches the convention across this workspace: 2 says the command
// line was wrong, as opposed to 1 for work that ran and failed.
const exitUsage = 2

// minArgs is the homeserver, the env file and the home alias placeholder.
const minArgs = 3

func main() {
	args := os.Args[1:]
	if len(args) < minArgs {
		fmt.Fprintln(os.Stderr, usage) //nolint:errcheck // reporting to a failed stderr has nowhere to go
		os.Exit(exitUsage)
	}

	cfg := resolve.Config{
		Homeserver:  args[0],
		EnvFile:     args[1],
		HomeAlias:   args[2],
		FreeAliases: args[minArgs:],
	}
	if err := resolve.Run(context.Background(), os.Stdout, os.Stderr, cfg, resolve.Options{}); err != nil {
		fmt.Fprintf(os.Stderr, "hermes-resolve-rooms: %v\n", err) //nolint:errcheck // as above
		os.Exit(1)
	}
}
