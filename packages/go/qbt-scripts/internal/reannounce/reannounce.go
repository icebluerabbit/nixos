// Package reannounce forces a tracker announce for torrents that have not
// started and whose trackers have all failed.
//
// A torrent added while its tracker was unreachable keeps the announce
// interval it was given, so it sits at zero progress until that interval
// elapses — half an hour or more on a private tracker, for a grab that would
// otherwise have started immediately. Asking again costs one announce.
//
// Only zero-progress torrents are touched. A torrent that has downloaded
// anything has already talked to someone, so whatever its tracker list says
// now is not what is holding it up.
package reannounce

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"homelab/app"
	"homelab/clients/qbittorrent"
	"homelab/httpx"
)

// Timeout bounds one WebAPI call. The client is in the same container, so a
// call that takes longer than this is not slow, it is wedged.
const Timeout = 10 * time.Second

// pseudoTrackerPrefix marks qBittorrent's DHT, PeX and LSD rows, which it
// returns in the tracker list as "** [DHT] **". They are not trackers and
// never announce, so a torrent carrying nothing else has no tracker to fail.
const pseudoTrackerPrefix = "** ["

// Config is what the tool takes on the command line.
type Config struct {
	// API is the qBittorrent WebUI root.
	API string `json:"api"`
}

// Defaults returns the configuration before flags and the environment.
func Defaults() Config { return Config{} }

// AllFailed reports whether every real tracker on a torrent is in a failed
// state.
//
// qBittorrent 5.x split the old catch-all 4 into 4 (not working), 5 (tracker
// error, which is how "unregistered torrent" arrives) and 6 (unreachable), so
// the test is >= 4 rather than == 4. 0 disabled, 1 not contacted, 2 working
// and 3 updating are all fine.
//
// A torrent with no real trackers reports false: there is nothing to
// re-announce to.
func AllFailed(trackers []qbittorrent.Tracker) bool {
	announcing := 0
	for _, t := range trackers {
		if strings.HasPrefix(t.URL, pseudoTrackerPrefix) {
			continue
		}
		announcing++
		if t.Status < qbittorrent.TrackerNotWorking {
			return false
		}
	}
	return announcing > 0
}

// Run re-announces every stalled torrent it finds.
func Run(ctx context.Context, env *app.Env, cfg Config) error {
	client, err := qbittorrent.New(httpx.Config{
		BaseURL: cfg.API,
		Timeout: Timeout,
		// No retries: this runs on a timer and a failed pass costs
		// nothing, while a retrying pass holds the unit open.
		RetryMax: -1,
		Logger:   env.Logger,
	})
	if err != nil {
		return err
	}

	torrents, err := client.TorrentsFiltered(ctx, "")
	if err != nil {
		return err
	}

	for _, t := range torrents {
		if t.Progress != 0 {
			continue
		}
		trackers, err := client.Trackers(ctx, t.Hash)
		if err != nil {
			return err
		}
		if !AllFailed(trackers) {
			continue
		}
		if env.DryRun {
			env.Logger.Info("would reannounce",
				slog.String("hash", t.Hash), slog.String("name", t.Name))
			continue
		}
		if err := client.Reannounce(ctx, []string{t.Hash}); err != nil {
			return err
		}
		env.Logger.Info("reannounced, all trackers errored",
			slog.String("hash", t.Hash), slog.String("name", t.Name))
	}
	return nil
}
