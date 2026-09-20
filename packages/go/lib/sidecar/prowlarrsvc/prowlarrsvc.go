// Package prowlarrsvc implements the sidecar contract over Prowlarr.
//
// Prowlarr is the only place the search side of the stack is recorded. Every
// query a *arr sent to every indexer, what came back, and how long it took,
// live here and nowhere else — the *arr keeps what it grabbed, not what it
// asked. That history is also the only evidence for a release that was offered
// and never taken.
//
// Separate from arrsvc despite the name: Prowlarr's history is a different
// shape, split across three event types with a per-event data bag rather than
// one record type, and it has no queue to enrich a notification from.
package prowlarrsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"homelab/clients/prowlarr"
	"homelab/sidecar"
)

// Source is what a Prowlarr event carries in sidecar.Event.Source.
const Source = "prowlarr"

// Event kinds this service emits, one per Prowlarr event type.
const (
	// KindGrabbed is a release pulled through Prowlarr by a client.
	KindGrabbed = "grabbed"
	// KindQuery is a search: a client asked an indexer a question.
	KindQuery = "query"
	// KindRSS is a scheduled feed poll, not a user-driven search.
	KindRSS = "rss"
	// KindWebhook is a Prowlarr notification, relayed as sent.
	KindWebhook = "webhook"
)

// IndexersPath is the sidecar route serving the indexer table.
const IndexersPath = "/indexers"

// walk is one event type and the kind it becomes.
//
// Ordered, and the order is part of the cursor: the walk finishes grabs before
// starting queries. Interleaving by date would need a merge across
// independently paged endpoints for no gain — managarr correlates by
// identifier, never by arrival order.
//
// RSS polls are deliberately absent. A scheduled feed fetch is not a question
// anyone asked, managarr records none of them, and at 44,000 records they
// outnumber real searches two to one — so serving them would mean paging tens
// of thousands of rows across the VLAN for a consumer that drops every one.
// KindRSS stays defined: the vocabulary is still accurate about what Prowlarr
// has, and a future consumer that wants them only needs a stage added here.
var walk = []struct {
	event prowlarr.EventType
	kind  string
}{
	{prowlarr.EventReleaseGrabbed, KindGrabbed},
	{prowlarr.EventIndexerQuery, KindQuery},
}

// Service wraps one Prowlarr instance.
type Service struct {
	client   *prowlarr.Client
	upstream string
}

// New builds the service. upstream is recorded for /info only.
func New(c *prowlarr.Client, upstream string) *Service {
	return &Service{client: c, upstream: upstream}
}

// Info describes this sidecar without touching Prowlarr.
func (s *Service) Info() sidecar.Info {
	return sidecar.Info{
		Sidecar:  "managarr-prowlarr",
		Source:   Source,
		Upstream: s.upstream,
		Capabilities: []string{
			sidecar.CapChanges,
			sidecar.CapEvents,
		},
	}
}

// cursor is a position within the three-stage walk.
type cursor struct {
	// stage indexes walk. Past the end means the walk is finished.
	stage int
	// offset is how many records of that stage have been read.
	offset int
}

func (c cursor) String() string { return fmt.Sprintf("%d:%d", c.stage, c.offset) }

// parseCursor reads one back. Empty is the beginning, not an error.
func parseCursor(s string) (cursor, error) {
	if s == "" {
		return cursor{}, nil
	}
	var c cursor
	if _, err := fmt.Sscanf(s, "%d:%d", &c.stage, &c.offset); err != nil ||
		c.stage < 0 || c.offset < 0 {
		return cursor{}, fmt.Errorf("prowlarrsvc: bad cursor %q", s)
	}
	return c, nil
}

// Changes returns history after cursor, oldest first within each stage.
//
// A stage that runs dry advances to the next rather than ending the walk, so a
// consumer never has to know there are three of them. The walk ends only when
// the last stage is exhausted.
func (s *Service) Changes(ctx context.Context, cur string, limit int) ([]sidecar.Event, error) {
	pos, err := parseCursor(cur)
	if err != nil {
		return nil, err
	}

	out := make([]sidecar.Event, 0, limit)
	for pos.stage < len(walk) && len(out) < limit {
		stage := walk[pos.stage]

		res, err := s.client.HistoryPageAt(ctx, stage.event,
			pos.offset/limit+1, limit)
		if err != nil {
			return nil, err
		}

		skip := pos.offset % limit
		records := res.Records
		if skip < len(records) {
			records = records[skip:]
		} else {
			records = nil
		}
		if room := limit - len(out); len(records) > room {
			records = records[:room]
		}

		for _, r := range records {
			payload, err := json.Marshal(r)
			if err != nil {
				return nil, fmt.Errorf("prowlarrsvc: encode record: %w", err)
			}
			pos.offset++
			out = append(out, sidecar.Event{
				Source:     Source,
				Kind:       stage.kind,
				OccurredAt: r.Date,
				// The release title for a grab, the search term for a
				// query. Prowlarr keys nothing by infohash — it never sees
				// the torrent, only the link it handed over.
				Ref:     ref(r),
				Payload: payload,
				Cursor:  pos.String(),
			})
		}

		// This stage is spent when the page came back short or the offset
		// reached the total Prowlarr reports. Move on rather than stop: the
		// consumer's walk ends on a short page, and ending it here would
		// leave the later stages permanently unread.
		if pos.offset >= res.TotalRecords || len(res.Records) < limit {
			pos.stage++
			pos.offset = 0
			// The stage boundary has to be recorded, or a consumer that
			// stops on this page resumes at the end of a finished stage and
			// reads it again.
			if n := len(out); n > 0 {
				out[n-1].Cursor = pos.String()
			}
		}
	}
	return out, nil
}

// ref picks the identifier a record is about.
//
// Prowlarr's data bag differs per event type and no key is guaranteed, so this
// takes the first that is present rather than assuming a shape.
func ref(r prowlarr.HistoryRecord) string {
	for _, key := range []string{"title", "query", "sourceTitle", "grabTitle"} {
		if v := strings.TrimSpace(r.Str(key)); v != "" {
			return v
		}
	}
	return ""
}

// Enrich relays a Prowlarr notification.
//
// Nothing is added. Prowlarr has no queue and no per-release detail beyond what
// the notification already carries, so a callback would fetch nothing the body
// does not have. The sidecar still normalises it into the envelope, which is
// what lets managarr ingest one shape from every service.
func (s *Service) Enrich(_ context.Context, body []byte) ([]sidecar.Event, error) {
	var hook struct {
		EventType string `json:"eventType"`
		Release   struct {
			ReleaseTitle string `json:"releaseTitle"`
		} `json:"release"`
	}
	if err := json.Unmarshal(body, &hook); err != nil {
		return nil, fmt.Errorf("prowlarrsvc: decode notification: %w", err)
	}
	if hook.EventType == "" {
		return nil, fmt.Errorf("prowlarrsvc: notification has no eventType")
	}

	return []sidecar.Event{{
		Source:  Source,
		Kind:    KindWebhook,
		Ref:     hook.Release.ReleaseTitle,
		Payload: json.RawMessage(body),
	}}, nil
}

// Routes serves the indexer table.
//
// Not events — a lookup table, and one managarr needs whole: a history record
// carries an indexerId and nothing else, so without this every record is
// attributed to a number.
func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+IndexersPath, func(w http.ResponseWriter, r *http.Request) {
		indexers, err := s.client.Indexers(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if indexers == nil {
			indexers = []prowlarr.Indexer{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"indexers": indexers})
	})
}
