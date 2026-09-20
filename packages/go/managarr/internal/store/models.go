package store

import "homelab/managarr/internal/model"

// Models is every table, in dependency order.
//
// AutoMigrate resolves foreign keys by walking this list, so a table must not
// appear before what it references. This list is also the definition of "the
// schema": a model not named here does not exist as far as the database is
// concerned, which is the check that replaces remembering to write a migration.
var Models = []any{
	// ── tier 1: no outbound references ─────────────────────────────────────
	&model.Service{},
	&model.Tracker{},
	&model.Client{},
	&model.Indexer{},
	&model.Filter{},
	&model.IrcNetwork{},
	&model.SoulseekPeer{},
	&model.Quality{},
	&model.Language{},
	&model.Studio{},
	&model.FileData{},
	// Bookkeeping rather than a fact about the stack: how far each pass has
	// read its sidecar. References nothing, and nothing references it.
	&model.SidecarCursor{},
	// What managarr deleted and why. Deliberately references nothing — it has
	// to outlive the torrent it describes, which is the whole point of it.
	&model.TorrentRemoval{},

	// ── tier 2: the catalog spine ──────────────────────────────────────────
	&model.Content{},
	&model.ContentExternalID{},
	&model.ContentGenre{},
	&model.Release{},

	// ── tier 3: service-scoped dimensions ──────────────────────────────────
	&model.CustomFormat{},
	&model.RootFolder{},
	&model.HealthEvent{},
	&model.IrcChannel{},
	&model.FilterIndexer{},

	// ── tier 4: acquisition ────────────────────────────────────────────────
	&model.Torrent{},
	&model.ClientTorrent{},
	&model.TorrentFile{},
	&model.TorrentTracker{},
	&model.SoulseekSearch{},
	&model.SoulseekDownload{},
	&model.SoulseekResult{},
	&model.Rip{},
	&model.Extraction{},

	// ── tier 5: discovery ──────────────────────────────────────────────────
	&model.Offer{},
	&model.Decision{},
	&model.Search{},
	&model.Grab{},
	&model.IndexerGrab{},
	&model.QueueItem{},
	&model.BlocklistEntry{},
	&model.ImportFailure{},
	&model.PackAttempt{},
	// After Torrent: both sides are infohashes that must already exist.
	&model.PackSource{},
	&model.SoularrEvent{},

	// ── tier 6: library ────────────────────────────────────────────────────
	&model.LibraryFile{},
	// After LibraryFile: an upload names the library file that was taken.
	&model.SoulseekUpload{},
	&model.MediaInfo{},
	&model.LibraryFileLanguage{},
	&model.LibraryFileCustomFormat{},
	&model.SubtitleFile{},
	&model.SubtitleWanted{},
	&model.BeetsItem{},
}
