package reconcile

import (
	"context"
	"fmt"
	"time"

	"homelab/clients/arr"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// syncFiles records every file the instance has imported.
//
// This is the authoritative statement of what is on disk and what content it
// belongs to. Nothing else in the stack can say that: qBittorrent knows a path,
// Bazarr knows a subtitle sits beside one, beets knows a tag — only the *arr
// knows the file at /media/tv/Show/S01E02.mkv is episode two.
//
// Provenance is NOT written here. Which torrent produced a file is a historical
// fact held by the import event, so it is filled by the history step; writing a
// NULL here would blank it on every pass.
func (a *Arr) syncFiles(ctx context.Context, ix *arrIndex) (int, error) {
	parentIDs := make([]int64, 0, len(ix.parent))
	for id := range ix.parent {
		parentIDs = append(parentIDs, id)
	}

	files, err := a.client.Files(ctx, parentIDs)
	if err != nil {
		return 0, err
	}
	known, err := a.knownFiles(ctx)
	if err != nil {
		return 0, err
	}

	observedAt := time.Now().UTC()

	// Referenced tables first: qualities, languages and custom formats are all
	// foreign keys from the rows below, and a file naming a quality nothing has
	// declared yet would fail the insert.
	if err := a.writeQualities(ctx, files); err != nil {
		return 0, err
	}
	languages, err := a.writeLanguages(ctx, files)
	if err != nil {
		return 0, err
	}
	formats, err := a.writeCustomFormats(ctx, files)
	if err != nil {
		return 0, err
	}

	rows := make([]model.LibraryFile, 0, len(files))
	var (
		infos    []model.MediaInfo
		langs    []model.LibraryFileLanguage
		fmtLinks []model.LibraryFileCustomFormat
	)

	for _, f := range files {
		id, ok := known[f.Path]
		if !ok {
			id = model.NewID()
			known[f.Path] = id
		}

		inode, size, err := statInto(ctx, a.dao, f.Path, observedAt)
		if err != nil {
			return 0, err
		}

		rows = append(rows, model.LibraryFile{
			Base:              model.Base{ID: id},
			Path:              f.Path,
			ContentID:         a.contentForFile(ix, f),
			ServiceName:       &a.serviceName,
			FileDataInode:     inode,
			FileDataSizeBytes: size,
			ImportedAt:        timePtr(f.DateAdded),
			QualityName:       strPtr(f.Quality.Quality.Name),
			QualityRevision:   intPtr(f.Quality.Revision.Version),
			CustomFormatScore: intPtr(f.CustomFormatScore),
			ReleaseGroup:      strPtr(f.ReleaseGroup),
			SceneName:         strPtr(f.SceneName),
			RelativePath:      strPtr(f.RelativePath),
			SeasonNumber:      intPtr(f.SeasonNumber),
			ArrFileID:         intPtr(int(f.ID)),
		})

		if f.MediaInfo != nil {
			infos = append(infos, mediaInfoRow(id, *f.MediaInfo))
		}
		for _, l := range f.Languages {
			if _, ok := languages[l.Name]; !ok {
				continue
			}
			langs = append(langs, model.LibraryFileLanguage{
				LibraryFileID: id, LanguageName: l.Name,
			})
		}
		for _, cf := range f.CustomFormats {
			if _, ok := formats[cf.Name]; !ok {
				continue
			}
			fmtLinks = append(fmtLinks, model.LibraryFileCustomFormat{
				LibraryFileID: id,
				ServiceName:   a.serviceName,
				FormatName:    cf.Name,
			})
		}
	}

	// removed_at and removal_reason are in the update list with nil values, so
	// a file the *arr still lists has them cleared. That is what makes the
	// column mean "gone as of the last look" rather than "was gone once" — a
	// path the *arr dropped and later re-imported would otherwise stay marked
	// removed forever. This pass runs before the history pass, which then
	// re-marks anything genuinely deleted.
	if err := dao.Upsert(ctx, a.dao, rows, []string{"path"}, []string{
		"content_id", "service_name", "file_data_inode", "file_data_size_bytes",
		"imported_at", "quality_name", "quality_revision", "custom_format_score",
		"release_group", "scene_name", "relative_path", "season_number",
		"arr_file_id", "removed_at", "removal_reason",
	}); err != nil {
		return 0, fmt.Errorf("%s: library files: %w", a.serviceName, err)
	}

	if err := a.writeFileDetail(ctx, infos, langs, fmtLinks); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// contentForFile resolves which content a file holds.
//
// Radarr's file names its movie and there is nothing below a movie. Sonarr and
// Lidarr are the other way round: the file record does not name the episode or
// track, the episode and track name the file, which is why the catalog step
// builds that direction.
func (a *Arr) contentForFile(ix *arrIndex, f arr.MediaFile) *model.ID {
	if a.client.Kind() == arr.Radarr {
		if id, ok := ix.parent[f.MovieID]; ok {
			return &id
		}
		return nil
	}
	if id, ok := ix.file[f.ID]; ok {
		return &id
	}
	return nil
}

// writeFileDetail persists the per-file detail tables.
func (a *Arr) writeFileDetail(
	ctx context.Context,
	infos []model.MediaInfo,
	langs []model.LibraryFileLanguage,
	fmtLinks []model.LibraryFileCustomFormat,
) error {
	if err := dao.UpsertAll(ctx, a.dao, infos, []string{"library_file_id"}); err != nil {
		return fmt.Errorf("%s: media info: %w", a.serviceName, err)
	}
	if err := dao.Insert(ctx, a.dao, langs,
		[]string{"library_file_id", "language_name"}); err != nil {
		return fmt.Errorf("%s: file languages: %w", a.serviceName, err)
	}
	if err := dao.Insert(ctx, a.dao, fmtLinks,
		[]string{"library_file_id", "service_name", "format_name"}); err != nil {
		return fmt.Errorf("%s: file custom formats: %w", a.serviceName, err)
	}
	return nil
}

// writeQualities records the qualities these files use.
//
// Source and resolution come from the file records rather than from a settings
// endpoint because that is where they are stated per-file — and a quality
// definition the library no longer uses is not worth a second request.
func (a *Arr) writeQualities(ctx context.Context, files []arr.MediaFile) error {
	seen := map[string]model.Quality{}
	for _, f := range files {
		q := f.Quality.Quality
		if q.Name == "" {
			continue
		}
		seen[q.Name] = model.Quality{
			Name:       q.Name,
			Source:     strPtr(q.Source),
			Resolution: intPtr(q.Resolution),
		}
	}
	rows := make([]model.Quality, 0, len(seen))
	for _, q := range seen {
		rows = append(rows, q)
	}
	if err := dao.Upsert(ctx, a.dao, rows, []string{"name"},
		[]string{"source", "resolution"}); err != nil {
		return fmt.Errorf("%s: qualities: %w", a.serviceName, err)
	}
	return nil
}

// writeLanguages records the languages these files carry and returns the set.
func (a *Arr) writeLanguages(ctx context.Context, files []arr.MediaFile) (map[string]struct{}, error) {
	seen := map[string]struct{}{}
	var rows []model.Language
	for _, f := range files {
		for _, l := range f.Languages {
			if l.Name == "" {
				continue
			}
			if _, ok := seen[l.Name]; ok {
				continue
			}
			seen[l.Name] = struct{}{}
			rows = append(rows, model.Language{Name: l.Name})
		}
	}
	if err := dao.Insert(ctx, a.dao, rows, []string{"name"}); err != nil {
		return nil, fmt.Errorf("%s: languages: %w", a.serviceName, err)
	}
	return seen, nil
}

// writeCustomFormats records this instance's custom formats and returns the set.
//
// Per-instance by primary key, which is not incidental: the same format name in
// Sonarr and Radarr is two separate definitions with unrelated ids and possibly
// different scores.
func (a *Arr) writeCustomFormats(ctx context.Context, files []arr.MediaFile) (map[string]struct{}, error) {
	seen := map[string]struct{}{}
	var rows []model.CustomFormat
	for _, f := range files {
		for _, cf := range f.CustomFormats {
			if cf.Name == "" {
				continue
			}
			if _, ok := seen[cf.Name]; ok {
				continue
			}
			seen[cf.Name] = struct{}{}
			rows = append(rows, model.CustomFormat{
				ServiceName: a.serviceName, Name: cf.Name,
			})
		}
	}
	if err := dao.Insert(ctx, a.dao, rows,
		[]string{"service_name", "name"}); err != nil {
		return nil, fmt.Errorf("%s: custom formats: %w", a.serviceName, err)
	}
	return seen, nil
}

// knownFiles loads the ids already assigned to this instance's files.
func (a *Arr) knownFiles(ctx context.Context) (map[string]model.ID, error) {
	lf := a.dao.Q().LibraryFile
	rows, err := lf.WithContext(ctx).
		Select(lf.ID, lf.Path).
		Where(lf.ServiceName.Eq(a.serviceName)).
		Find()
	if err != nil {
		return nil, fmt.Errorf("%s: known files: %w", a.serviceName, err)
	}
	out := make(map[string]model.ID, len(rows))
	for _, r := range rows {
		out[r.Path] = r.ID
	}
	return out, nil
}

// mediaInfoRow converts a probe result.
//
// Bitrates and sample rates are text rather than numbers because Lidarr sends
// them preformatted — "1411 kbps" — while Sonarr sends integers. Storing the
// string keeps both without inventing a unit neither stated.
func mediaInfoRow(fileID model.ID, mi arr.MediaInfo) model.MediaInfo {
	return model.MediaInfo{
		LibraryFileID:         fileID,
		VideoCodec:            strPtr(mi.VideoCodec),
		VideoBitrate:          strPtr(mi.VideoBitrate.String()),
		VideoBitDepth:         intPtr(mi.VideoBitDepth),
		VideoFps:              floatPtr(mi.VideoFps),
		VideoDynamicRange:     strPtr(mi.VideoDynamicRange),
		VideoDynamicRangeType: strPtr(mi.VideoDynamicRangeType),
		Resolution:            strPtr(mi.Resolution),
		RunTime:               strPtr(mi.RunTime),
		ScanType:              strPtr(mi.ScanType),
		AudioCodec:            strPtr(mi.AudioCodec),
		AudioBitrate:          strPtr(mi.AudioBitrate.String()),
		AudioChannels:         floatPtr(mi.AudioChannels),
		AudioLanguages:        strPtr(mi.AudioLanguages),
		AudioStreamCount:      intPtr(mi.AudioStreamCount),
		AudioBits:             strPtr(mi.AudioBits.String()),
		AudioSampleRate:       strPtr(mi.AudioSampleRate.String()),
		Subtitles:             strPtr(mi.Subtitles),
	}
}
