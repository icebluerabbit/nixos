package ingest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// prowlarrHook is Prowlarr's notification body.
//
// Its event surface is thin: a grab it proxied, plus health and version events.
// A grab carries the release and the indexer but never a downloadId — Prowlarr
// hands the .torrent back and the *arr talks to the client, so it never learns
// what the torrent became.
type prowlarrHook struct {
	EventType string `json:"eventType"`
	Release   struct {
		ReleaseTitle string `json:"releaseTitle"`
		Indexer      string `json:"indexer"`
		Size         int64  `json:"size"`
	} `json:"release"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

// prowlarr records a proxied grab or a health flare.
//
// The grab leg writes the release and its indexer and nothing more, because
// that is all Prowlarr knows. The history pass records the same grab with its
// own id; this exists so the release row is present the moment it is grabbed
// rather than up to an hour later, which is what lets the autobrr and *arr
// sides meet on it in between.
func (i *Ingest) prowlarr(ctx context.Context, body []byte) error {
	if i.cfg.Prowlarr == "" {
		return nil
	}
	var hook prowlarrHook
	if err := decode(body, &hook); err != nil {
		return fmt.Errorf("prowlarr: decode: %w", err)
	}

	switch strings.ToLower(hook.EventType) {
	case arrEventGrab:
		if _, err := i.lookups.Release(ctx, hook.Release.ReleaseTitle); err != nil {
			return err
		}
		_, err := i.lookups.Indexer(ctx, hook.Release.Indexer)
		return err

	case arrEventHealthIssue, arrEventHealthRestored:
		if hook.Type == "" {
			return nil
		}
		now := time.Now().UTC()
		source := hook.Level
		if source == "" {
			source = "webhook"
		}
		return dao.Upsert(ctx, i.dao, []model.HealthEvent{{
			ServiceName: i.cfg.Prowlarr,
			Type:        hook.Type,
			Source:      source,
			Message:     strPtr(hook.Message),
			FirstSeenAt: now,
			LastSeenAt:  now,
		}}, []string{"service_name", "type", "source"},
			[]string{"message", "last_seen_at"})
	}
	return nil
}
