package reconcile

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// syncQueue records what the instance is currently waiting on.
//
// The queue is the only place a download's problem is stated in the *arr's own
// terms. qBittorrent will happily report a torrent as complete while Sonarr
// sits on it refusing to import — "Found archive file, might need to be
// extracted", "One or more episodes expected in this release were not imported"
// — and that sentence exists nowhere else.
//
// A queue is a snapshot of right now, so this is the one part of the *arr that
// a poll can genuinely miss: an item that appears and imports between two
// passes is never seen. It is recorded rather than derived because the
// alternative is not recording it at all.
func (a *Arr) syncQueue(ctx context.Context, ix *arrIndex) (int, error) {
	items, err := a.client.Queue(ctx, a.pageSize)
	if err != nil {
		return 0, err
	}

	observedAt := time.Now().UTC()
	rows := make([]model.QueueItem, 0, len(items))

	for _, q := range items {
		hash, err := a.lookups.Torrent(ctx, q.DownloadID)
		if err != nil {
			return 0, err
		}
		release, err := a.lookups.Release(ctx, q.Title)
		if err != nil {
			return 0, err
		}

		rows = append(rows, model.QueueItem{
			ServiceName:     a.serviceName,
			ExternalID:      strconv.FormatInt(q.ID, 10),
			ContentID:       ix.contentFor(0, q.AlbumID, q.SeriesID, q.MovieID, q.ArtistID),
			ReleaseID:       release,
			TorrentInfohash: hash,
			Status:          strPtr(q.Status),
			TrackedStatus:   strPtr(trackedStatus(q.TrackedDownloadStatus, q.TrackedDownloadState)),
			ErrorMessage:    strPtr(queueMessage(q.ErrorMessage, q.StatusMessages)),
			SizeBytes:       int64Ptr(int64(q.Size)),
			SizeLeftBytes:   int64Ptr(int64(q.Sizeleft)),
			EstimatedAt:     q.EstimatedCompletion,
			ObservedAt:      observedAt,
		})
	}

	if err := dao.Upsert(ctx, a.dao, rows,
		[]string{"service_name", "external_id"},
		[]string{"content_id", "release_id", "torrent_infohash", "status",
			"tracked_status", "error_message", "size_bytes", "size_left_bytes",
			"estimated_at", "observed_at"},
	); err != nil {
		return 0, fmt.Errorf("%s: queue: %w", a.serviceName, err)
	}
	return len(rows), nil
}

// trackedStatus joins the two halves of the *arr's tracking state.
//
// They are separate fields answering different questions — status is how bad it
// is (ok / warning / error), state is where it is (downloading / importPending
// / failedPending) — and either alone is ambiguous.
func trackedStatus(status, state string) string {
	switch {
	case status == "":
		return state
	case state == "":
		return status
	default:
		return status + "/" + state
	}
}

// queueMessage picks the most useful explanation available.
//
// errorMessage is set for hard failures; statusMessages carries the softer
// "why is this not importing" text, which is the common case and the one worth
// reading. Both are folded into one column because a queue item has at most one
// reason anybody cares about.
func queueMessage(errorMessage string, status []struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
},
) string {
	if errorMessage != "" {
		return errorMessage
	}
	var parts []string
	for _, s := range status {
		parts = append(parts, s.Messages...)
	}
	return strings.Join(parts, "; ")
}

// syncBlocklist records releases the instance has refused to take again.
//
// The difference between a grab that quietly failed and one that was judged bad
// enough to never retry. Without it, a release that failed five times looks
// like five unrelated failures.
func (a *Arr) syncBlocklist(ctx context.Context, ix *arrIndex) (int, error) {
	items, err := a.client.Blocklist(ctx, a.pageSize)
	if err != nil {
		return 0, err
	}

	rows := make([]model.BlocklistEntry, 0, len(items))
	for _, b := range items {
		hash, err := a.lookups.Torrent(ctx, b.DownloadID)
		if err != nil {
			return 0, err
		}
		release, err := a.lookups.Release(ctx, b.SourceTitle)
		if err != nil {
			return 0, err
		}
		indexer, err := a.lookups.Indexer(ctx, b.Indexer)
		if err != nil {
			return 0, err
		}

		rows = append(rows, model.BlocklistEntry{
			ServiceName:     a.serviceName,
			ExternalID:      strconv.FormatInt(b.ID, 10),
			ReleaseID:       release,
			ContentID:       ix.contentFor(0, b.AlbumID, b.SeriesID, b.MovieID, b.ArtistID),
			TorrentInfohash: hash,
			Reason:          strPtr(b.Message),
			IndexerName:     indexer,
			BlockedAt:       timePtr(b.Date),
		})
	}

	if err := dao.Upsert(ctx, a.dao, rows,
		[]string{"service_name", "external_id"},
		[]string{"release_id", "content_id", "torrent_infohash", "reason",
			"indexer_name", "blocked_at"},
	); err != nil {
		return 0, fmt.Errorf("%s: blocklist: %w", a.serviceName, err)
	}
	return len(rows), nil
}
