// Package bazarrsvc implements the sidecar contract over Bazarr.
//
// Bazarr is the only source of subtitle state, and the only source of an
// ABSENCE: which languages are still wanted cannot be observed on disk, because
// a missing subtitle leaves nothing behind. Its history is also the only record
// of which provider a subtitle came from and what it scored.
//
// No webhook. Bazarr notifies through Apprise providers rather than a plain
// HTTP callback, so there is nothing for this to receive and Enrich reports
// itself unsupported rather than accepting bodies it will never get.
package bazarrsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"homelab/clients/bazarr"
	"homelab/sidecar"
)

// Source is what a Bazarr event carries in sidecar.Event.Source.
const Source = "bazarr"

// Event kinds this service emits, one per half of the library.
const (
	KindSeriesHistory = "series_history"
	KindMovieHistory  = "movie_history"
)

// WantedPath is the sidecar route serving what is still missing.
const WantedPath = "/wanted"

// halves are Bazarr's two collections.
//
// Bazarr keeps series and movies apart rather than in one table with a
// discriminator, so a walk has to cover both. Ordered, and the order is part of
// the cursor: series finish before movies start.
var halves = []struct {
	apiKind string
	event   string
}{
	{apiKind: "series", event: KindSeriesHistory},
	{apiKind: "movies", event: KindMovieHistory},
}

// Service wraps one Bazarr instance.
type Service struct {
	// Bazarr has no webhook: it notifies through Apprise, not HTTP callbacks.
	sidecar.Unsupported

	client   *bazarr.Client
	upstream string
}

// New builds the service. upstream is recorded for /info only.
func New(c *bazarr.Client, upstream string) *Service {
	return &Service{client: c, upstream: upstream}
}

// Info describes this sidecar without touching Bazarr.
func (s *Service) Info() sidecar.Info {
	return sidecar.Info{
		Sidecar:      "managarr-bazarr",
		Source:       Source,
		Upstream:     s.upstream,
		Capabilities: []string{sidecar.CapChanges},
	}
}

// cursor is a position within the two-stage walk.
type cursor struct {
	// stage indexes halves. Past the end means the walk is finished.
	stage int
	// lastID is the highest record id read in that stage.
	lastID int64
}

func (c cursor) String() string { return fmt.Sprintf("%d:%d", c.stage, c.lastID) }

// parseCursor reads one back.
//
// An empty cursor starts at lastID -1, not 0. The filter is "id greater than
// this", so 0 would skip the record with id 0 — silently, on the first call,
// forever. Bazarr numbers from 1 today, which is exactly the kind of thing that
// makes the bug invisible until it is not.
func parseCursor(s string) (cursor, error) {
	if s == "" {
		return cursor{lastID: -1}, nil
	}
	var c cursor
	if _, err := fmt.Sscanf(s, "%d:%d", &c.stage, &c.lastID); err != nil || c.stage < 0 {
		return cursor{}, fmt.Errorf("bazarrsvc: bad cursor %q", s)
	}
	return c, nil
}

// Changes returns history after cursor, oldest first within each half.
//
// Keyed on the record id rather than an offset, and it can be: Bazarr's history
// endpoint has no paging at all — it takes start and length and this asks for
// everything — so the whole collection is in hand and the cursor is a filter
// over it rather than a position that something could shift.
//
// A half that runs dry advances to the next rather than ending the walk, so a
// consumer never has to know there are two of them.
func (s *Service) Changes(ctx context.Context, cur string, limit int) ([]sidecar.Event, error) {
	pos, err := parseCursor(cur)
	if err != nil {
		return nil, err
	}

	out := make([]sidecar.Event, 0, limit)
	for pos.stage < len(halves) && len(out) < limit {
		half := halves[pos.stage]

		records, err := s.client.History(ctx, half.apiKind)
		if err != nil {
			return nil, err
		}

		fresh := make([]bazarr.HistoryRecord, 0, len(records))
		for _, r := range records {
			if r.ID > pos.lastID {
				fresh = append(fresh, r)
			}
		}
		sort.Slice(fresh, func(i, j int) bool { return fresh[i].ID < fresh[j].ID })
		if room := limit - len(out); len(fresh) > room {
			fresh = fresh[:room]
		}

		for _, r := range fresh {
			payload, err := json.Marshal(r)
			if err != nil {
				return nil, fmt.Errorf("bazarrsvc: encode record: %w", err)
			}
			pos.lastID = r.ID
			out = append(out, sidecar.Event{
				Source: Source,
				Kind:   half.event,
				// The video file the subtitle belongs to. Bazarr's own
				// timestamp is a preformatted display string rather than an
				// instant, so OccurredAt is left unset and managarr
				// substitutes receipt time — better than parsing prose.
				Ref:     r.VideoPath,
				Payload: payload,
				Cursor:  pos.String(),
			})
		}

		// Nothing fresh left in this half. Move on rather than stop: the
		// consumer's walk ends on a short page, and ending it here would
		// leave movies permanently unread.
		if len(fresh) == 0 || len(out) < limit {
			pos.stage++
			pos.lastID = -1
			// The boundary has to be recorded, or a consumer that stops on
			// this page resumes at the end of a finished half and reads it
			// again.
			if n := len(out); n > 0 {
				out[n-1].Cursor = pos.String()
			}
		}
	}
	return out, nil
}

// Routes serves what is still missing.
//
// Not events and not derivable from anything else: a subtitle that was never
// downloaded leaves no trace on disk, so this is the only way to know a language
// is still wanted.
func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+WantedPath, func(w http.ResponseWriter, r *http.Request) {
		kind := r.URL.Query().Get("kind")
		if kind != "series" && kind != "movies" {
			http.Error(w, "kind must be series or movies", http.StatusBadRequest)
			return
		}
		blacklist, err := s.client.Blacklist(r.Context(), kind)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if blacklist == nil {
			blacklist = []bazarr.BlacklistRecord{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"blacklist": blacklist})
	})
}
