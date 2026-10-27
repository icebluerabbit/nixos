// Command tuwunel-init declaratively provisions tuwunel's users, rooms and
// spaces after the homeserver comes up.
//
// It takes five positional arguments, which is why it does not run through
// lib/app: that bootstrap rejects positional arguments, and the container
// unit passes these positionally — the two JSON documents included.
//
//	tuwunel-init <port> <server-name> <admin-user> <users-json> <rooms-json>
//
// The registration secret and the per-user passwords come from the
// environment instead, so they never appear in the homeserver's process
// list.
//
// main is thin by construction; the work is in internal/provision.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"homelab/tuwunel-init/internal/provision"
)

// usage is what a caller that got the arguments wrong is told.
const usage = "usage: tuwunel-init <port> <server-name> <admin-user> <users-json> <rooms-json>"

// exitUsage matches the convention across this workspace: 2 says the command
// line was wrong, as opposed to 1 for work that ran and failed.
const exitUsage = 2

// wantArgs is the full positional set.
const wantArgs = 5

func main() {
	args := os.Args[1:]
	if len(args) != wantArgs {
		fmt.Fprintln(os.Stderr, usage) //nolint:errcheck // reporting to a failed stderr has nowhere to go
		os.Exit(exitUsage)
	}

	cfg := provision.Config{
		Port:       args[0],
		ServerName: args[1],
		AdminUser:  args[2],
	}
	if err := json.Unmarshal([]byte(args[3]), &cfg.Users); err != nil {
		fail("parse users: %v", err)
	}
	if err := json.Unmarshal([]byte(args[4]), &cfg.Rooms); err != nil {
		fail("parse rooms: %v", err)
	}

	secrets := provision.EnvSecrets()
	if secrets.RegSecret == "" {
		fail("TUWUNEL_REG_SECRET is not set")
	}

	if err := provision.Run(context.Background(), os.Stderr, cfg, secrets, provision.Options{}); err != nil {
		fail("%v", err)
	}
}

// fail reports and exits 1: the work ran and could not be done.
func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "tuwunel-init: "+format+"\n", args...) //nolint:errcheck // as above
	os.Exit(1)
}
