package ingest

import (
	"context"
	"fmt"
	"time"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// soularrHook is the event soularr's Vector sidecar emits.
//
// soularr has no webhook, so this is a journal line the sidecar recognised and
// re-encoded. The shape belongs to the remap in _notify.nix, not to soularr, so
// a change on either side has to be made on both.
type soularrHook struct {
	Source string `json:"source"`
	Event  string `json:"event"`
	Album  string `json:"album"`
	Artist string `json:"artist"`
	Count  string `json:"count"`
	// AlbumID is Lidarr's album id, and the only reason any of this is
	// storable. soularr prints it on exactly one of its log lines.
	AlbumID string `json:"album_id"`
}

// soularr records what soularr did about an album Lidarr wants.
//
// Only the events carrying a Lidarr album id are stored, which today is the
// failed-import denylist and nothing else. That is not a shortcut: soularr logs
// its other outcomes — search failed, download completed — with the album and
// artist as free text and no identifier at all, and resolving those to content
// would mean matching on title, which this model does not do anywhere.
//
// The consequence is worth stating plainly: soularr_events holds import
// failures, not the full picture of soularr's activity. The rest is visible in
// the journal and in Loki, and is deliberately not promoted to a fact here.
func (i *Ingest) soularr(ctx context.Context, body []byte) error {
	var hook soularrHook
	if err := decode(body, &hook); err != nil {
		return fmt.Errorf("soularr: decode: %w", err)
	}
	if hook.Event == "" || hook.AlbumID == "" {
		return nil
	}
	if i.cfg.Lidarr == "" {
		// No Lidarr instance configured, so there is no catalog to resolve the
		// album id against.
		return nil
	}

	// Lidarr's own id, namespaced exactly as the *arr catalog pass writes it.
	ce := i.dao.Q().ContentExternalID
	ext, err := ce.WithContext(ctx).
		Where(ce.Provider.Eq(i.cfg.Lidarr), ce.Value.Eq("album:"+hook.AlbumID)).
		First()
	if err != nil {
		// A miss is normal rather than an error: soularr can fail an import for
		// an album Lidarr has since stopped tracking, and the reconcile pass
		// may not have run since it was added.
		i.logger.DebugContext(ctx, "soularr event for unknown album",
			"album_id", hook.AlbumID, "event", hook.Event)
		return nil
	}

	// count is incremented rather than assigned. Each journal line is one
	// occurrence, and an album that has failed to import five times is a
	// different fact from one that failed once — which is the question this
	// table exists to answer.
	now := time.Now().UTC()
	one := 1
	return dao.UpsertCounter(ctx, i.dao, []model.SoularrEvent{{
		ContentID:  ext.ContentID,
		Event:      hook.Event,
		Count:      &one,
		LastSeenAt: now,
	}}, []string{"content_id", "event"}, "count", []string{"last_seen_at"})
}
