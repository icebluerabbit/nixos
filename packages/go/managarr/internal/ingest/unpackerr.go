package ingest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// unpackerrHook is unpackerr's webhook payload.
//
// The `data` section differs per event — a queued event has no output path, a
// failure has an error and no files — so everything below the top level is
// optional.
type unpackerrHook struct {
	Path  string `json:"path"`
	App   string `json:"app"`
	Event string `json:"event"`
	Time  string `json:"time"`
	IDs   struct {
		DownloadID string `json:"downloadId"`
		Title      string `json:"title"`
	} `json:"ids"`
	Data struct {
		// Archives maps a containing directory to the archive files in it.
		Archives map[string][]string `json:"archives"`
		Files    []string            `json:"files"`
		Output   string              `json:"output"`
		// unpackerr renders this with Go's time.Time.String() rather than
		// RFC3339, which encoding/json refuses — see flexTime. A plain
		// time.Time here failed the whole notification on this one field.
		Start   flexTime `json:"start"`
		Elapsed string   `json:"elapsed"`
		Bytes   int64    `json:"bytes"`
		Error   string   `json:"error"`
	} `json:"data"`
}

// unpackerr records an extraction.
//
// Extraction is where the download side stops matching the library side.
// Everywhere else in this stack an imported file is a hardlink to the
// downloaded one, so they share an inode and the mapping is observable. An
// extracted file is a NEW inode written from an archive — no link, no shared
// identity, nothing on disk connecting the two. unpackerr is the only witness
// that the archive became that file, and if this event is missed the
// relationship is not recoverable by any later pass.
func (i *Ingest) unpackerr(ctx context.Context, body []byte) error {
	var hook unpackerrHook
	if err := decode(body, &hook); err != nil {
		return fmt.Errorf("unpackerr: decode: %w", err)
	}
	if hook.IDs.DownloadID == "" {
		// Without the infohash there is nothing to attribute the extraction
		// to, and extractions are keyed by it.
		return nil
	}

	hash, err := i.lookups.Torrent(ctx, hook.IDs.DownloadID)
	if err != nil || hash == nil {
		return err
	}

	archives := hook.archives()
	if len(archives) == 0 {
		// A queued or deleted event that names no archive. The status belongs
		// to an extraction, and there is no extraction to put it on.
		return nil
	}

	finished := extractionFinished(hook.Event)
	rows := make([]model.Extraction, 0, len(archives))
	for _, archive := range archives {
		row := model.Extraction{
			TorrentInfohash: *hash,
			ArchivePath:     archive,
			ExtractedPath:   strPtr(hook.Data.Output),
			Status:          strPtr(hook.Event),
			StartedAt:       timePtr(hook.Data.Start.Time),
		}
		if finished {
			now := time.Now().UTC()
			row.FinishedAt = &now
		}
		rows = append(rows, row)
	}

	return dao.Upsert(ctx, i.dao, rows,
		[]string{"torrent_infohash", "archive_path"},
		[]string{"extracted_path", "status", "started_at", "finished_at"})
}

// archives lists every archive path the event names.
//
// unpackerr reports them as a map of containing directory to member files, so
// the archive's own path is the two joined. Falls back to the event's path when
// the map is absent, which is what a failure event sends.
func (h unpackerrHook) archives() []string {
	var out []string
	for dir, files := range h.Data.Archives {
		for _, f := range files {
			if strings.HasPrefix(f, "/") {
				// Already absolute: some versions report full paths.
				out = append(out, f)
				continue
			}
			out = append(out, strings.TrimSuffix(dir, "/")+"/"+f)
		}
	}
	if len(out) == 0 && h.Path != "" {
		out = append(out, h.Path)
	}
	return out
}

// extractionFinished reports whether an event ends an extraction.
//
// unpackerr's vocabulary: queued, extracting, extracted, imported, deleted,
// extractfailed. Only the terminal ones stamp a finish time — marking
// "extracting" as finished would make every in-flight extraction look complete.
func extractionFinished(event string) bool {
	switch strings.ToLower(event) {
	case "extracted", "imported", "deleted", "extractfailed":
		return true
	default:
		return false
	}
}
