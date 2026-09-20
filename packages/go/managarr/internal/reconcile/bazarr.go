package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"homelab/clients/bazarr"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// Bazarr reconciles the subtitle half of the library.
//
// Two facts come from here and from nowhere else: which subtitle files exist,
// and which languages are still missing. The second cannot be observed on disk
// at all — an absent subtitle leaves nothing behind — so it is only ever true
// because Bazarr said so.
//
// Correlation is free. Bazarr stamps sonarrEpisodeId and radarrId on every
// record, which are exactly the ids the *arr pass wrote into
// content_external_ids, so subtitles join to content by shared identifier with
// no title matching anywhere in the path.
type Bazarr struct {
	client  *bazarr.Client
	dao     *dao.DAO
	lookups *dao.Lookups
	logger  *slog.Logger
	// serviceName is this Bazarr; sonarrName and radarrName are the instances
	// whose ids its records carry, and they are what its ids resolve through.
	serviceName string
	sonarrName  string
	radarrName  string
}

// NewBazarr builds the pass.
func NewBazarr(c *bazarr.Client, d *dao.DAO, l *dao.Lookups, logger *slog.Logger, serviceName, sonarrName, radarrName string) *Bazarr {
	return &Bazarr{
		client: c, dao: d, lookups: l, logger: logger,
		serviceName: serviceName, sonarrName: sonarrName, radarrName: radarrName,
	}
}

// BazarrStats reports what a run did.
type BazarrStats struct {
	Subtitles int
	Wanted    int
	Took      time.Duration
}

// Run performs one pass.
func (b *Bazarr) Run(ctx context.Context) (BazarrStats, error) {
	var st BazarrStats
	started := time.Now()
	b.lookups.Reset()

	version, err := b.client.Version(ctx)
	if err != nil {
		return st, err
	}
	now := time.Now().UTC()
	if err := dao.Upsert(ctx, b.dao, []model.Service{{
		Name:       b.serviceName,
		Kind:       "bazarr",
		Version:    strPtr(version),
		LastSeenAt: &now,
	}}, []string{"name"}, []string{"kind", "version", "last_seen_at"}); err != nil {
		return st, err
	}

	contents, err := b.contentIndex(ctx)
	if err != nil {
		return st, err
	}
	files, err := b.fileIndex(ctx)
	if err != nil {
		return st, err
	}

	// subs is keyed by subtitle path, which is what both the listing and the
	// history identify a subtitle by. Collected across all four sources before
	// writing, because the listing says what exists while the history says
	// where it came from, and the row needs both.
	subs := map[string]*model.SubtitleFile{}
	var wanted []model.SubtitleWanted

	if err := b.collectSeries(ctx, contents, files, subs, &wanted); err != nil {
		return st, err
	}
	if err := b.collectMovies(ctx, contents, files, subs, &wanted); err != nil {
		return st, err
	}
	if err := b.collectHistory(ctx, subs); err != nil {
		return st, err
	}
	if err := b.collectBlacklist(ctx, subs); err != nil {
		return st, err
	}

	st.Subtitles, st.Wanted, err = b.write(ctx, subs, wanted)
	if err != nil {
		return st, err
	}

	st.Took = time.Since(started)
	return st, nil
}

// collectSeries records the subtitles and gaps of every episode.
func (b *Bazarr) collectSeries(
	ctx context.Context,
	contents, files map[string]model.ID,
	subs map[string]*model.SubtitleFile,
	wanted *[]model.SubtitleWanted,
) error {
	series, err := b.client.SeriesList(ctx)
	if err != nil {
		return err
	}
	ids := make([]int64, 0, len(series))
	for _, s := range series {
		ids = append(ids, s.SonarrSeriesID)
	}

	episodes, err := b.client.Episodes(ctx, ids)
	if err != nil {
		return err
	}
	for _, e := range episodes {
		key := b.sonarrName + "\x00episode:" + strconv.FormatInt(e.SonarrEpisodeID, 10)
		content, ok := contents[key]
		var contentID *model.ID
		if ok {
			contentID = &content
		}
		b.collect(subs, wanted, contentID, files[e.Path], e.Subtitles, e.MissingSubtitles)
	}
	return nil
}

// collectMovies records the subtitles and gaps of every film.
func (b *Bazarr) collectMovies(
	ctx context.Context,
	contents, files map[string]model.ID,
	subs map[string]*model.SubtitleFile,
	wanted *[]model.SubtitleWanted,
) error {
	movies, err := b.client.Movies(ctx)
	if err != nil {
		return err
	}
	for _, m := range movies {
		key := b.radarrName + "\x00movie:" + strconv.FormatInt(m.RadarrID, 10)
		content, ok := contents[key]
		var contentID *model.ID
		if ok {
			contentID = &content
		}
		b.collect(subs, wanted, contentID, files[m.Path], m.Subtitles, m.MissingSubtitles)
	}
	return nil
}

// collect folds one video's subtitle state into the accumulators.
//
// A subtitle with no path is embedded in the video rather than a file of its
// own. It is skipped because path is this table's identity: there is nothing to
// key an embedded track by, and inventing one would create a row that no later
// pass could ever match again.
func (b *Bazarr) collect(
	subs map[string]*model.SubtitleFile,
	wanted *[]model.SubtitleWanted,
	contentID *model.ID,
	fileID model.ID,
	have []bazarr.Subtitle,
	missing []bazarr.Wanted,
) {
	var libraryFile *model.ID
	if fileID != (model.ID{}) {
		libraryFile = &fileID
	}

	for _, s := range have {
		if s.Path == "" {
			continue
		}
		subs[s.Path] = &model.SubtitleFile{
			Path:            s.Path,
			ContentID:       contentID,
			LibraryFileID:   libraryFile,
			LanguageName:    strPtr(s.Name),
			LanguageCode:    strPtr(s.Code3),
			Forced:          boolPtr(s.Forced),
			HearingImpaired: boolPtr(s.HI),
		}
	}

	if contentID == nil {
		// A gap is stated about a piece of content. Without one there is
		// nothing to record it against.
		return
	}
	for _, w := range missing {
		if w.Name == "" {
			continue
		}
		*wanted = append(*wanted, model.SubtitleWanted{
			ContentID:       *contentID,
			LanguageName:    w.Name,
			LanguageCode:    strPtr(w.Code3),
			Forced:          boolPtr(w.Forced),
			HearingImpaired: boolPtr(w.HI),
			ObservedAt:      time.Now().UTC(),
		})
	}
}

// collectHistory adds provenance to the subtitles already collected.
//
// Only downloads are read. A deletion refers to a file that is gone, and a sync
// re-times an existing one without changing where it came from — neither is a
// statement about provenance, and both would overwrite one with nothing.
//
// History rows are not created here either: a history entry for a subtitle the
// listing did not report is a subtitle that no longer exists.
func (b *Bazarr) collectHistory(ctx context.Context, subs map[string]*model.SubtitleFile) error {
	for _, kind := range [...]string{"series", "movies"} {
		records, err := b.client.History(ctx, kind)
		if err != nil {
			return err
		}
		for _, r := range records {
			switch r.Action {
			case bazarr.ActionDownload, bazarr.ActionUpgrade, bazarr.ActionManual:
			default:
				continue
			}
			sub, ok := subs[r.SubtitlesPath]
			if !ok {
				continue
			}
			sub.Provider = strPtr(r.Provider)
			sub.ProviderSubID = strPtr(r.SubsID)
			sub.Score = parseScore(r.Score)
			sub.Matches = strPtr(strings.Join(r.Matches, "; "))
			sub.Mismatches = strPtr(strings.Join(r.DontMatches, "; "))
			sub.Upgradable = boolPtr(r.Upgradable)
			sub.DownloadedAt = parseBazarrTime(r.ParsedTimestamp)
		}
	}
	return nil
}

// collectBlacklist marks the subtitles Bazarr has been told to refuse.
func (b *Bazarr) collectBlacklist(ctx context.Context, subs map[string]*model.SubtitleFile) error {
	for _, kind := range [...]string{"series", "movies"} {
		records, err := b.client.Blacklist(ctx, kind)
		if err != nil {
			return err
		}
		for _, r := range records {
			if sub, ok := subs[r.SubtitlesPath]; ok {
				sub.Blacklisted = boolPtr(true)
			}
		}
	}
	return nil
}

// write persists the collected rows.
func (b *Bazarr) write(
	ctx context.Context,
	subs map[string]*model.SubtitleFile,
	wanted []model.SubtitleWanted,
) (files, wantedCount int, err error) {
	// Languages are a foreign key from both tables, so they go first.
	seen := map[string]struct{}{}
	var languages []model.Language
	add := func(name, code string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		languages = append(languages, model.Language{Name: name, Code: strPtr(code)})
	}
	for _, s := range subs {
		add(deref(s.LanguageName), deref(s.LanguageCode))
	}
	for _, w := range wanted {
		add(w.LanguageName, deref(w.LanguageCode))
	}
	if err := dao.Upsert(ctx, b.dao, languages, []string{"name"},
		[]string{"code"}); err != nil {
		return 0, 0, fmt.Errorf("%s: languages: %w", b.serviceName, err)
	}

	rows := make([]model.SubtitleFile, 0, len(subs))
	for _, s := range subs {
		rows = append(rows, *s)
	}
	if err := dao.Upsert(ctx, b.dao, rows, []string{"path"}, []string{
		"content_id", "library_file_id", "language_name", "language_code",
		"forced", "hearing_impaired", "provider", "provider_sub_id", "score",
		"matches", "mismatches", "blacklisted", "upgradable", "downloaded_at",
	}); err != nil {
		return 0, 0, fmt.Errorf("%s: subtitles: %w", b.serviceName, err)
	}

	if err := dao.Upsert(ctx, b.dao, wanted,
		[]string{"content_id", "language_name"},
		[]string{"language_code", "forced", "hearing_impaired", "observed_at"},
	); err != nil {
		return 0, 0, fmt.Errorf("%s: wanted: %w", b.serviceName, err)
	}
	return len(rows), len(wanted), nil
}

// contentIndex maps "<service>\x00<local key>" to a content id.
//
// Loaded for the two *arrs whose ids Bazarr carries. Doing it as one query
// rather than a lookup per episode matters here: a subtitle pass touches every
// episode in the library, not a working set.
func (b *Bazarr) contentIndex(ctx context.Context) (map[string]model.ID, error) {
	ce := b.dao.Q().ContentExternalID
	rows, err := ce.WithContext(ctx).
		Where(ce.Provider.In(b.sonarrName, b.radarrName)).
		Find()
	if err != nil {
		return nil, fmt.Errorf("%s: content index: %w", b.serviceName, err)
	}
	out := make(map[string]model.ID, len(rows))
	for _, r := range rows {
		out[r.Provider+"\x00"+r.Value] = r.ContentID
	}
	return out, nil
}

// fileIndex maps a video path to its library file.
func (b *Bazarr) fileIndex(ctx context.Context) (map[string]model.ID, error) {
	lf := b.dao.Q().LibraryFile
	rows, err := lf.WithContext(ctx).Select(lf.ID, lf.Path).Find()
	if err != nil {
		return nil, fmt.Errorf("%s: file index: %w", b.serviceName, err)
	}
	out := make(map[string]model.ID, len(rows))
	for _, r := range rows {
		out[r.Path] = r.ID
	}
	return out, nil
}

// parseScore reads Bazarr's match score, which arrives as a percentage string.
func parseScore(s string) *float64 {
	s = strings.TrimSuffix(strings.TrimSpace(s), "%")
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

// bazarrTimeLayouts are the formats parsed_timestamp has been seen in.
//
// Bazarr renders it for display rather than serialising it, so the format
// follows the instance's date setting. Tried in order; an unparseable value
// yields NULL rather than a wrong time.
var bazarrTimeLayouts = [...]string{
	"01/02/06 15:04:05",
	"02/01/06 15:04:05",
	"2006-01-02 15:04:05",
	time.RFC3339,
}

// parseBazarrTime converts a rendered timestamp.
func parseBazarrTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range bazarrTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			u := t.UTC()
			return &u
		}
	}
	return nil
}

// deref reads an optional string, treating nil as empty.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
