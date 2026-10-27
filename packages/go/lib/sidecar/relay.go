package sidecar

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"homelab/httpx"
)

// Relay hands events to managarr.
//
// No persistence, by design. If managarr is unreachable the event is dropped
// and counted, because every upstream a sidecar wraps keeps its own history and
// the hourly reconcile backfills from it — a lost webhook costs latency, not
// data. Buying durability here would mean a queue, a state directory on
// containers that are deliberately ephemeral, and two sources of truth to
// reconcile. Not worth it for latency.
//
// The one upstream where that reasoning does NOT hold is qBittorrent, which
// keeps no history: a torrent removed between passes leaves nothing behind.
// That is worth solving when a qBittorrent sidecar exists, and solving there
// rather than here.
type Relay struct {
	http    *httpx.Client
	path    string
	logger  *slog.Logger
	metrics *Metrics
}

// NewRelay builds a relay pointed at managarr's ingest endpoint.
func NewRelay(cfg httpx.Config, path string, logger *slog.Logger, m *Metrics) (*Relay, error) {
	c, err := httpx.New(cfg)
	if err != nil {
		return nil, err
	}
	if path == "" {
		path = DefaultRelayPath
	}
	return &Relay{http: c, path: path, logger: logger, metrics: m}, nil
}

// DefaultRelayPath is managarr's ingest route.
const DefaultRelayPath = "/ingest/sidecar"

// Send hands events over, one request for the batch.
//
// Batched because a season pack import is one webhook and forty events, and
// forty requests for one upstream notification would make the sidecar the
// noisiest thing on the VLAN.
//
// A partial failure is not distinguished: managarr either accepted the batch or
// did not. Splitting it to find which event it disliked would multiply requests
// against a consumer that is already refusing them.
func (r *Relay) Send(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}

	body := struct {
		Events []Event `json:"events"`
	}{Events: events}

	if err := r.http.Do(ctx, http.MethodPost, r.path, body, nil); err != nil {
		r.metrics.failed.WithLabelValues(StageRelay).Add(float64(len(events)))
		// Logged at warn, not error: managarr being restarted is routine,
		// and the reconcile recovers what was missed. An error here would
		// page for something that fixes itself within the hour.
		r.logger.WarnContext(ctx, "relay to managarr failed; reconcile will backfill",
			"events", len(events), "err", err)
		return fmt.Errorf("sidecar: relay %d events: %w", len(events), err)
	}

	for _, e := range events {
		r.metrics.relayed.WithLabelValues(e.Kind).Inc()
	}
	return nil
}
