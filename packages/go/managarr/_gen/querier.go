package main

import (
	"time"

	"gorm.io/gen"

	"homelab/managarr/internal/model"
)

// LibraryFileQuerier holds the statements the generated field API cannot
// express, written as annotated SQL that gen turns into typed methods.
//
// ApplyBasic covers anything shaped like "assign one value per column". The
// method below is not that on two counts: it assigns a DIFFERENT value per ROW,
// joined in from a VALUES list, and its CASE reads the stored row while writing
// it. Hand-written SQL was the alternative, and hand-written SQL is what the
// generated API exists to remove — so it is declared here, where the parameters
// stay Go values the compiler checks and the column names are verified against
// the model at generation time.
//
// FORMAT IS LOAD-BEARING. gen splits a method's doc comment at the first blank
// line and treats everything after it as SQL, so each method gets exactly one
// description paragraph and then nothing but the statement. Rationale that
// would normally sit in the doc comment has to live up here instead.
//
// On MarkRemovals specifically: one statement per chunk rather than one per
// file, because a plain IN list cannot carry a distinct timestamp per path and
// a statement per deletion is thousands of round trips on a backfill.
// superseded_at is set only for an upgrade, and only when not already set — the
// earliest replacement is the true one, and a path reused by a later file must
// not rewrite it.
type LibraryFileQuerier interface {
	// MarkRemovals records what the *arr deleted, with each file's own timestamp and reason.
	//
	// UPDATE @@table AS lf
	//    SET removed_at = v.at,
	//        removal_reason = v.reason,
	//        superseded_at = CASE
	//            WHEN v.reason = @upgradeReason AND lf.superseded_at IS NULL
	//            THEN v.at ELSE lf.superseded_at END
	//   FROM (VALUES
	//   {{for i, r := range removals}}
	//     {{if i > 0}},{{end}} (@r.Path, @r.At::timestamptz, @r.Reason)
	//   {{end}}
	//   ) AS v(path, at, reason)
	//  WHERE lf.path = v.path
	MarkRemovals(removals []model.LibraryFileRemoval, upgradeReason string) (gen.RowsAffected, error)

	// LinkRips marks the library files a rip produced.
	//
	// UPDATE @@table lf
	//    SET rip_id = r.id
	//   FROM beets_items b
	//   JOIN rips r ON r.release_mbid = b.mb_album_id
	//  WHERE lf.rip_id IS NULL
	//    AND b.library_file_id = lf.id
	LinkRips() (gen.RowsAffected, error)

	// LinkSoulseekDownloads names the Soulseek download each library file came from.
	//
	// UPDATE @@table lf
	//    SET soulseek_download_id = d.id
	//   FROM (SELECT DISTINCT ON (file_data_inode, file_data_size_bytes)
	//                id, file_data_inode, file_data_size_bytes
	//           FROM soulseek_downloads
	//          WHERE file_data_inode IS NOT NULL
	//          ORDER BY file_data_inode, file_data_size_bytes, finished_at) d
	//  WHERE lf.soulseek_download_id IS NULL
	//    AND lf.file_data_inode IS NOT NULL
	//    AND lf.file_data_inode = d.file_data_inode
	//    AND lf.file_data_size_bytes = d.file_data_size_bytes
	LinkSoulseekDownloads() (gen.RowsAffected, error)
}

// SearchQuerier gives a Prowlarr search the content it was looking for.
//
// One method per provider rather than one method taking a column name. The
// column and the provider vary together — tvdb_id is only ever matched against
// provider 'tvdb' — and the previous form interpolated both into the statement
// with Sprintf, which is a column name assembled at runtime and checked by
// nothing.
//
// Order matters at the call site, not here: each is NULL-only, so running them
// most-specific first means tvdb wins over imdb for a record carrying both,
// rather than the winner depending on which statement ran last.
type SearchQuerier interface {
	// LinkContentByTvdb resolves searches that carried a TVDB id.
	//
	// UPDATE @@table s
	//    SET content_id = c.content_id
	//   FROM content_external_ids c
	//  WHERE s.content_id IS NULL
	//    AND s.tvdb_id IS NOT NULL
	//    AND c.provider = 'tvdb'
	//    AND c.value = s.tvdb_id
	LinkContentByTvdb() (gen.RowsAffected, error)

	// LinkContentByTmdb resolves searches that carried a TMDB id.
	//
	// UPDATE @@table s
	//    SET content_id = c.content_id
	//   FROM content_external_ids c
	//  WHERE s.content_id IS NULL
	//    AND s.tmdb_id IS NOT NULL
	//    AND c.provider = 'tmdb'
	//    AND c.value = s.tmdb_id
	LinkContentByTmdb() (gen.RowsAffected, error)

	// LinkContentByImdb resolves searches that carried an IMDb id.
	//
	// UPDATE @@table s
	//    SET content_id = c.content_id
	//   FROM content_external_ids c
	//  WHERE s.content_id IS NULL
	//    AND s.imdb_id IS NOT NULL
	//    AND c.provider = 'imdb'
	//    AND c.value = s.imdb_id
	LinkContentByImdb() (gen.RowsAffected, error)
}

// GrabQuerier attaches each grab to the offer that caused it.
//
// Two passes, tried in order, because an *arr records a download id only when
// the client accepted the release — so most older history has no infohash to
// join on and needs the weaker key.
//
// Both use DISTINCT ON to pick ONE offer per key. The same torrent is regularly
// announced on IRC and later proposed again as a cross-seed candidate; the
// announce is preferred and then the earliest, so the answer is the offer that
// actually caused the grab rather than a later sighting of the same torrent.
//
// LinkOffersByRelease is still an equality join, not a similarity one: the
// release title is unique by construction and the indexer pins which tracker's
// copy it was.
type GrabQuerier interface {
	// LinkOffersByHash attaches a grab to the offer carrying the same infohash.
	//
	// UPDATE @@table g
	//    SET offer_id = o.id
	//   FROM (SELECT DISTINCT ON (infohash) id, infohash
	//           FROM offers
	//          WHERE infohash IS NOT NULL
	//          ORDER BY infohash, (kind = 'announce') DESC, discovered_at) o
	//  WHERE g.offer_id IS NULL
	//    AND g.torrent_infohash IS NOT NULL
	//    AND o.infohash = g.torrent_infohash
	LinkOffersByHash() (gen.RowsAffected, error)

	// LinkOffersByRelease attaches a grab to an announce of the same release on the same indexer.
	//
	// UPDATE @@table g
	//    SET offer_id = o.id
	//   FROM (SELECT DISTINCT ON (release_id, indexer_name) id, release_id, indexer_name
	//           FROM offers
	//          WHERE kind = 'announce' AND release_id IS NOT NULL
	//          ORDER BY release_id, indexer_name, discovered_at) o
	//  WHERE g.offer_id IS NULL
	//    AND g.release_id IS NOT NULL
	//    AND g.indexer_name IS NOT NULL
	//    AND g.release_id = o.release_id
	//    AND g.indexer_name = o.indexer_name
	LinkOffersByRelease() (gen.RowsAffected, error)
}

// ReleaseQuerier gives a release the content it was grabbed for.
type ReleaseQuerier interface {
	// LinkContent takes the content from the earliest grab of this release.
	//
	// UPDATE @@table r
	//    SET content_id = g.content_id
	//   FROM (SELECT DISTINCT ON (release_id) release_id, content_id
	//           FROM grabs
	//          WHERE release_id IS NOT NULL AND content_id IS NOT NULL
	//          ORDER BY release_id, grabbed_at) g
	//  WHERE r.content_id IS NULL
	//    AND r.id = g.release_id
	LinkContent() (gen.RowsAffected, error)
}

// TorrentQuerier names a torrent's release, from a grab where one exists and
// from an offer otherwise.
//
// The second covers everything no *arr ever grabbed: a cross-seed candidate, or
// an announce whose webhook recorded the hash before anything downloaded it.
type TorrentQuerier interface {
	// LinkReleaseFromGrab names a torrent's release from the grab that took it.
	//
	// UPDATE @@table t
	//    SET release_id = g.release_id
	//   FROM (SELECT DISTINCT ON (torrent_infohash) torrent_infohash, release_id
	//           FROM grabs
	//          WHERE torrent_infohash IS NOT NULL AND release_id IS NOT NULL
	//          ORDER BY torrent_infohash, grabbed_at) g
	//  WHERE t.release_id IS NULL
	//    AND t.infohash = g.torrent_infohash
	LinkReleaseFromGrab() (gen.RowsAffected, error)

	// LinkReleaseFromOffer names a torrent's release from the offer that carried both.
	//
	// UPDATE @@table t
	//    SET release_id = o.release_id
	//   FROM (SELECT DISTINCT ON (infohash) infohash, release_id
	//           FROM offers
	//          WHERE infohash IS NOT NULL AND release_id IS NOT NULL
	//          ORDER BY infohash, (kind = 'announce') DESC, discovered_at) o
	//  WHERE t.release_id IS NULL
	//    AND t.infohash = o.infohash
	LinkReleaseFromOffer() (gen.RowsAffected, error)
}

// RipQuerier gives a rip the album Lidarr tracks.
//
// The rip log names a MusicBrainz RELEASE and Lidarr tracks a release GROUP, so
// the two never join directly. beets holds both — it tags every track with the
// release id and has already been matched to Lidarr's content — which makes it
// the bridge rather than a coincidence.
type RipQuerier interface {
	// LinkContent bridges a rip to Lidarr's album through beets.
	//
	// UPDATE @@table r
	//    SET content_id = b.content_id
	//   FROM (SELECT DISTINCT ON (mb_album_id) mb_album_id, content_id
	//           FROM beets_items
	//          WHERE mb_album_id IS NOT NULL AND content_id IS NOT NULL
	//          ORDER BY mb_album_id) b
	//  WHERE r.content_id IS NULL
	//    AND r.release_mbid IS NOT NULL
	//    AND r.release_mbid = b.mb_album_id
	LinkContent() (gen.RowsAffected, error)
}

// SoulseekUploadQuerier names the library file a peer took.
//
// Joined on the inode rather than the path, though slskd reports a library path
// directly and the two usually agree. The inode survives an *arr rename and the
// path does not, so matching on the path would quietly stop working for exactly
// the files that get reorganised most.
//
// This is what turns a pile of transfers into an answer: group the uploads by
// library file and you have what of yours people actually want.
type SoulseekUploadQuerier interface {
	// LinkLibraryFiles attaches each upload to the library file it served.
	//
	// UPDATE @@table u
	//    SET library_file_id = lf.id
	//   FROM library_files lf
	//  WHERE u.library_file_id IS NULL
	//    AND u.file_data_inode IS NOT NULL
	//    AND lf.file_data_inode = u.file_data_inode
	//    AND lf.file_data_size_bytes = u.file_data_size_bytes
	LinkLibraryFiles() (gen.RowsAffected, error)
}

// SidecarCursorQuerier is the cursor advance, which is an upsert whose update
// clause reads the stored row.
//
// events_read ACCUMULATES rather than being overwritten, so a pass that reads
// nothing leaves the total intact — which is what makes "has this source ever
// produced anything" answerable at all. last_event_at coalesces for the mirror
// reason: a page whose events carried no timestamp must not blank a real one
// recorded earlier.
//
// Neither is a value the caller can compute and assign. Both are functions of
// what is already stored, evaluated inside the statement so two passes
// finishing together cannot read-modify-write over each other.
//
// See LibraryFileQuerier on comment format: one description paragraph, blank
// line, then SQL and nothing else.
type SidecarCursorQuerier interface {
	// Advance moves the cursor forward and folds this page into its counters.
	//
	// INSERT INTO @@table (source, cursor, last_event_at, events_read, updated_at)
	// VALUES (@source, @cursor, @lastEventAt, @read, @now)
	// ON CONFLICT (source) DO UPDATE SET
	//     cursor = EXCLUDED.cursor,
	//     last_event_at = COALESCE(EXCLUDED.last_event_at, sidecar_cursors.last_event_at),
	//     events_read = sidecar_cursors.events_read + @read,
	//     updated_at = EXCLUDED.updated_at
	Advance(source, cursor string, lastEventAt *time.Time, read int64, now time.Time) error
}
