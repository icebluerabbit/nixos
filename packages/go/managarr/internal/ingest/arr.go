package ingest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// arrWebhook is the Servarr notification body, over the fields worth keeping.
//
// One struct for all three apps and every event type. The family sends a union
// — a Grab has a release and no files, an ImportComplete has files and a
// release, a HealthIssue has neither — so absence is normal and every section
// is optional.
type arrWebhook struct {
	EventType string `json:"eventType"`
	// DownloadID is the infohash, uppercase. Present on grab and import.
	DownloadID string `json:"downloadId"`
	Release    struct {
		ReleaseTitle string `json:"releaseTitle"`
		Indexer      string `json:"indexer"`
		Size         int64  `json:"size"`
		ReleaseGroup string `json:"releaseGroup"`
	} `json:"release"`

	// Import events. Sonarr sends episodeFile (singular) on Download and
	// episodeFiles (plural) on ImportComplete; Radarr sends movieFile; Lidarr
	// sends trackFiles. All of them are the same shape underneath.
	EpisodeFile  *arrFile  `json:"episodeFile"`
	EpisodeFiles []arrFile `json:"episodeFiles"`
	MovieFile    *arrFile  `json:"movieFile"`
	TrackFiles   []arrFile `json:"trackFiles"`

	// Health events.
	Level   string `json:"level"`
	Message string `json:"message"`
	Type    string `json:"type"`
	WikiURL string `json:"wikiUrl"`

	InstanceName   string `json:"instanceName"`
	ApplicationURL string `json:"applicationUrl"`
}

// arrFile is one imported file as a notification reports it.
type arrFile struct {
	ID        int64  `json:"id"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SceneName string `json:"sceneName"`
}

// Event types the family sends. Compared case-insensitively: Sonarr sends
// "Grab" while Lidarr has sent "grab", and a mismatch here is silent.
const (
	arrEventGrab           = "grab"
	arrEventDownload       = "download"
	arrEventImportComplete = "importcomplete"
	arrEventHealthIssue    = "health"
	arrEventHealthRestored = "healthrestored"
)

// arr records one Servarr notification.
//
// Two things are written and neither duplicates the hourly pass:
//
//   - On a grab, the release-to-torrent link. The *arr's history carries the
//     same downloadId, but the pass runs hourly and cross-seed asks about a
//     torrent within seconds of it landing.
//   - On an import, the file's provenance. Same reasoning: this is the moment
//     the mapping from torrent to library file becomes true.
//
// Health is recorded because the API reports only what is failing NOW, so a
// check that flared and cleared between two passes is invisible to it.
func (i *Ingest) arr(ctx context.Context, serviceName string, body []byte) error {
	if serviceName == "" {
		// The instance was not configured, so there is no Service row to hang
		// the event off and the foreign keys would fail.
		return nil
	}
	var hook arrWebhook
	if err := decode(body, &hook); err != nil {
		return fmt.Errorf("arr: decode: %w", err)
	}

	switch strings.ToLower(hook.EventType) {
	case arrEventGrab:
		return i.arrGrab(ctx, hook)
	case arrEventDownload, arrEventImportComplete:
		return i.arrImport(ctx, serviceName, hook)
	case arrEventHealthIssue, arrEventHealthRestored:
		return i.arrHealth(ctx, serviceName, hook)
	}
	return nil
}

// arrGrab records which release became which torrent.
func (i *Ingest) arrGrab(ctx context.Context, hook arrWebhook) error {
	if hook.DownloadID == "" || hook.Release.ReleaseTitle == "" {
		return nil
	}
	releaseID, err := i.lookups.Release(ctx, hook.Release.ReleaseTitle)
	if err != nil {
		return err
	}
	hash, err := i.lookups.Torrent(ctx, hook.DownloadID)
	if err != nil {
		return err
	}
	if hash == nil || releaseID == nil {
		return nil
	}
	if _, err := i.lookups.Indexer(ctx, hook.Release.Indexer); err != nil {
		return err
	}

	return dao.UpsertGuarded(ctx, i.dao, []model.Torrent{{
		Infohash:       *hash,
		ReleaseID:      releaseID,
		TotalSizeBytes: int64Ptr(hook.Release.Size),
	}}, []string{"infohash"},
		"torrents.release_id IS NULL",
		[]string{"release_id", "total_size_bytes"})
}

// arrImport records the torrent behind each imported file.
//
// Keyed on path, which is the only thing the event and the file record share.
// Guarded on NULL so a later rename or re-import cannot reattribute a file to
// a different torrent than the one that produced it.
func (i *Ingest) arrImport(ctx context.Context, serviceName string, hook arrWebhook) error {
	hash, err := i.lookups.Torrent(ctx, hook.DownloadID)
	if err != nil {
		return err
	}
	if hash == nil {
		// A manual import has no download behind it.
		return nil
	}

	paths := make([]string, 0, 4)
	for _, f := range hook.files() {
		if f.Path != "" {
			paths = append(paths, f.Path)
		}
	}
	if len(paths) == 0 {
		return nil
	}

	lf := i.dao.Q().LibraryFile
	res, err := lf.WithContext(ctx).
		Where(lf.Path.In(paths...), lf.TorrentInfohash.IsNull()).
		Update(lf.TorrentInfohash, *hash)
	if err != nil {
		return fmt.Errorf("arr: link import: %w", err)
	}
	i.logger.DebugContext(ctx, "import linked",
		"service", serviceName, "infohash", *hash, "files", res.RowsAffected)
	return nil
}

// files flattens whichever file collection this app sent.
func (h arrWebhook) files() []arrFile {
	var out []arrFile
	if h.EpisodeFile != nil {
		out = append(out, *h.EpisodeFile)
	}
	if h.MovieFile != nil {
		out = append(out, *h.MovieFile)
	}
	out = append(out, h.EpisodeFiles...)
	out = append(out, h.TrackFiles...)
	return out
}

// arrHealth records a health check flare.
//
// first_seen_at is written on insert and never updated, so a check that has
// been failing since Tuesday still says Tuesday on Friday.
func (i *Ingest) arrHealth(ctx context.Context, serviceName string, hook arrWebhook) error {
	if hook.Type == "" {
		return nil
	}
	now := time.Now().UTC()
	source := hook.Level
	if source == "" {
		// The notification's level is what the API calls source. An event
		// without one still needs a key, and the type alone is not unique.
		source = "webhook"
	}
	return dao.Upsert(ctx, i.dao, []model.HealthEvent{{
		ServiceName: serviceName,
		Type:        hook.Type,
		Source:      source,
		Message:     strPtr(hook.Message),
		FirstSeenAt: now,
		LastSeenAt:  now,
	}}, []string{"service_name", "type", "source"},
		[]string{"message", "last_seen_at"})
}
