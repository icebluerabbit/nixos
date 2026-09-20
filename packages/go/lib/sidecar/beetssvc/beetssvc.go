// Package beetssvc implements the sidecar contract over beets' web plugin.
//
// beets is the only thing in this stack that has read the files themselves.
// Lidarr knows what it asked for and what it filed; beets knows what the bytes
// actually say — which pressing, which label, which country, what the bitrate
// really is rather than what the release name claimed.
//
// It is also where the music loop closes. mb_releasegroupid is Lidarr's
// foreignAlbumId under a different name, so a tagged file resolves to the album
// that was searched for by shared identifier rather than by comparing titles.
//
// One deployment requirement: the plugin's include_paths option must be on. It
// defaults to false, and without it every item returns a null path — which
// removes the only field tying a tagged item to a file on disk, and with it the
// inode that links the file back to the Soulseek download that produced it.
package beetssvc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"homelab/clients/beets"
	"homelab/sidecar"
)

// Source is what a beets event carries in sidecar.Event.Source.
const Source = "beets"

// KindItem is one tagged file in beets' library.
const KindItem = "item"

// StatsPath is the sidecar route serving beets' own counts.
const StatsPath = "/stats"

// Service wraps one beets web plugin.
type Service struct {
	// beets has no webhook and no notification of any kind: it is a command
	// line tool with a read API bolted on.
	sidecar.Unsupported

	client   *beets.Client
	upstream string
}

// New builds the service. upstream is recorded for /info only.
func New(c *beets.Client, upstream string) *Service {
	return &Service{client: c, upstream: upstream}
}

// Info describes this sidecar without touching beets.
func (s *Service) Info() sidecar.Info {
	return sidecar.Info{
		Sidecar:      "managarr-beets",
		Source:       Source,
		Upstream:     s.upstream,
		Capabilities: []string{sidecar.CapChanges},
	}
}

// Changes returns library items after cursor, by ascending item id.
//
// Keyed on the item id rather than an offset, and it can be: /item/ has no
// paging — it returns the whole library in one response — so the collection is
// in hand and the cursor filters it rather than indexing into something that
// could shift.
//
// beets ids are assigned on import and never reused, so ascending id is a
// stable order. It is not chronological — a re-import keeps the id — but the
// contract asks that following the cursor cover everything, not that it run in
// time order, and beets is a library rather than a log.
func (s *Service) Changes(ctx context.Context, cursor string, limit int) ([]sidecar.Event, error) {
	after, err := parseID(cursor)
	if err != nil {
		return nil, err
	}

	items, err := s.client.Items(ctx)
	if err != nil {
		return nil, err
	}

	fresh := make([]beets.Item, 0, len(items))
	withPath := 0
	for _, it := range items {
		if it.Path != "" {
			withPath++
		}
		if it.ID > after {
			fresh = append(fresh, it)
		}
	}
	// Every item came back without a path, so include_paths is off. Said
	// here rather than left to the consumer: from managarr's side this looks
	// identical to a library of untagged imports.
	if len(items) > 0 && withPath == 0 {
		return nil, fmt.Errorf(
			"beetssvc: all %d items have no path; enable the web plugin's include_paths",
			len(items))
	}

	sort.Slice(fresh, func(i, j int) bool { return fresh[i].ID < fresh[j].ID })
	if len(fresh) > limit {
		fresh = fresh[:limit]
	}

	out := make([]sidecar.Event, 0, len(fresh))
	for _, it := range fresh {
		payload, err := json.Marshal(it)
		if err != nil {
			return nil, fmt.Errorf("beetssvc: encode item: %w", err)
		}
		out = append(out, sidecar.Event{
			Source: Source,
			Kind:   KindItem,
			// beets' import time, which is when it took the file rather
			// than when the file was made.
			OccurredAt: it.AddedAt(),
			// The path on disk. It is this table's key in managarr and the
			// only thing tying a tagged item to a file — and through that
			// file's inode, to the download that produced it.
			Ref:     it.Path,
			Payload: payload,
			Cursor:  strconv.FormatInt(it.ID, 10),
		})
	}
	return out, nil
}

// parseID reads a cursor as an exclusive lower bound on the item id.
//
// An empty cursor is -1, not 0. The filter is "id greater than this", so
// returning 0 for the beginning would skip the item with id 0 — and skip it
// silently, on the very first call, forever.
func parseID(cursor string) (int64, error) {
	if cursor == "" {
		return -1, nil
	}
	n, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("beetssvc: bad cursor %q", cursor)
	}
	return n, nil
}

// Routes serves beets' own counts.
//
// Also the cheapest check that the plugin is reachable and has a library behind
// it, which /healthz deliberately does not test.
func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+StatsPath, func(w http.ResponseWriter, r *http.Request) {
		stats, err := s.client.Stats(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stats)
	})
}
