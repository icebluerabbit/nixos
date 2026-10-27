// Command qbt-peer-exporter prints one JSON line of aggregated qBittorrent
// peer state for Vector's exec source to turn into gauges.
//
// Alone among the tools in this repo it does not run on homelab/app, for two
// reasons that are both about its caller.
//
// It takes positional arguments — the WebUI URL and the torrent cap — because
// Vector's exec source names them that way in the container's logging
// configuration, and app rejects positionals outright.
//
// More importantly it must never write to stderr. Vector's exec source turns
// each stderr line into an event alongside the stdout ones, and the transform
// behind it parses every event as JSON, so one log line would show up as a
// parse failure rather than as a log. That rules out a structured logger, and
// leaves the sample itself as the only channel a failure can be reported on:
// scrape_ok says whether the numbers mean anything.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"homelab/clients/qbittorrent"
	"homelab/httpx"
	"homelab/qbt-scripts/internal/peers"
)

// defaultBase is where qBittorrent's WebUI sits in its own container.
const defaultBase = "http://127.0.0.1:8080"

// exitUsage matches the convention the rest of these tools follow.
const exitUsage = 2

func main() {
	base, capN, err := parseArgs(os.Args[1:])
	if err != nil {
		// The one case that goes to stderr: a usage error means Vector
		// is misconfigured and no sample is coming, ever. Reporting it
		// as scrape_ok=0 would hide a permanent fault as a transient
		// one.
		fmt.Fprintf(os.Stderr, "qbt-peer-exporter: %v\n", err)
		os.Exit(exitUsage)
	}

	stats := scrape(context.Background(), base, capN)
	if err := peers.Emit(os.Stdout, stats); err != nil {
		os.Exit(1)
	}
}

// parseArgs reads the positional command line: [base [cap]].
func parseArgs(args []string) (base string, capN int, err error) {
	base, capN = defaultBase, peers.DefaultCap
	if len(args) > 0 {
		base = args[0]
	}
	if len(args) > 1 {
		capN, err = strconv.Atoi(args[1])
		if err != nil {
			return "", 0, fmt.Errorf("cap %q is not a number", args[1])
		}
	}
	return base, capN, nil
}

// scrape builds the client and takes one sample. A client that cannot be
// built yields the zero sample, which carries scrape_ok=0.
func scrape(ctx context.Context, base string, capN int) peers.Stats {
	client, err := qbittorrent.New(httpx.Config{
		BaseURL: base,
		Timeout: peers.CallTimeout,
		// No retries. The budget is the whole point of this tool's
		// shape, and a retrying client would spend it on one torrent.
		RetryMax: -1,
	})
	if err != nil {
		return peers.Stats{}
	}
	return peers.Scrape(ctx, client, capN, peers.Budget)
}
