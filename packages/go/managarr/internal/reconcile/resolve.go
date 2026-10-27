package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"homelab/managarr/internal/dao"
)

// Resolve links rows that could not be linked when they were written.
//
// Every pass writes what its own service knows and leaves a foreign key NULL
// when the answer lives in another service's data. Prowlarr records a search
// against an IMDb id before any *arr has been read; autobrr records an announce
// before the grab that took it exists. Running the joins afterwards is what
// turns those into one connected record.
//
// Two rules hold for everything here:
//
//   - NULL only. A link is written once and never revised, so a pass ordering
//     accident cannot silently rewrite an established fact.
//   - Shared identifiers only. Every join below is an equality on an id that
//     two services independently recorded — an infohash, a TVDB number, a
//     MusicBrainz release. Nothing is matched by title, size or timing.
type Resolve struct {
	dao    *dao.DAO
	logger *slog.Logger
}

// NewResolve builds the pass.
func NewResolve(d *dao.DAO, logger *slog.Logger) *Resolve {
	return &Resolve{dao: d, logger: logger}
}

// ResolveStats reports how many rows each link filled.
type ResolveStats struct {
	SearchContent     int
	GrabOffer         int
	ReleaseContent    int
	TorrentRelease    int
	RipContent        int
	FileRip           int
	FileSoulseek      int
	UploadLibraryFile int
	Took              time.Duration
}

// link is one NULL-filling join.
type link struct {
	name string
	// run is a generated method — see _gen/querier.go. These are correlated
	// cross-table UPDATEs, which the field API has no form for, so they are
	// declared as annotated SQL and generated rather than assembled here as
	// strings. Several used to be built with Sprintf.
	run  func() (int64, error)
	into *int
}

// Run applies every link.
func (r *Resolve) Run(ctx context.Context) (ResolveStats, error) {
	var st ResolveStats
	started := time.Now()

	q := r.dao.Q()
	search := q.Search.WithContext(ctx)
	grab := q.Grab.WithContext(ctx)
	torrent := q.Torrent.WithContext(ctx)
	libraryFile := q.LibraryFile.WithContext(ctx)

	links := []link{
		// A Prowlarr search carries whichever external id the caller sent.
		// Tried most specific first and NULL-only, so tvdb wins over imdb
		// when a record carries both rather than the order depending on which
		// statement ran last.
		{"search content (tvdb)", search.LinkContentByTvdb, &st.SearchContent},
		{"search content (tmdb)", search.LinkContentByTmdb, &st.SearchContent},
		{"search content (imdb)", search.LinkContentByImdb, &st.SearchContent},

		// An *arr grab and an autobrr announce are the same event seen from
		// two sides. The infohash is the exact join; the release-and-indexer
		// pair is the fallback for grabs the *arr recorded without one.
		{"grab offer (infohash)", grab.LinkOffersByHash, &st.GrabOffer},
		{"grab offer (release)", grab.LinkOffersByRelease, &st.GrabOffer},

		// A release grabbed for a piece of content IS that content. Taken
		// from the earliest grab, so a later mis-grab cannot redefine it.
		{"release content", q.Release.WithContext(ctx).LinkContent, &st.ReleaseContent},

		// A torrent's release, from whichever side named it first.
		{"torrent release (grab)", torrent.LinkReleaseFromGrab, &st.TorrentRelease},
		{"torrent release (offer)", torrent.LinkReleaseFromOffer, &st.TorrentRelease},

		// An announce's infohash is NOT resolved here. It arrives on autobrr's
		// webhook, which now identifies the announce exactly through
		// (indexer, tracker_torrent_id), so the handler writes it to the right
		// row directly. This used to be a link that guessed from the release
		// when exactly one torrent mapped to it — which by construction never
		// fired for a cross-seeded release, the case it most needed to cover.

		// The music loop closes through MusicBrainz: whipper writes the
		// release id into the rip log, beets writes the same id onto every
		// tagged track, and beets has already been matched to Lidarr's album.
		{"rip content", q.Rip.WithContext(ctx).LinkContent, &st.RipContent},
		{"file rip", libraryFile.LinkRips, &st.FileRip},

		// And the Soulseek half of the same question. Music is imported by
		// move rather than by hardlink, and a move keeps the inode, so the
		// library file and the download it came from are the same bytes.
		{"file soulseek download", libraryFile.LinkSoulseekDownloads, &st.FileSoulseek},

		// And what peers took from this share, which is the only demand-side
		// fact in the model.
		{"upload library file", q.SoulseekUpload.WithContext(ctx).LinkLibraryFiles, &st.UploadLibraryFile},
	}

	for _, l := range links {
		rows, err := l.run()
		if err != nil {
			return st, fmt.Errorf("resolve: %s: %w", l.name, err)
		}
		*l.into += int(rows)
		r.logger.DebugContext(ctx, "resolved", "link", l.name, "rows", rows)
	}

	st.Took = time.Since(started)
	return st, nil
}
