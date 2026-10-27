package model

// SoulseekResult is one file one peer offered in answer to one search.
//
// The record of what was AVAILABLE, as opposed to what was taken. Soulseek has
// no tracker, no indexer and no catalogue — a file exists only while the peer
// holding it is online — so a search result is the only evidence that a given
// release was ever obtainable, and it is evidence with a shelf life of minutes.
//
// Kept in full rather than filtered to the files that became downloads. The
// alternatives are the useful part: which other peers had the album, at what
// bitrate, and whether a better copy was on offer than the one taken.
type SoulseekResult struct {
	SearchID     ID     `gorm:"type:uuid;primaryKey"`
	PeerUsername string `gorm:"primaryKey"`
	// RemotePath is the peer's own path, Windows-style and backslash
	// separated. It is the only identifier a Soulseek file has.
	RemotePath string `gorm:"primaryKey"`

	SizeBytes *int64
	Extension *string

	// Audio attributes, all optional: a peer may advertise none of them, and
	// most do. Together they are what distinguishes a 16/44 rip from a 24/96
	// one when both answer the same search under the same name.
	Bitrate    *int
	BitDepth   *int
	SampleRate *int
	LengthSec  *int

	// Locked means the peer holds the file behind a share restriction, so it
	// was listed and could not have been downloaded. Without it a locked file
	// reads as an alternative that was passed over.
	Locked *bool

	Search *SoulseekSearch `gorm:"foreignKey:SearchID;constraint:OnDelete:CASCADE"`
	Peer   *SoulseekPeer   `gorm:"foreignKey:PeerUsername;references:Username"`
}

func (SoulseekResult) TableName() string { return "soulseek_results" }
