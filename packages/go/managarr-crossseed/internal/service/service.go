// Package service implements the sidecar contract over cross-seed's database.
//
// Three methods and two extra routes. Everything else — the listener, the
// contract routes, the relay, the metrics — is lib/sidecar's, which is what
// keeps this file to the part that is actually about cross-seed.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"homelab/clients/crossseed"
	"homelab/managarr-crossseed/internal/store"
	"homelab/sidecar"
)

// Service reads cross-seed's decision log.
//
// Enrich comes from the embedded Unsupported: cross-seed's webhook fires only
// on an injection, so everything worth knowing is in the database and nothing
// arrives by push. A sidecar that wrapped the webhook would report the matches
// and none of the rejections — which is the exact gap this exists to close.
type Service struct {
	sidecar.Unsupported

	store  *store.Store
	dbPath string
}

// New builds the service over cross-seed's live database path.
func New(dbPath string) *Service {
	return &Service{store: store.New(dbPath), dbPath: dbPath}
}

// Info describes this sidecar. Touches nothing, so it answers even when the
// database is missing — which is when it is most worth asking.
func (s *Service) Info() sidecar.Info {
	return sidecar.Info{
		Sidecar:      "managarr-crossseed",
		Source:       crossseed.Source,
		Upstream:     s.dbPath,
		Capabilities: []string{sidecar.CapChanges},
	}
}

// Changes returns decisions recorded after cursor, oldest first.
func (s *Service) Changes(ctx context.Context, cursor string, limit int) ([]sidecar.Event, error) {
	after, err := store.ParseCursor(cursor)
	if err != nil {
		return nil, err
	}

	rows, err := s.store.Decisions(ctx, after, limit)
	if err != nil {
		return nil, err
	}

	out := make([]sidecar.Event, 0, len(rows))
	for _, r := range rows {
		payload, err := json.Marshal(r.Decision)
		if err != nil {
			return nil, fmt.Errorf("service: encode decision: %w", err)
		}
		out = append(out, sidecar.Event{
			Source:     crossseed.Source,
			Kind:       crossseed.KindDecision,
			OccurredAt: r.FirstSeen,
			// The searchee name, not the candidate hash. It is what
			// managarr resolves a release by; the candidate is
			// frequently a torrent this stack never obtained.
			Ref:     r.SearcheeName,
			Payload: payload,
			Cursor:  r.Cursor.String(),
		})
	}
	return out, nil
}

// Routes adds what is not events.
//
// Both of these are lookup tables rather than a stream, so forcing them through
// /changes would mean inventing a timestamp and a cursor for rows that have
// neither.
func (s *Service) Routes(mux *http.ServeMux) {
	// The name-to-infohash map: the only bridge from a name-keyed searchee
	// to a real download, and one managarr needs whole rather than paged.
	mux.HandleFunc("GET "+crossseed.ClientTorrentsPath, func(w http.ResponseWriter, r *http.Request) {
		torrents, err := s.store.ClientTorrents(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if torrents == nil {
			torrents = []crossseed.ClientTorrent{}
		}
		writeJSON(w, map[string]any{"clientTorrents": torrents})
	})

	// cross-seed's own record of which infohash produced which linked path.
	// An injected cross-seed lives in its own linkDir and shares no path with
	// the torrent it was matched against, so this is where cross-seed states
	// that the two are the same content — by identifier, not by name.
	mux.HandleFunc("GET "+crossseed.EnsemblePath, func(w http.ResponseWriter, r *http.Request) {
		entries, err := s.store.Ensemble(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if entries == nil {
			entries = []crossseed.EnsembleEntry{}
		}
		writeJSON(w, map[string]any{"ensemble": entries})
	})

	// Row counts. Doubles as the readiness check that actually opens the
	// database, which /healthz deliberately does not.
	mux.HandleFunc("GET "+crossseed.StatsPath, func(w http.ResponseWriter, r *http.Request) {
		stats, err := s.store.Stats(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, stats)
	})
}

// writeJSON sends a result. The status is already committed by the time
// encoding can fail, so there is nothing left to tell the client.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
