package ingest

import (
	"context"
	"fmt"
	"time"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// qbtHook is what qBittorrent's AutoRun scripts post.
//
// qBittorrent has no HTTP webhook; these are external programs run on add and
// on completion, so every field arrives as a shell argument turned into a JSON
// string — including size.
type qbtHook struct {
	Event    string `json:"event"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Size     string `json:"size"`
	Infohash string `json:"infohash"`
}

// qBittorrent's two AutoRun events.
const (
	qbtAdded    = "added"
	qbtFinished = "finished"
)

// qbittorrent records a client-side torrent event.
//
// The hourly snapshot is the authority on what the client holds; this covers
// the window it cannot see. A torrent added and removed between two passes —
// a failed grab, a cross-seed that did not match, a manual test — leaves no
// trace in a snapshot taken an hour later, and the add event is the only
// evidence it was ever there.
//
// It is also where added_at and completed_at come from precisely rather than
// approximately: qBittorrent reports both in its API, but only for torrents it
// still has.
func (i *Ingest) qbittorrent(ctx context.Context, body []byte) error {
	var hook qbtHook
	if err := decode(body, &hook); err != nil {
		return fmt.Errorf("qbittorrent: decode: %w", err)
	}
	if hook.Infohash == "" {
		return nil
	}
	if i.cfg.QBittorrent == "" {
		return nil
	}

	hash, err := i.lookups.Torrent(ctx, hook.Infohash)
	if err != nil {
		return err
	}
	if hash == nil {
		return nil
	}
	if _, err := i.lookups.Client(ctx, i.cfg.QBittorrent, model.ClientQBittorrent); err != nil {
		return err
	}

	// The name is the release as the client sees it, which for a torrent that
	// was never renamed is the release title the announce used.
	releaseID, err := i.lookups.Release(ctx, hook.Name)
	if err != nil {
		return err
	}
	if err := dao.UpsertGuarded(ctx, i.dao, []model.Torrent{{
		Infohash:       *hash,
		ReleaseID:      releaseID,
		TotalSizeBytes: int64Ptr(parseBytes(hook.Size)),
	}}, []string{"infohash"},
		"torrents.release_id IS NULL",
		[]string{"release_id", "total_size_bytes"}); err != nil {
		return err
	}

	now := time.Now().UTC()
	row := model.ClientTorrent{
		ClientName:      i.cfg.QBittorrent,
		TorrentInfohash: *hash,
		Category:        strPtr(hook.Category),
	}

	// Each event sets its own timestamp and neither touches the other's. An
	// added event arriving after a finished one — which happens on a recheck —
	// must not blank the completion.
	var updates []string
	switch hook.Event {
	case qbtAdded:
		row.AddedAt = &now
		updates = []string{"category", "added_at"}
	case qbtFinished:
		row.CompletedAt = &now
		updates = []string{"category", "completed_at"}
	default:
		return nil
	}

	return dao.Upsert(ctx, i.dao, []model.ClientTorrent{row},
		[]string{"client_name", "torrent_infohash"}, updates)
}
