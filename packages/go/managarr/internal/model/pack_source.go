package model

import "time"

// PackSource links a season pack to an episode torrent it was assembled from.
//
// The association seasonpackarr creates and then forgets. It takes a pack
// announce, finds episodes already in the client, hardlinks them into a pack
// directory, and the resulting pack is added as an ordinary torrent — after
// which nothing anywhere says which episodes went into it. Downstream, a pack
// looks exactly like something that was downloaded.
//
// This matters for the same reason the cross-seed pairing does: it decides what
// is safe to delete. A pack and its episodes are the SAME BYTES under different
// names, so removing an episode torrent's data while the pack still seeds, or
// the reverse, silently breaks whichever survives.
//
// HOW IT IS ESTABLISHED, and its one weakness: by file name. seasonpackarr
// names client torrents in its log the way qBittorrent named them, and the pack
// directory it builds contains files named after the episodes it linked. Those
// names join to client_torrents.name. That is string equality on a name rather
// than on an identifier, and it is the only route — seasonpackarr reports no
// infohash anywhere, in its log, its notifier or its API, and its two HTTP
// routes are POST-only decision endpoints with nothing to read back.
//
// Via records which of the two it came from, so a row's strength is visible
// rather than assumed.
type PackSource struct {
	// PackInfohash is the assembled season pack.
	PackInfohash string `gorm:"primaryKey"`
	// SourceInfohash is an episode torrent whose data the pack contains.
	SourceInfohash string `gorm:"primaryKey"`

	// Via is how the link was established — see SourceVia constants.
	Via string `gorm:"not null;index"`
	// MatchedName is the file or torrent name the join was made on, kept so a
	// wrong association can be diagnosed rather than merely doubted.
	MatchedName string `gorm:"not null"`

	ObservedAt time.Time `gorm:"not null"`

	Pack   *Torrent `gorm:"foreignKey:PackInfohash;references:Infohash"`
	Source *Torrent `gorm:"foreignKey:SourceInfohash;references:Infohash"`
}

// How a PackSource was established.
const (
	// SourceViaPackContents is the strongest available: a file inside the pack
	// directory resolved to a client torrent of that name. It describes what is
	// actually on disk now, rather than what a log said once.
	SourceViaPackContents = "pack_contents"
	// SourceViaLog is seasonpackarr's own statement that it matched an episode,
	// read from its structured log. Weaker: it records an intention at a moment,
	// and the pack may since have been rebuilt or the episode replaced.
	SourceViaLog = "log"
)

func (PackSource) TableName() string { return "pack_sources" }
