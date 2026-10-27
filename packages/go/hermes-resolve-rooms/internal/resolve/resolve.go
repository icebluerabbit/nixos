// Package resolve turns declared Matrix room aliases into room IDs.
//
// Matrix mints room IDs server-side at createRoom, so there is nothing to
// declare in Nix — only the alias is knowable at eval time. Hermes takes room
// IDs, never aliases, so they are resolved here and appended to the .env the
// activation script just wrote.
//
// This runs on every container start, after the .env has been rebuilt from
// scratch, so a recreated room heals itself and nothing accumulates.
//
// An unresolved alias is not fatal: an absent MATRIX_HOME_ROOM only means
// unattended output has nowhere to go, and an absent
// MATRIX_FREE_RESPONSE_ROOMS only means an @mention stays required.
package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Retry budget. tuwunel creates the rooms at its own startup, so the first
// few lookups routinely 404 while it works through them.
const (
	// DefaultRetries is how many times an alias is looked up.
	DefaultRetries = 60
	// DefaultRetryDelay is how long to wait between lookups.
	DefaultRetryDelay = 2 * time.Second
)

// Environment keys written into the .env.
const (
	// HomeRoomKey is where unattended output goes.
	HomeRoomKey = "MATRIX_HOME_ROOM"
	// FreeResponseKey lists the rooms where no @mention is required.
	FreeResponseKey = "MATRIX_FREE_RESPONSE_ROOMS"
)

// NoHomeRoom is the placeholder the unit passes when no home room is
// declared, because the argument is positional and cannot simply be omitted.
const NoHomeRoom = "-"

// directoryPath is the tokenless alias lookup — the same one tuwunel's own
// init makes.
const directoryPath = "/_matrix/client/v3/directory/room/"

// Config is what the tool takes on the command line.
type Config struct {
	// Homeserver is the Matrix server to ask, e.g. http://127.0.0.1:6167.
	Homeserver string
	// EnvFile is the .env to append to.
	EnvFile string
	// HomeAlias is the home room's alias, or NoHomeRoom.
	HomeAlias string
	// FreeAliases are the rooms where an @mention is not required.
	FreeAliases []string
}

// Options tune the retry loop. They exist so tests do not wait two minutes
// for the unresolved path; nothing sets them from the command line.
type Options struct {
	Retries    int
	RetryDelay time.Duration
	Client     *http.Client
}

// withDefaults fills in whatever the caller left unset.
func (o Options) withDefaults() Options {
	if o.Retries <= 0 {
		o.Retries = DefaultRetries
	}
	if o.RetryDelay <= 0 {
		o.RetryDelay = DefaultRetryDelay
	}
	if o.Client == nil {
		o.Client = http.DefaultClient
	}
	return o
}

// Run resolves the declared aliases and appends what it found to the .env.
//
// stdout carries the lines written, stderr the aliases that could not be
// resolved. Neither case is an error: the gateway runs either way, with less
// reach.
func Run(ctx context.Context, stdout, stderr io.Writer, cfg Config, opts Options) error {
	opts = opts.withDefaults()

	if cfg.HomeAlias != NoHomeRoom {
		roomID := resolveAlias(ctx, cfg.Homeserver, cfg.HomeAlias, opts)
		switch roomID {
		case "":
			fmt.Fprintf(stderr, //nolint:errcheck // a report that cannot be written has nowhere to go
				"could not resolve %s; starting without a home room "+
					"(cron and notification output will be dropped)\n", cfg.HomeAlias)
		default:
			if err := appendEnv(stdout, cfg.EnvFile, HomeRoomKey, roomID); err != nil {
				return err
			}
		}
	}

	if len(cfg.FreeAliases) == 0 {
		return nil
	}

	resolved := make([]string, 0, len(cfg.FreeAliases))
	for _, alias := range cfg.FreeAliases {
		if roomID := resolveAlias(ctx, cfg.Homeserver, alias, opts); roomID != "" {
			resolved = append(resolved, roomID)
		}
	}
	if len(resolved) == 0 {
		fmt.Fprintln(stderr, //nolint:errcheck // a report that cannot be written has nowhere to go
			"could not resolve any free-response room; "+
				"@mention stays required in every room")
		return nil
	}
	return appendEnv(stdout, cfg.EnvFile, FreeResponseKey, strings.Join(resolved, ","))
}

// resolveAlias returns the room ID for an alias, retrying while tuwunel
// creates it, and "" once the budget is spent.
func resolveAlias(ctx context.Context, homeserver, alias string, opts Options) string {
	url := strings.TrimRight(homeserver, "/") + directoryPath + escapeAlias(alias)

	for attempt := range opts.Retries {
		if roomID := lookup(ctx, opts.Client, url); roomID != "" {
			return roomID
		}
		if attempt == opts.Retries-1 {
			break
		}
		select {
		case <-ctx.Done():
			return ""
		case <-time.After(opts.RetryDelay):
		}
	}
	return ""
}

// lookup performs one directory request, returning "" for anything that is
// not a room ID. Every failure here is retried, so none of them is reported.
func lookup(ctx context.Context, client *http.Client, url string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return ""
	}

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	raw, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}

	var body struct {
		RoomID string `json:"room_id"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return body.RoomID
}

// appendEnv adds one KEY=value line to the .env and echoes it.
func appendEnv(stdout io.Writer, path, key, value string) error {
	line := key + "=" + value + "\n"

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // the path is the unit's own env file
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	if _, err := f.WriteString(line); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}

	fmt.Fprint(stdout, line) //nolint:errcheck // a report that cannot be written has nowhere to go
	return nil
}

// escapeAlias percent-encodes an alias for a path segment.
//
// Everything outside the unreserved set goes, which is what Python's
// quote(alias, safe="") did: a room alias is #room:server, and both the hash
// and the colon have to be encoded for the directory endpoint to see one
// segment.
func escapeAlias(alias string) string {
	var b strings.Builder
	for i := range len(alias) {
		c := alias[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
