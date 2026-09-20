// Package slskdsvc implements the sidecar contract over slskd.
//
// slskd's event log is the authoritative record of the Soulseek side, and the
// only one. /transfers is a live view that expires a transfer once it finishes,
// so it shows what is in flight and nothing else; the event log is persisted and
// is the only place slskd states the mapping between a peer's remote path and
// the local file it wrote.
//
// That mapping is what closes the music loop. A library file shares an inode
// with the download that produced it — beets imports by move — and the download
// names the peer and the remote path, which names the search result, which names
// the search. Without this log the chain starts at a file with no provenance.
//
// Uploads matter too, in the other direction: on an upload LocalFilename is a
// file of yours a peer took, which is a library path, so the log is also the
// record of what this homelab has shared.
package slskdsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"homelab/clients/slskd"
	"homelab/sidecar"
)

// Source is what an slskd event carries in sidecar.Event.Source.
const Source = "slskd"

// KindEvent is one entry of slskd's event log, whatever its type.
//
// Not split per type. slskd's own Type field already says which it is, and a
// sidecar that enumerated them would need a release every time slskd added one.
const KindEvent = "event"

// SearchesPath is the sidecar route serving the live search list.
const SearchesPath = "/searches"

// Service wraps one slskd instance.
type Service struct {
	// Enrich is unsupported: slskd has no webhook. Its journal reaches
	// managarr through the Vector sidecar instead, which is a different
	// transport for a different reason — log lines, not application events.
	sidecar.Unsupported

	client   *slskd.Client
	upstream string
	pageSize int
}

// New builds the service. upstream is recorded for /info only.
func New(c *slskd.Client, upstream string, pageSize int) *Service {
	if pageSize <= 0 {
		pageSize = 500
	}
	return &Service{client: c, upstream: upstream, pageSize: pageSize}
}

// Info describes this sidecar without touching slskd.
func (s *Service) Info() sidecar.Info {
	return sidecar.Info{
		Sidecar:      "managarr-slskd",
		Source:       Source,
		Upstream:     s.upstream,
		Capabilities: []string{sidecar.CapChanges},
	}
}

// Changes returns a page of the event log, newest first.
//
// The cursor is a descending offset, and the direction is slskd's own: it
// serves /api/v0/events newest-first and reports no total, so there is nothing
// to count back from to turn that into an ascending position.
//
// Walking in the native direction is what makes the offset safe. New events are
// prepended, so one arriving mid-walk pushes everything to a HIGHER offset — a
// page already read gets read again rather than jumped over. A duplicate costs
// an idempotent upsert on managarr's side; a skip would cost the record, and
// the record exists nowhere else.
func (s *Service) Changes(ctx context.Context, cursor string, limit int) ([]sidecar.Event, error) {
	offset, err := parseOffset(cursor)
	if err != nil {
		return nil, err
	}

	// Capped below whatever the consumer asked for. Every download event
	// embeds its entire transfer object, so a page the contract would happily
	// allow is hundreds of megabytes off the wire and in memory at once.
	if limit > s.pageSize {
		limit = s.pageSize
	}

	events, err := s.client.EventsPageAt(ctx, offset, limit)
	if err != nil {
		return nil, err
	}

	out := make([]sidecar.Event, 0, len(events))
	for i, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return nil, fmt.Errorf("slskdsvc: encode event: %w", err)
		}
		out = append(out, sidecar.Event{
			Source:     Source,
			Kind:       KindEvent,
			OccurredAt: e.Timestamp,
			// slskd's own event id. The transfer events carry a peer and a
			// path inside Data rather than at the top level, and reaching
			// in here would make this sidecar know one payload shape out of
			// a dozen — managarr decodes it where it already has the types.
			Ref:     e.ID,
			Payload: payload,
			Cursor:  strconv.Itoa(offset + i + 1),
		})
	}
	return out, nil
}

// parseOffset reads a cursor. Empty is the beginning, not an error.
func parseOffset(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(cursor)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("slskdsvc: bad cursor %q", cursor)
	}
	return n, nil
}

// Routes serves the live search list.
//
// Searches are state, not events: slskd keeps them until they are deleted, and
// what managarr wants is which are still there. They are also the far end of
// the music chain — a download names a peer and a remote path, and the search
// is what asked for it.
func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+SearchesPath, func(w http.ResponseWriter, r *http.Request) {
		searches, err := s.client.Searches(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if searches == nil {
			searches = []slskd.Search{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"searches": searches})
	})
}
