// Package autobrrsvc implements the sidecar contract over autobrr.
//
// autobrr is the only record of what was ANNOUNCED. Every release that appeared
// on every tracker's IRC channel passes through it, and it keeps the ones it
// rejected alongside the ones it grabbed — so it answers "was this ever offered
// to me, and why did I not take it", which nothing downstream can.
//
// The rejections are most of the corpus and exist nowhere else: a release the
// filter declined never reaches a *arr, a torrent client or a tracker's history.
package autobrrsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"homelab/clients/autobrr"
	"homelab/sidecar"
)

// Source is what an autobrr event carries in sidecar.Event.Source.
const Source = "autobrr"

// Event kinds this service emits.
const (
	// KindRelease is one announce and what autobrr decided about it.
	KindRelease = "release"
	// KindAction is a filter action firing, posted by autobrr itself.
	KindAction = "action"
)

// Routes serving autobrr's lookup tables.
const (
	IndexersPath = "/indexers"
	FiltersPath  = "/filters"
)

// Service wraps one autobrr instance.
type Service struct {
	client   *autobrr.Client
	upstream string
}

// New builds the service. upstream is recorded for /info only.
func New(c *autobrr.Client, upstream string) *Service {
	return &Service{client: c, upstream: upstream}
}

// Info describes this sidecar without touching autobrr.
func (s *Service) Info() sidecar.Info {
	return sidecar.Info{
		Sidecar:  "managarr-autobrr",
		Source:   Source,
		Upstream: s.upstream,
		Capabilities: []string{
			sidecar.CapChanges,
			sidecar.CapEvents,
		},
	}
}

// Changes returns releases after cursor, oldest first.
//
// The cursor is an ascending offset. autobrr serves this list newest-first and
// pages by offset, which cannot be a cursor — a release arriving mid-walk
// shifts every later page down by one. Counting from the oldest end instead
// gives a position that new releases cannot move.
func (s *Service) Changes(ctx context.Context, cursor string, limit int) ([]sidecar.Event, error) {
	offset, err := parseOffset(cursor)
	if err != nil {
		return nil, err
	}

	page, err := s.client.ReleasesPageAsc(ctx, offset, limit)
	if err != nil {
		return nil, err
	}

	out := make([]sidecar.Event, 0, len(page.Records))
	for i, r := range page.Records {
		payload, err := json.Marshal(r)
		if err != nil {
			return nil, fmt.Errorf("autobrrsvc: encode release: %w", err)
		}
		out = append(out, sidecar.Event{
			Source:     Source,
			Kind:       KindRelease,
			OccurredAt: r.Timestamp,
			// The release name. autobrr never sees an infohash — it reads
			// an announce line and hands a URL onward — so the name is the
			// only identifier it and the rest of the stack share.
			Ref:     r.Name,
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
		return 0, fmt.Errorf("autobrrsvc: bad cursor %q", cursor)
	}
	return n, nil
}

// Enrich records a filter action firing.
//
// autobrr has no notification webhook in the usual sense; what it has is an
// action of type "webhook" that a filter fires when it matches, with a body the
// filter author writes from autobrr's template macros. So the payload shape is
// this homelab's own, not a vendor schema, and the sidecar keeps it whole
// rather than reaching into it.
//
// Nothing is fetched back. The action fires at the moment of the match, and the
// release row it refers to is already on its way through Changes — enriching
// here would race autobrr's own write.
func (s *Service) Enrich(_ context.Context, body []byte) ([]sidecar.Event, error) {
	var action struct {
		// Both spellings: autobrr's macro is {{.TorrentName}}, and the
		// filters here have used either depending on when they were written.
		Name        string `json:"name"`
		TorrentName string `json:"torrent_name"`
		ReleaseName string `json:"release_name"`
	}
	if err := json.Unmarshal(body, &action); err != nil {
		return nil, fmt.Errorf("autobrrsvc: decode action: %w", err)
	}

	ref := action.Name
	if ref == "" {
		ref = action.TorrentName
	}
	if ref == "" {
		ref = action.ReleaseName
	}

	return []sidecar.Event{{
		Source:  Source,
		Kind:    KindAction,
		Ref:     ref,
		Payload: json.RawMessage(body),
	}}, nil
}

// Routes serves autobrr's lookup tables.
//
// Indexers and filters are configuration rather than events: a release row
// names its filter and indexer as strings, and these are what turn those into
// the thing they refer to.
func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+IndexersPath, func(w http.ResponseWriter, r *http.Request) {
		indexers, err := s.client.Indexers(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if indexers == nil {
			indexers = []autobrr.Indexer{}
		}
		writeJSON(w, map[string]any{"indexers": indexers})
	})

	mux.HandleFunc("GET "+FiltersPath, func(w http.ResponseWriter, r *http.Request) {
		filters, err := s.client.Filters(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if filters == nil {
			filters = []autobrr.Filter{}
		}
		writeJSON(w, map[string]any{"filters": filters})
	})
}

// writeJSON sends a result. The status is already committed by the time
// encoding can fail, so there is nothing left to tell the client.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
