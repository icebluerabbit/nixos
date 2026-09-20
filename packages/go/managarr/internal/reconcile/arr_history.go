package reconcile

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"homelab/clients/arr"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// syncHistory replays the instance's own history.
//
// Three things come out of it, and nothing else in the stack holds any of them:
//
//	grabbed   which release was taken for which content, through which indexer
//	imported  which torrent produced which file on disk
//	failed    which grabs never became a file, and why
//
// The middle one is the join that makes the whole model work. A file has a path
// and a torrent has a path, but they are different paths — the torrent's is
// where it was downloaded, the file's is where it was moved to — so the only
// honest link between them is the import event that performed the move.
func (a *Arr) syncHistory(ctx context.Context, ix *arrIndex, records []arr.HistoryRecord) (grabs, imports, failures, removed int, err error) {

	var (
		grabRows []model.Grab
		failRows []model.ImportFailure
		removals []removal
		// byHash groups imported paths by the torrent that produced them: one
		// torrent yields many files, and updating them together turns tens of
		// thousands of statements into a few hundred.
		byHash = map[string][]string{}
	)

	for _, r := range records {
		content := ix.contentFor(r.EpisodeID, r.AlbumID, r.SeriesID, r.MovieID, r.ArtistID)

		switch r.Kind() {
		case arr.EventGrabbed:
			row, err := a.grabRow(ctx, r, content)
			if err != nil {
				return 0, 0, 0, 0, err
			}
			grabRows = append(grabRows, row)

		case arr.EventImported:
			path := r.Data[arr.DataImportedPath]
			hash := strings.ToLower(strings.TrimSpace(r.DownloadID))
			if path == "" || hash == "" {
				// An import with no download behind it is a manual file drop,
				// and there is no torrent to attribute it to.
				continue
			}
			byHash[hash] = append(byHash[hash], path)

		case arr.EventFailed, arr.EventIgnored, arr.EventImportIncomplete:
			// Three outcomes, one table. All three are a grab that stopped
			// short of becoming a library file; the kind is what says whether
			// it broke, was abandoned, or half-worked.
			row, err := a.failureRow(ctx, r, content, failureKind(r.Kind()))
			if err != nil {
				return 0, 0, 0, 0, err
			}
			failRows = append(failRows, row)

		case arr.EventDeleted:
			// sourceTitle is the file's absolute path on a delete, which is an
			// exact join to library_files.path. Reason is the *arr's own enum:
			// Upgrade, MissingFromDisk or Manual.
			if r.SourceTitle == "" {
				continue
			}
			removals = append(removals, removal{
				Path:   r.SourceTitle,
				At:     r.Date.UTC(),
				Reason: r.Data[arr.DataReason],
			})
		}
	}

	if err := dao.Upsert(ctx, a.dao, grabRows,
		[]string{"service_name", "source_ref"},
		[]string{"content_id", "release_id", "torrent_infohash", "indexer_name",
			"quality_name", "quality_revision", "custom_format_score",
			"size_bytes", "indexer_flags", "release_group"},
	); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("%s: grabs: %w", a.serviceName, err)
	}

	if err := dao.Upsert(ctx, a.dao, failRows,
		[]string{"service_name", "source_ref"},
		[]string{"content_id", "torrent_infohash", "reason", "source_path", "kind"},
	); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("%s: import failures: %w", a.serviceName, err)
	}

	imports, err = a.linkImports(ctx, byHash)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	removed, err = a.markRemovals(ctx, removals)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return len(grabRows), imports, len(failRows), removed, nil
}

// grabRow builds one grab from a history record.
func (a *Arr) grabRow(ctx context.Context, r arr.HistoryRecord, content *model.ID) (model.Grab, error) {
	hash, err := a.lookups.Torrent(ctx, r.DownloadID)
	if err != nil {
		return model.Grab{}, err
	}
	indexer, err := a.lookups.Indexer(ctx, r.Data[arr.DataIndexer])
	if err != nil {
		return model.Grab{}, err
	}
	quality, err := a.lookups.Quality(ctx, r.Quality.Quality.Name)
	if err != nil {
		return model.Grab{}, err
	}
	release, err := a.lookups.Release(ctx, r.SourceTitle)
	if err != nil {
		return model.Grab{}, err
	}

	// size arrives as a decimal string of bytes. A value that will not parse is
	// dropped rather than guessed at.
	var size int64
	if s := r.Data[arr.DataSize]; s != "" {
		size, _ = strconv.ParseInt(s, 10, 64)
	}

	return model.Grab{
		ServiceName:       a.serviceName,
		SourceRef:         strconv.FormatInt(r.ID, 10),
		GrabbedAt:         r.Date.UTC(),
		ContentID:         content,
		ReleaseID:         release,
		TorrentInfohash:   hash,
		IndexerName:       indexer,
		QualityName:       quality,
		QualityRevision:   intPtr(r.Quality.Revision.Version),
		CustomFormatScore: intPtr(r.CustomFormatScore),
		SizeBytes:         int64Ptr(size),
		IndexerFlags:      strPtr(r.Data[arr.DataIndexerFlags]),
		ReleaseGroup:      strPtr(r.Data[arr.DataReleaseGroup]),
	}, nil
}

// failureRow builds one import failure from a history record.
func (a *Arr) failureRow(ctx context.Context, r arr.HistoryRecord, content *model.ID, kind string) (model.ImportFailure, error) {
	hash, err := a.lookups.Torrent(ctx, r.DownloadID)
	if err != nil {
		return model.ImportFailure{}, err
	}

	// Each outcome puts its explanation under a different key, and the apps
	// disagree even within one — an empty reason is the one thing that makes
	// any of these rows useless. Tried in order rather than switched on the
	// kind, because a build that moves the text is more likely than one that
	// renames the event.
	reason := firstNonEmpty(
		r.Data[arr.DataReason],
		r.Data[arr.DataMessage],
		// Lidarr serialises a partial import's complaints as a JSON array.
		r.Data[arr.DataStatusMessages],
	)

	// droppedPath is where the file was when the import was attempted, which is
	// the download directory rather than the library.
	path := r.Data[arr.DataDroppedPath]
	if path == "" {
		path = r.SourceTitle
	}

	return model.ImportFailure{
		ServiceName:     a.serviceName,
		SourceRef:       strconv.FormatInt(r.ID, 10),
		At:              r.Date.UTC(),
		ContentID:       content,
		TorrentInfohash: hash,
		Kind:            kind,
		Reason:          strPtr(reason),
		SourcePath:      strPtr(path),
	}, nil
}

// importChunk bounds an IN list. Postgres accepts far more, but the statement
// cache keys on the parameter count, so unbounded chunks would evict the plan
// on every differently-sized batch.
const importChunk = 500

// linkImports writes the file-to-torrent provenance.
//
// Keyed on path, which is the only thing the import event and the file record
// share. A path the *arr has since renamed will not match, and that is correct:
// the row on disk is no longer the row that was imported, and guessing at which
// file a stale path became would invent provenance rather than record it.
func (a *Arr) linkImports(ctx context.Context, byHash map[string][]string) (int, error) {
	var linked int
	for hash, paths := range byHash {
		if _, err := a.lookups.Torrent(ctx, hash); err != nil {
			return linked, err
		}
		for start := 0; start < len(paths); start += importChunk {
			end := min(start+importChunk, len(paths))

			lf := a.dao.Q().LibraryFile
			res, err := lf.WithContext(ctx).
				Where(lf.Path.In(paths[start:end]...)).
				Update(lf.TorrentInfohash, hash)
			if err != nil {
				return linked, fmt.Errorf("%s: link import %s: %w",
					a.serviceName, hash, err)
			}
			linked += int(res.RowsAffected)
		}
	}
	return linked, nil
}

// removal is one file the *arr stopped tracking.
type removal struct {
	Path   string
	At     time.Time
	Reason string
}

// reasonUpgrade is the *arr's word for "replaced by a better release".
//
// The other two values of the enum are MissingFromDisk and Manual. All three are
// recorded; only this one also sets superseded_at, because only this one says
// something arrived to take its place.
const reasonUpgrade = "upgrade"

// markRemovals records what the *arr dropped and why.
//
// This is the only source for it. A file the *arr no longer tracks simply stops
// appearing in its file list, so a snapshot alone cannot distinguish "upgraded",
// "deleted by hand" and "vanished off the disk" — it cannot even tell that the
// file was ever there. History states all three outright and keeps them
// forever, which is what makes this backfillable rather than something that had
// to have been watched live.
//
// Note what this does NOT claim: the bytes are usually still on disk. An
// imported file is a hardlink to the still-seeding torrent copy and the *arr
// only ever drops its own link, so this is the library's view of the file, not
// the filesystem's.
//
// Written as one statement per chunk rather than one per file: each row carries
// its own timestamp and reason, so a plain IN list cannot express it and a
// statement per deletion would be thousands on a backfill.
func (a *Arr) markRemovals(ctx context.Context, removals []removal) (int, error) {
	var marked int
	for start := 0; start < len(removals); start += importChunk {
		end := min(start+importChunk, len(removals))

		chunk := make([]model.LibraryFileRemoval, 0, end-start)
		for _, r := range removals[start:end] {
			chunk = append(chunk, model.LibraryFileRemoval{
				Path: r.Path,
				At:   r.At,
				// Lowercased here rather than in the statement: the comparison
				// against reasonUpgrade is what decides superseded_at, and the
				// *arr spells its enum in PascalCase.
				Reason: strings.ToLower(r.Reason),
			})
		}

		// Generated from an annotated statement — see _gen/querier.go. The
		// field API assigns one value per column, and this needs a different
		// value per ROW from a VALUES list plus a CASE that reads the stored
		// row while writing it.
		rows, err := a.dao.Q().LibraryFile.WithContext(ctx).
			MarkRemovals(chunk, reasonUpgrade)
		if err != nil {
			return marked, fmt.Errorf("%s: removals: %w", a.serviceName, err)
		}
		marked += int(rows)
	}
	return marked, nil
}

// applyRenames follows each file's path through every rename the *arr made.
//
// Not a record of renames — a correction to the row. library_files.path is the
// unique key and the join target for import provenance and for Bazarr's
// subtitles, so a path that has gone stale does not merely look wrong: the next
// file snapshot inserts a SECOND row at the new path, the old row keeps the
// infohash, the new one has none, and the provenance detaches with both rows
// looking perfectly valid.
//
// Applied in history order, which is oldest-first, so a file renamed twice is
// followed a to b to c rather than landing on whichever hop was seen last.
//
// A rename onto a path that already exists is skipped rather than forced. That
// is the shape of an older database where the snapshot already created the
// second row; merging them means choosing which of two rows' columns to keep,
// and guessing wrong silently reattributes a file. It is logged instead.
func (a *Arr) applyRenames(ctx context.Context, records []arr.HistoryRecord) (int, error) {
	var renamed int

	for _, r := range records {
		if r.Kind() != arr.EventRenamed {
			continue
		}
		// SourcePath is the file's old absolute path and Path its new one.
		// sourceTitle carries the same old path, and is the fallback for the
		// builds that send it and nothing else.
		from := r.Data[arr.DataSourcePath]
		if from == "" {
			from = r.SourceTitle
		}
		to := r.Data[arr.DataPath]
		if from == "" || to == "" || from == to {
			continue
		}

		lf := a.dao.Q().LibraryFile
		occupied, err := lf.WithContext(ctx).Where(lf.Path.Eq(to)).Count()
		if err != nil {
			return renamed, fmt.Errorf("%s: rename target %q: %w", a.serviceName, to, err)
		}
		if occupied > 0 {
			a.logger.DebugContext(ctx, "rename target already present",
				"service", a.serviceName, "from", from, "to", to)
			continue
		}

		res, err := lf.WithContext(ctx).
			Where(lf.Path.Eq(from)).
			Update(lf.Path, to)
		if err != nil {
			return renamed, fmt.Errorf("%s: rename %q: %w", a.serviceName, from, err)
		}
		renamed += int(res.RowsAffected)
	}
	return renamed, nil
}

// failureKind maps an event to the outcome it records.
//
// Defaults to failed rather than to an empty string: the column is NOT NULL,
// and a new event kind routed here by mistake is better recorded as a failure
// than as a row that cannot be written at all.
func failureKind(k arr.EventKind) string {
	switch k {
	case arr.EventIgnored:
		return model.ImportIgnored
	case arr.EventImportIncomplete:
		return model.ImportIncomplete
	default:
		return model.ImportFailed
	}
}

// firstNonEmpty returns the first value that says something.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
