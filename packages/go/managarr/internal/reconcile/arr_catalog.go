package reconcile

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"homelab/clients/arr"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// arrIndex maps an instance's own numeric ids onto content keys.
//
// Built once per pass and consulted by every later step, because the *arr
// refers to everything by its local integer and the model refers to everything
// by UUID. Without this the files, history, queue and blocklist steps would each
// re-resolve the same few thousand entities.
type arrIndex struct {
	// parent is seriesId / movieId / artistId.
	parent map[int64]model.ID
	// child is episodeId / albumId.
	child map[int64]model.ID
	// file is episodeFileId / trackFileId, pointing at the LEAF content the
	// file holds. Empty for Radarr, whose files name their movie directly.
	file map[int64]model.ID
	// total counts every content row the catalog touched, including seasons,
	// which have no numeric id of their own.
	total int
}

func newArrIndex() *arrIndex {
	return &arrIndex{
		parent: map[int64]model.ID{},
		child:  map[int64]model.ID{},
		file:   map[int64]model.ID{},
	}
}

func (ix *arrIndex) count() int { return ix.total }

// contentFor resolves a record's content from whichever ids it carries.
//
// Most specific first: a Sonarr history row names both its series and its
// episode, and the episode is the answer. Returns nil when the *arr referenced
// something it no longer tracks, which is normal for old history.
func (ix *arrIndex) contentFor(episodeID, albumID, seriesID, movieID, artistID int64) *model.ID {
	for _, id := range [...]int64{episodeID, albumID} {
		if id != 0 {
			if c, ok := ix.child[id]; ok {
				return &c
			}
		}
	}
	for _, id := range [...]int64{seriesID, movieID, artistID} {
		if id != 0 {
			if c, ok := ix.parent[id]; ok {
				return &c
			}
		}
	}
	return nil
}

// catalog accumulates the rows one pass will write.
//
// Contents are held in three levels rather than one slice because parent_id is
// a self-referencing foreign key: a season cannot be inserted before its series
// exists. Levels are written in order.
type catalog struct {
	level0 []model.Content // series, movie, artist
	level1 []model.Content // season, album
	level2 []model.Content // episode, track
	local  []model.ContentExternalID
	shared []model.ContentExternalID
	genres []model.ContentGenre
	studio []model.Studio
}

// syncCatalog records everything the instance tracks.
func (a *Arr) syncCatalog(ctx context.Context) (*arrIndex, error) {
	parents, err := a.client.Parents(ctx)
	if err != nil {
		return nil, err
	}

	known, err := a.knownContent(ctx)
	if err != nil {
		return nil, err
	}
	studios, err := a.knownStudios(ctx)
	if err != nil {
		return nil, err
	}

	ix := newArrIndex()
	cat := &catalog{}

	// idFor returns the content id for one of this instance's entities,
	// minting one on first sight. The local key is namespaced by entity type
	// because the apps number each collection from 1 independently — series 5
	// and episode 5 both exist.
	idFor := func(localKey string) model.ID {
		if id, ok := known[localKey]; ok {
			return id
		}
		id := model.NewID()
		known[localKey] = id
		cat.local = append(cat.local, model.ContentExternalID{
			ContentID: id,
			Provider:  a.serviceName,
			Value:     localKey,
		})
		return id
	}

	// studioFor resolves a production company or network, minting one on first
	// sight. Kind is part of the key: HBO the network and HBO Films the studio
	// are different rows, and nothing here can tell which a bare name means.
	studioFor := func(name, kind string) *model.ID {
		if name == "" {
			return nil
		}
		key := kind + "\x00" + name
		id, ok := studios[key]
		if !ok {
			id = model.NewID()
			studios[key] = id
			cat.studio = append(cat.studio, model.Studio{
				Base: model.Base{ID: id}, Name: name, Kind: kind,
			})
		}
		return &id
	}

	for _, p := range parents {
		if err := a.collectParent(ctx, p, ix, cat, idFor, studioFor); err != nil {
			return nil, err
		}
	}

	if err := a.writeCatalog(ctx, cat); err != nil {
		return nil, err
	}
	ix.total = len(cat.level0) + len(cat.level1) + len(cat.level2)
	return ix, nil
}

// collectParent builds the rows for one series, movie or artist and its
// children.
func (a *Arr) collectParent(
	ctx context.Context,
	p arr.Parent,
	ix *arrIndex,
	cat *catalog,
	idFor func(string) model.ID,
	studioFor func(string, string) *model.ID,
) error {
	switch a.client.Kind() {
	case arr.Radarr:
		id := idFor("movie:" + strconv.FormatInt(p.ID, 10))
		ix.parent[p.ID] = id
		cat.level0 = append(cat.level0, model.Content{
			Base:      model.Base{ID: id},
			Kind:      model.KindMovie,
			Title:     p.Name(),
			SortTitle: strPtr(p.SortKey()),
			Year:      intPtr(p.Year),
			Overview:  strPtr(p.Overview),
			RuntimeMs: minutesToMs(p.Runtime),
			Monitored: boolPtr(p.Monitored),
			AddedAt:   p.Added,
			StudioID:  studioFor(p.Studio, model.StudioProduction),
		})
		cat.shared = append(cat.shared, sharedIDs(id, map[string]string{
			model.ProviderTMDB: numID(p.TmdbID),
			model.ProviderIMDB: p.ImdbID,
		})...)
		cat.genres = append(cat.genres, genreRows(id, p.Genres)...)
		return nil

	case arr.Sonarr:
		return a.collectSeries(ctx, p, ix, cat, idFor, studioFor)

	case arr.Lidarr:
		return a.collectArtist(ctx, p, ix, cat, idFor)
	}
	return fmt.Errorf("%s: unhandled kind %q", a.serviceName, a.client.Kind())
}

// collectSeries builds a series, its seasons and its episodes.
func (a *Arr) collectSeries(
	ctx context.Context,
	p arr.Parent,
	ix *arrIndex,
	cat *catalog,
	idFor func(string) model.ID,
	studioFor func(string, string) *model.ID,
) error {
	seriesID := idFor("series:" + strconv.FormatInt(p.ID, 10))
	ix.parent[p.ID] = seriesID
	cat.level0 = append(cat.level0, model.Content{
		Base:      model.Base{ID: seriesID},
		Kind:      model.KindSeries,
		Title:     p.Name(),
		SortTitle: strPtr(p.SortKey()),
		Year:      intPtr(p.Year),
		Overview:  strPtr(p.Overview),
		RuntimeMs: minutesToMs(p.Runtime),
		Monitored: boolPtr(p.Monitored),
		AddedAt:   p.Added,
		StudioID:  studioFor(p.Network, model.StudioNetwork),
	})
	cat.shared = append(cat.shared, sharedIDs(seriesID, map[string]string{
		model.ProviderTVDB: numID(p.TvdbID),
		model.ProviderIMDB: p.ImdbID,
	})...)
	cat.genres = append(cat.genres, genreRows(seriesID, p.Genres)...)

	seasons := make(map[int]model.ID, len(p.Seasons))
	for _, s := range p.Seasons {
		id := idFor(fmt.Sprintf("season:%d:%d", p.ID, s.SeasonNumber))
		seasons[s.SeasonNumber] = id
		cat.level1 = append(cat.level1, model.Content{
			Base:      model.Base{ID: id},
			Kind:      model.KindSeason,
			Title:     seasonTitle(s.SeasonNumber),
			ParentID:  &seriesID,
			Monitored: boolPtr(s.Monitored),
		})
	}

	episodes, err := a.client.Episodes(ctx, p.ID)
	if err != nil {
		return err
	}
	for _, e := range episodes {
		id := idFor("episode:" + strconv.FormatInt(e.ID, 10))
		ix.child[e.ID] = id
		if e.EpisodeFileID != 0 {
			ix.file[e.EpisodeFileID] = id
		}
		// Falls back to the series when Sonarr reports an episode in a season
		// the series resource did not list, which happens while a new season is
		// being added.
		parent := seriesID
		if s, ok := seasons[e.SeasonNumber]; ok {
			parent = s
		}
		cat.level2 = append(cat.level2, model.Content{
			Base:      model.Base{ID: id},
			Kind:      model.KindEpisode,
			Title:     e.Title,
			ParentID:  &parent,
			Overview:  strPtr(e.Overview),
			RuntimeMs: minutesToMs(e.Runtime),
			Monitored: boolPtr(e.Monitored),
			Year:      airYear(e.AirDateUtc),
		})
		if e.TvdbID != 0 {
			cat.shared = append(cat.shared, model.ContentExternalID{
				ContentID: id,
				Provider:  model.ProviderTVDBEpisode,
				Value:     numID(e.TvdbID),
			})
		}
	}
	return nil
}

// collectArtist builds an artist, its albums and its tracks.
//
// MusicBrainz ids are recorded at all three levels, and the album one is the
// single most valuable identifier in the music half of this stack: it is
// Lidarr's foreignAlbumId and beets' mb_releasegroupid, which is what lets a
// tagged file on disk be tied back to the release that was grabbed for it.
func (a *Arr) collectArtist(
	ctx context.Context,
	p arr.Parent,
	ix *arrIndex,
	cat *catalog,
	idFor func(string) model.ID,
) error {
	artistID := idFor("artist:" + strconv.FormatInt(p.ID, 10))
	ix.parent[p.ID] = artistID
	cat.level0 = append(cat.level0, model.Content{
		Base:      model.Base{ID: artistID},
		Kind:      model.KindArtist,
		Title:     p.Name(),
		SortTitle: strPtr(p.SortKey()),
		Overview:  strPtr(p.Overview),
		Monitored: boolPtr(p.Monitored),
		AddedAt:   p.Added,
	})
	cat.shared = append(cat.shared, sharedIDs(artistID, map[string]string{
		model.ProviderMusicBrainz: p.ForeignArtistID,
	})...)
	cat.genres = append(cat.genres, genreRows(artistID, p.Genres)...)

	albums, err := a.client.Albums(ctx, p.ID)
	if err != nil {
		return err
	}
	for _, al := range albums {
		id := idFor("album:" + strconv.FormatInt(al.ID, 10))
		ix.child[al.ID] = id
		cat.level1 = append(cat.level1, model.Content{
			Base:      model.Base{ID: id},
			Kind:      model.KindAlbum,
			Title:     al.Title,
			ParentID:  &artistID,
			Overview:  strPtr(al.Overview),
			Monitored: boolPtr(al.Monitored),
			AddedAt:   al.Added,
			Year:      airYear(al.ReleaseDate),
		})
		cat.shared = append(cat.shared, sharedIDs(id, map[string]string{
			model.ProviderMusicBrainz: al.ForeignAlbumID,
		})...)
		cat.genres = append(cat.genres, genreRows(id, al.Genres)...)
	}

	tracks, err := a.client.Tracks(ctx, p.ID)
	if err != nil {
		return err
	}
	for _, t := range tracks {
		id := idFor("track:" + strconv.FormatInt(t.ID, 10))
		if t.TrackFileID != 0 {
			ix.file[t.TrackFileID] = id
		}
		parent := artistID
		if al, ok := ix.child[t.AlbumID]; ok {
			parent = al
		}
		cat.level2 = append(cat.level2, model.Content{
			Base:     model.Base{ID: id},
			Kind:     model.KindTrack,
			Title:    t.Title,
			ParentID: &parent,
			// Lidarr reports duration in milliseconds, unlike every other
			// runtime in the family, which are minutes.
			RuntimeMs: int64Ptr(t.Duration),
			Monitored: boolPtr(t.Monitored),
		})
		cat.shared = append(cat.shared, sharedIDs(id, map[string]string{
			model.ProviderMusicBrainz: t.ForeignTrackID,
		})...)
	}
	return nil
}

// writeCatalog persists the accumulated rows in dependency order.
func (a *Arr) writeCatalog(ctx context.Context, cat *catalog) error {
	if err := dao.Insert(ctx, a.dao, cat.studio, []string{"name", "kind"}); err != nil {
		return fmt.Errorf("%s: studios: %w", a.serviceName, err)
	}

	updates := []string{
		"kind", "title", "sort_title", "year", "overview",
		"runtime_ms", "monitored", "added_at", "parent_id", "studio_id",
	}
	for _, level := range [...][]model.Content{cat.level0, cat.level1, cat.level2} {
		if err := dao.Upsert(ctx, a.dao, level, []string{"id"}, updates); err != nil {
			return fmt.Errorf("%s: contents: %w", a.serviceName, err)
		}
	}

	// The instance's own ids, keyed by (content, provider): re-running must not
	// duplicate them and the value for a given content never changes.
	if err := dao.Insert(ctx, a.dao, cat.local,
		[]string{"content_id", "provider"}); err != nil {
		return fmt.Errorf("%s: local ids: %w", a.serviceName, err)
	}

	// Third-party ids: first claim wins. Two contents asserting the same TMDb
	// id is a real conflict, but it is one to notice later rather than one to
	// fail a backfill over.
	//
	// No arbiter, because this table has TWO unique constraints and naming
	// either one leaves the other unguarded. Keyed on (provider, value) it
	// collided on the primary key (content_id, provider) instead, and since
	// Postgres does not apply ON CONFLICT to an index it was not given, the
	// 23505 propagated and killed the pass — taking radarr and lidarr with it,
	// because a failing pass aborts the ones after it. The library ingest for
	// all three stopped for nineteen hours before anything noticed.
	if err := dao.InsertIgnore(ctx, a.dao, cat.shared); err != nil {
		return fmt.Errorf("%s: external ids: %w", a.serviceName, err)
	}

	if err := dao.Insert(ctx, a.dao, cat.genres,
		[]string{"content_id", "genre"}); err != nil {
		return fmt.Errorf("%s: genres: %w", a.serviceName, err)
	}
	return nil
}

// knownContent loads the ids this instance has already been assigned.
//
// One query instead of a lookup per entity: a television library is tens of
// thousands of episodes, and a round trip each would make the catalog step the
// slowest thing in the service by an order of magnitude.
func (a *Arr) knownContent(ctx context.Context) (map[string]model.ID, error) {
	ce := a.dao.Q().ContentExternalID
	rows, err := ce.WithContext(ctx).
		Where(ce.Provider.Eq(a.serviceName)).
		Find()
	if err != nil {
		return nil, fmt.Errorf("%s: known content: %w", a.serviceName, err)
	}
	out := make(map[string]model.ID, len(rows))
	for _, r := range rows {
		out[r.Value] = r.ContentID
	}
	return out, nil
}

// knownStudios loads every studio, keyed by kind and name.
func (a *Arr) knownStudios(ctx context.Context) (map[string]model.ID, error) {
	rows, err := a.dao.Q().Studio.WithContext(ctx).Find()
	if err != nil {
		return nil, fmt.Errorf("%s: studios: %w", a.serviceName, err)
	}
	out := make(map[string]model.ID, len(rows))
	for _, r := range rows {
		out[r.Kind+"\x00"+r.Name] = r.ID
	}
	return out, nil
}

// sharedIDs builds external id rows, dropping the absent ones.
func sharedIDs(contentID model.ID, ids map[string]string) []model.ContentExternalID {
	out := make([]model.ContentExternalID, 0, len(ids))
	for provider, value := range ids {
		if value == "" || value == "0" {
			continue
		}
		out = append(out, model.ContentExternalID{
			ContentID: contentID, Provider: provider, Value: value,
		})
	}
	return out
}

// genreRows builds genre links.
func genreRows(contentID model.ID, genres []string) []model.ContentGenre {
	out := make([]model.ContentGenre, 0, len(genres))
	for _, g := range genres {
		if g == "" {
			continue
		}
		out = append(out, model.ContentGenre{ContentID: contentID, Genre: g})
	}
	return out
}

// numID renders a numeric external id, treating zero as absent.
func numID(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

// minutesToMs converts a Servarr runtime, which is always minutes.
func minutesToMs(minutes int) *int64 {
	return int64Ptr(int64(minutes) * int64(time.Minute/time.Millisecond))
}

// airYear takes the year from an air or release date.
func airYear(t *time.Time) *int {
	if t == nil || t.IsZero() {
		return nil
	}
	return intPtr(t.Year())
}

// seasonTitle names a season. Zero is Sonarr's specials bucket, not a season.
func seasonTitle(n int) string {
	if n == 0 {
		return "Specials"
	}
	return "Season " + strconv.Itoa(n)
}
