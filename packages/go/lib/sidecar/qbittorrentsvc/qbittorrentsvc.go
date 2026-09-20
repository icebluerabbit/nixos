// Package qbittorrentsvc implements the sidecar contract over qBittorrent.
//
// The one sidecar here that adds a capability rather than a cursor.
//
// qBittorrent keeps no history. Its API reports what it holds RIGHT NOW, so a
// torrent added and removed between two hourly passes leaves nothing behind —
// no record that it existed, what it was, or where its bytes went. Every other
// service in this stack persists its own past and can be backfilled from it;
// this one cannot, which makes a missed event here real data loss rather than
// latency.
//
// What it does have is one event: an external program run on completion. That
// program posts here, and this fills in everything the argv leaves out.
package qbittorrentsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"homelab/clients/qbittorrent"
	"homelab/sidecar"
)

// Source is what a qBittorrent event carries in sidecar.Event.Source.
const Source = "qbittorrent"

// Event kinds this service emits.
const (
	// KindAdded is a torrent seen for the first time, by added_on.
	KindAdded = "added"
	// KindCompleted is the completion hook firing, enriched.
	KindCompleted = "completed"
)

// Routes serving what is state rather than events.
const (
	// TorrentsPath is the full current snapshot.
	TorrentsPath = "/torrents"
	// FilesPath is one torrent's file list, by ?hash=.
	FilesPath = "/files"
	// TrackersPath is one torrent's trackers, by ?hash=.
	TrackersPath = "/trackers"
)

// Service wraps one qBittorrent instance.
type Service struct {
	client   *qbittorrent.Client
	upstream string
}

// New builds the service. upstream is recorded for /info only.
func New(c *qbittorrent.Client, upstream string) *Service {
	return &Service{client: c, upstream: upstream}
}

// Info describes this sidecar without touching qBittorrent.
func (s *Service) Info() sidecar.Info {
	return sidecar.Info{
		Sidecar:  "managarr-qbittorrent",
		Source:   Source,
		Upstream: s.upstream,
		Capabilities: []string{
			sidecar.CapChanges,
			sidecar.CapEvents,
		},
	}
}

// cursor is a position in the added_on ordering.
//
// (added_on, hash), not added_on alone. qBittorrent records it in whole seconds
// and a *arr grabbing a season pack adds several torrents inside one, so a
// timestamp cursor either re-reads that group forever or steps past part of it.
// The hash breaks every tie and is stable for the life of the torrent.
type cursor struct {
	addedOn int64
	hash    string
}

func (c cursor) String() string {
	if c == (cursor{}) {
		return ""
	}
	return fmt.Sprintf("%d:%s", c.addedOn, c.hash)
}

// parseCursor reads one back. Empty is the beginning, not an error.
func parseCursor(s string) (cursor, error) {
	if s == "" {
		return cursor{}, nil
	}
	secs, hash, ok := strings.Cut(s, ":")
	if !ok {
		return cursor{}, fmt.Errorf("qbittorrentsvc: bad cursor %q", s)
	}
	n, err := strconv.ParseInt(secs, 10, 64)
	if err != nil {
		return cursor{}, fmt.Errorf("qbittorrentsvc: bad cursor %q", s)
	}
	return cursor{addedOn: n, hash: hash}, nil
}

// after reports whether t sorts after the cursor.
func (c cursor) after(t qbittorrent.Torrent) bool {
	if t.AddedOn != c.addedOn {
		return t.AddedOn > c.addedOn
	}
	return t.Hash > c.hash
}

// Changes returns torrents added after cursor, oldest first.
//
// Additions only. This is the append-only part of qBittorrent — a torrent's
// added_on never moves — and it is what lets managarr learn about a torrent
// within a poll rather than an hour. Everything that CHANGES about a torrent
// afterwards, its state and ratio and progress, is a snapshot and is served
// whole on /torrents instead; expressing a ratio tick as an event would
// manufacture an event stream the client does not have.
//
// The whole list is fetched and sorted per call because qBittorrent offers no
// filter and no ordering. It is a few thousand torrents, which is one request.
func (s *Service) Changes(ctx context.Context, cur string, limit int) ([]sidecar.Event, error) {
	pos, err := parseCursor(cur)
	if err != nil {
		return nil, err
	}

	torrents, err := s.client.Torrents(ctx)
	if err != nil {
		return nil, err
	}

	fresh := make([]qbittorrent.Torrent, 0, len(torrents))
	for _, t := range torrents {
		if pos == (cursor{}) || pos.after(t) {
			fresh = append(fresh, t)
		}
	}
	sortByAdded(fresh)
	if len(fresh) > limit {
		fresh = fresh[:limit]
	}

	out := make([]sidecar.Event, 0, len(fresh))
	for _, t := range fresh {
		payload, err := json.Marshal(t)
		if err != nil {
			return nil, fmt.Errorf("qbittorrentsvc: encode torrent: %w", err)
		}
		out = append(out, sidecar.Event{
			Source:     Source,
			Kind:       KindAdded,
			OccurredAt: unix(t.AddedOn),
			Ref:        strings.ToLower(t.Hash),
			Payload:    payload,
			Cursor:     cursor{addedOn: t.AddedOn, hash: t.Hash}.String(),
		})
	}
	return out, nil
}

// completion is what qBittorrent's external program posts.
//
// The hook is "run this command when a torrent finishes", so the body is
// whatever that command was told to send — this homelab's shape, built from
// qBittorrent's %-macros, not a vendor schema. These five are everything
// qBittorrent will hand a hook.
type completion struct {
	Event    string `json:"event"`
	Infohash string `json:"infohash"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Size     string `json:"size"`
}

// enriched is a completion once the sidecar has filled it in.
//
// Flat rather than nesting the completion under a key: it is one event, and the
// fields the hook supplied are no less part of it than the ones read back.
type enriched struct {
	completion
	Torrent  *qbittorrent.Torrent  `json:"torrent,omitempty"`
	Files    []qbittorrent.File    `json:"files,omitempty"`
	Trackers []qbittorrent.Tracker `json:"trackers,omitempty"`
}

// Enrich fills in a completion hook.
//
// This is the whole point of the sidecar. qBittorrent's hook passes a handful
// of %-macros on a command line and nothing else — no file list, no tracker, no
// sizes. Those are reachable only while the torrent is still in the client, and
// the window between a completion and a *arr importing and removing it is
// minutes. An hourly pass is not there to see it.
//
// A lookup that fails is not fatal: the bare completion is still worth more
// than nothing, because after the torrent is gone nothing else will ever
// mention it.
func (s *Service) Enrich(ctx context.Context, body []byte) ([]sidecar.Event, error) {
	var c completion
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("qbittorrentsvc: decode completion: %w", err)
	}
	if c.Infohash == "" {
		return nil, fmt.Errorf("qbittorrentsvc: completion has no infohash")
	}
	// Lowercased once, here. qBittorrent reports it uppercase and everything
	// downstream keys on lowercase.
	hash := strings.ToLower(c.Infohash)
	c.Infohash = hash
	payload := enriched{completion: c}
	if torrents, err := s.client.Torrents(ctx); err == nil {
		for i := range torrents {
			if strings.EqualFold(torrents[i].Hash, hash) {
				payload.Torrent = &torrents[i]
				break
			}
		}
	}
	if files, err := s.client.Files(ctx, hash); err == nil {
		payload.Files = files
	}
	if trackers, err := s.client.Trackers(ctx, hash); err == nil {
		payload.Trackers = trackers
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("qbittorrentsvc: encode completion: %w", err)
	}

	var at int64
	if payload.Torrent != nil {
		at = payload.Torrent.CompletionOn
	}
	return []sidecar.Event{{
		Source:     Source,
		Kind:       KindCompleted,
		OccurredAt: unix(at),
		Ref:        hash,
		Payload:    encoded,
	}}, nil
}

// Routes serves what is state rather than events.
func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+TorrentsPath, func(w http.ResponseWriter, r *http.Request) {
		torrents, err := s.client.Torrents(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if torrents == nil {
			torrents = []qbittorrent.Torrent{}
		}
		writeJSON(w, map[string]any{"torrents": torrents})
	})

	mux.HandleFunc("GET "+FilesPath, func(w http.ResponseWriter, r *http.Request) {
		hash := r.URL.Query().Get("hash")
		if hash == "" {
			http.Error(w, "hash is required", http.StatusBadRequest)
			return
		}
		files, err := s.client.Files(r.Context(), strings.ToLower(hash))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if files == nil {
			files = []qbittorrent.File{}
		}
		writeJSON(w, map[string]any{"files": files})
	})

	mux.HandleFunc("GET "+TrackersPath, func(w http.ResponseWriter, r *http.Request) {
		hash := r.URL.Query().Get("hash")
		if hash == "" {
			http.Error(w, "hash is required", http.StatusBadRequest)
			return
		}
		trackers, err := s.client.Trackers(r.Context(), strings.ToLower(hash))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if trackers == nil {
			trackers = []qbittorrent.Tracker{}
		}
		writeJSON(w, map[string]any{"trackers": trackers})
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// sortByAdded orders torrents oldest first, ties broken by hash — the same
// ordering the cursor encodes, so a page boundary lands in a defined place.
func sortByAdded(ts []qbittorrent.Torrent) {
	sort.Slice(ts, func(i, j int) bool {
		if ts[i].AddedOn != ts[j].AddedOn {
			return ts[i].AddedOn < ts[j].AddedOn
		}
		return ts[i].Hash < ts[j].Hash
	})
}

// unix converts a qBittorrent timestamp, which is 0 or negative when the client
// has nothing to report rather than a real instant at the epoch.
func unix(secs int64) time.Time {
	if secs <= 0 {
		return time.Time{}
	}
	return time.Unix(secs, 0).UTC()
}
