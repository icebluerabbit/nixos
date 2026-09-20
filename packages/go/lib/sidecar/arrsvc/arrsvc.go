// Package arrsvc implements the sidecar contract over a Servarr instance.
//
// Sonarr, Radarr and Lidarr ship the same API under different nouns, and
// lib/clients/arr already treats them as one. This is the sidecar half of that:
// three binaries — managarr-sonarr, managarr-radarr, managarr-lidarr — differ
// only by the Kind they are built with.
//
// What it adds over managarr calling the *arr directly:
//
//   - A since-cursor. /history takes page, pageSize and a sort, and nothing
//     else; without a cursor every reconcile re-walks the whole history, which
//     here is 28,000 records an hour, forever.
//   - Webhook enrichment. A Grab notification names the release and the
//     download id but not what the download client actually called it, and the
//     queue record that knows is gone minutes later.
//   - The API key stops crossing the VLAN. The sidecar runs inside the *arr's
//     own container and talks to 127.0.0.1, so the key lives only where the
//     service that owns it already has it.
package arrsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"homelab/clients/arr"
	"homelab/sidecar"
)

// Event kinds this service emits.
const (
	// KindHistory is one record of the *arr's own history.
	KindHistory = "history"
	// KindWebhook is one notification, enriched.
	KindWebhook = "webhook"
)

// Service wraps one Servarr instance.
type Service struct {
	client   *arr.Client
	upstream string
}

// New builds the service. upstream is recorded for /info only.
func New(c *arr.Client, upstream string) *Service {
	return &Service{client: c, upstream: upstream}
}

// Info describes this sidecar. Touches nothing: it has to answer when the
// upstream does not, which is when it is most worth asking.
func (s *Service) Info() sidecar.Info {
	kind := string(s.client.Kind())
	return sidecar.Info{
		Sidecar:  "managarr-" + kind,
		Source:   kind,
		Upstream: s.upstream,
		Capabilities: []string{
			sidecar.CapChanges,
			sidecar.CapEvents,
		},
	}
}

// Changes returns history records after cursor, oldest first.
//
// The cursor is a decimal offset. The *arr offers no keyset filter — there is
// a /history/since?date= endpoint, but it is unpaged and returns every record
// from that date in one response, which on a first walk is the entire history.
// Ascending order is what makes an offset usable: history is appended, so a new
// record lands at the end and cannot shift a page already read.
func (s *Service) Changes(ctx context.Context, cursor string, limit int) ([]sidecar.Event, error) {
	offset, err := parseOffset(cursor)
	if err != nil {
		return nil, err
	}

	records, err := s.records(ctx, offset, limit)
	if err != nil {
		return nil, err
	}

	out := make([]sidecar.Event, 0, len(records))
	for i, r := range records {
		payload, err := json.Marshal(r)
		if err != nil {
			return nil, fmt.Errorf("arrsvc: encode history record: %w", err)
		}
		out = append(out, sidecar.Event{
			Source:     string(s.client.Kind()),
			Kind:       KindHistory,
			OccurredAt: r.Date,
			// The download id, uppercase, is what ties a history record to
			// a torrent. Empty on events with no download behind them —
			// a rename, a manual file delete — which is normal.
			Ref:     r.DownloadID,
			Payload: payload,
			Cursor:  strconv.Itoa(offset + i + 1),
		})
	}
	return out, nil
}

// records reads limit records starting at offset.
//
// Loops because an offset need not land on a page boundary: a consumer is free
// to change its limit between calls, and returning a short page then would end
// its walk early — the walk stops on a short page, which has to mean "the
// history ended" and nothing else.
func (s *Service) records(ctx context.Context, offset, limit int) ([]arr.HistoryRecord, error) {
	out := make([]arr.HistoryRecord, 0, limit)

	for len(out) < limit {
		want := offset + len(out)
		page := want/limit + 1
		skip := want % limit

		res, err := s.client.HistoryPageAt(ctx, page, limit)
		if err != nil {
			return nil, err
		}
		if skip >= len(res.Records) {
			return out, nil
		}

		take := res.Records[skip:]
		if room := limit - len(out); len(take) > room {
			take = take[:room]
		}
		out = append(out, take...)

		// Past the end the *arr reports, or the page came back short —
		// either way there is nothing further to read.
		if want+len(take) >= res.TotalRecords || len(res.Records) < limit {
			return out, nil
		}
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
		return 0, fmt.Errorf("arrsvc: bad cursor %q", cursor)
	}
	return n, nil
}

// enriched is what a notification becomes once the sidecar has filled it in.
type enriched struct {
	// Webhook is the notification exactly as the *arr sent it.
	Webhook arr.Webhook `json:"webhook"`
	// Queue is the download client's record for this grab, when it was still
	// there to find. The notification says what was grabbed; this says what
	// the client called it and how big it actually is.
	Queue *arr.QueueItem `json:"queue,omitempty"`
}

// Enrich turns one notification into events.
//
// Several events from one body is normal and deliberate: an ImportComplete for
// a season pack carries a file per episode, and collapsing them would discard
// exactly the per-file detail managarr records.
func (s *Service) Enrich(ctx context.Context, body []byte) ([]sidecar.Event, error) {
	var hook arr.Webhook
	if err := json.Unmarshal(body, &hook); err != nil {
		return nil, fmt.Errorf("arrsvc: decode notification: %w", err)
	}
	if hook.EventType == "" {
		return nil, fmt.Errorf("arrsvc: notification has no eventType")
	}

	payload := enriched{Webhook: hook}

	// Only on a grab, and only when there is a download id to match. This is
	// the whole reason enrichment happens here rather than in managarr: the
	// queue record exists for the minutes between the grab and the import,
	// and an hourly pass is not there to see it.
	if hook.Event() == arr.WebhookGrab && hook.DownloadID != "" {
		if item, err := s.queueItem(ctx, hook.DownloadID); err == nil {
			payload.Queue = item
		} else {
			// Not fatal. The event is still worth relaying without the
			// queue detail, and a failure here would drop it entirely.
			payload.Queue = nil
		}
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("arrsvc: encode notification: %w", err)
	}

	return []sidecar.Event{{
		Source:  string(s.client.Kind()),
		Kind:    KindWebhook,
		Ref:     hook.DownloadID,
		Payload: encoded,
	}}, nil
}

// queueItem finds the download client's record for one download id.
func (s *Service) queueItem(ctx context.Context, downloadID string) (*arr.QueueItem, error) {
	items, err := s.client.Queue(ctx, 0)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if strings.EqualFold(items[i].DownloadID, downloadID) {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("arrsvc: %s not in queue", downloadID)
}
