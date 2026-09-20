package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"homelab/clients/beets"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
	"homelab/sidecar"
	"homelab/sidecar/beetssvc"
)

// Beets reconciles the tag layer of the music library.
//
// beets is the only thing here that has read the files themselves. Lidarr knows
// what it asked for and what it filed; beets knows what the bytes actually say
// — which pressing, which label, which country, what the bitrate really is
// rather than what the release name claimed.
//
// It is also the join that closes the music loop. mb_releasegroupid is Lidarr's
// foreignAlbumId under a different name, so a tagged file resolves to the album
// the *arr tracks by shared identifier, and from there to the grab, the torrent
// and the announce that produced it.
type Beets struct {
	client *beets.Client
	// walk reads the library through the sidecar with a stored cursor. Nil
	// reads beets directly.
	//
	// The weakest cursor case of the five passes that have one, and worth
	// being honest about: beets has no since-filter, so the sidecar fetches
	// the whole library either way and the cursor saves writes rather than
	// reads. What it actually buys is that managarr stops holding a second
	// service's address and the item shape lives in one place.
	walk    *SidecarWalk
	dao     *dao.DAO
	lookups *dao.Lookups
	logger  *slog.Logger
}

// NewBeets builds the pass. A nil walk reads beets directly.
func NewBeets(c *beets.Client, walk *SidecarWalk, d *dao.DAO, l *dao.Lookups, logger *slog.Logger) *Beets {
	return &Beets{client: c, walk: walk, dao: d, lookups: l, logger: logger}
}

// withItems reads the library and runs fn over it, advancing the cursor only
// once fn has succeeded.
//
// The work has to happen INSIDE the walk, not after it. This pass builds two
// indexes and writes every row in one upsert at the end; a cursor advanced when
// the read finished would mark rows read that nothing had written yet, and the
// next pass would skip them permanently.
func (b *Beets) withItems(ctx context.Context, fn func(context.Context, []beets.Item) error) (string, error) {
	if b.walk == nil {
		items, err := b.client.Items(ctx)
		if err != nil {
			return "", err
		}
		return "", fn(ctx, items)
	}

	st, err := b.walk.RunAtomic(ctx, func(ctx context.Context, events []sidecar.Event) error {
		items := make([]beets.Item, 0, len(events))
		for _, e := range events {
			if e.Kind != beetssvc.KindItem {
				continue
			}
			it, err := sidecar.DecodePayload[beets.Item](e)
			if err != nil {
				return fmt.Errorf("beets: decode item: %w", err)
			}
			items = append(items, it)
		}
		return fn(ctx, items)
	})
	return st.Resumed, err
}

// BeetsStats reports what a run did.
type BeetsStats struct {
	Items   int
	Matched int
	// Resumed is the cursor this pass started from, empty on a first run or
	// when no sidecar is configured.
	Resumed string
	Took    time.Duration
}

// Run records every tagged file.
func (b *Beets) Run(ctx context.Context) (BeetsStats, error) {
	var st BeetsStats
	started := time.Now()
	b.lookups.Reset()

	resumed, err := b.withItems(ctx, func(ctx context.Context, items []beets.Item) error {
		// Both indexes are loaded whole because a beets pass touches the
		// entire music library rather than a working set, and a per-row lookup
		// against every track is a round trip per track.
		contents, err := b.contentIndex(ctx)
		if err != nil {
			return err
		}
		files, err := b.fileIndex(ctx)
		if err != nil {
			return err
		}

		rows := make([]model.BeetsItem, 0, len(items))
		for _, it := range items {
			if it.Path == "" {
				// The web plugin's include_paths option is off. Path is this
				// table's key and the only thing tying a tagged item to a
				// file, so there is nothing to record without it.
				continue
			}
			rows = append(rows, b.itemRow(it, contents, files, &st))
		}

		if err := dao.UpsertAll(ctx, b.dao, rows, []string{"path"}); err != nil {
			return fmt.Errorf("beets: items: %w", err)
		}
		st.Items = len(rows)

		if st.Items == 0 && len(items) > 0 {
			// Worth saying out loud rather than reporting a successful pass
			// that wrote nothing: every item came back without a path.
			b.logger.WarnContext(ctx, "beets returned no paths",
				"items", len(items), "hint", "enable the web plugin's include_paths")
		}
		return nil
	})
	if err != nil {
		return st, err
	}
	st.Resumed = resumed

	st.Took = time.Since(started)
	return st, nil
}

// itemRow converts one item and resolves what it can.
func (b *Beets) itemRow(it beets.Item, contents, files map[string]model.ID, st *BeetsStats) model.BeetsItem {
	row := model.BeetsItem{
		Path:             it.Path,
		MbTrackID:        strPtr(it.MbTrackID),
		MbAlbumID:        strPtr(it.MbAlbumID),
		MbReleaseGroupID: strPtr(it.MbReleaseGroupID),
		MbArtistID:       strPtr(it.MbArtistID),
		AcoustID:         strPtr(it.AcoustID),
		ISRC:             strPtr(it.ISRC),
		Album:            strPtr(it.Album),
		AlbumArtist:      strPtr(it.AlbumArtist),
		Artist:           strPtr(it.Artist),
		Title:            strPtr(it.Title),
		TrackNumber:      intPtr(it.Track),
		DiscNumber:       intPtr(it.Disc),
		Year:             intPtr(it.Year),
		Label:            strPtr(it.Label),
		Country:          strPtr(it.Country),
		Media:            strPtr(it.Media),
		Format:           strPtr(it.Format),
		Bitrate:          intPtr(it.Bitrate),
		BitrateMode:      strPtr(it.BitrateMode),
		SampleRate:       intPtr(it.SampleRate),
		Channels:         intPtr(it.Channels),
		LengthSec:        floatPtr(it.Length),
		AddedAt:          timePtr(it.AddedAt()),
	}

	// Path first: beets and Lidarr name the same file identically once Lidarr
	// has imported it, so this is exact when it hits.
	if id, ok := files[it.Path]; ok {
		row.LibraryFileID = &id
	}
	// Then the identifier ladder: track, then album. Both are MusicBrainz ids
	// Lidarr also carries, so a hit is a fact rather than a guess.
	for _, mbid := range [...]string{it.MbTrackID, it.MbAlbumID} {
		if mbid == "" {
			continue
		}
		if id, ok := contents[mbid]; ok {
			row.ContentID = &id
			st.Matched++
			break
		}
	}
	return row
}

// contentIndex maps a MusicBrainz id to the content it identifies.
//
// One provider, because that is the whole point: Lidarr's artist, album and
// track ids ARE MusicBrainz ids, written by the *arr pass under the same
// provider beets uses. No translation step exists because none is needed.
func (b *Beets) contentIndex(ctx context.Context) (map[string]model.ID, error) {
	ce := b.dao.Q().ContentExternalID
	rows, err := ce.WithContext(ctx).
		Where(ce.Provider.Eq(model.ProviderMusicBrainz)).
		Find()
	if err != nil {
		return nil, fmt.Errorf("beets: content index: %w", err)
	}
	out := make(map[string]model.ID, len(rows))
	for _, r := range rows {
		out[r.Value] = r.ContentID
	}
	return out, nil
}

// fileIndex maps a path to its library file.
func (b *Beets) fileIndex(ctx context.Context) (map[string]model.ID, error) {
	lf := b.dao.Q().LibraryFile
	rows, err := lf.WithContext(ctx).Select(lf.ID, lf.Path).Find()
	if err != nil {
		return nil, fmt.Errorf("beets: file index: %w", err)
	}
	out := make(map[string]model.ID, len(rows))
	for _, r := range rows {
		out[r.Path] = r.ID
	}
	return out, nil
}
