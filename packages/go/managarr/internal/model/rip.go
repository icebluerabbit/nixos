package model

import "time"

// Rip is a CD ripped on this system.
//
// Unre-acquirable: no tracker holds it and no peer is seeding it, so if the
// files are lost the disc has to be ripped again. The whipper log beside the
// audio is the only provenance, and it does not follow the files when beets
// hands them to Lidarr — so SourcePath is the staging copy, not the library.
//
// ReleaseMBID is the specific pressing and is what resolves to Content;
// MusicBrainzDiscID identifies the physical disc and does not.
type Rip struct {
	Base
	SourcePath string `gorm:"not null;uniqueIndex"`

	ContentID *ID `gorm:"type:uuid;index"`
	// Column names are given explicitly for the two identifier fields.
	// GORM's snake_case rule splits on every case change, so MusicBrainzDiscID
	// becomes music_brainz_disc_id and ReleaseMBID becomes release_mb_id —
	// names nobody would write by hand and which raw SQL therefore gets wrong.
	MusicBrainzDiscID *string `gorm:"column:musicbrainz_disc_id"`
	ReleaseMBID       *string `gorm:"column:release_mbid;index"`
	CddbDiscID        *string
	// AccurateRipStatus is the disc-level verdict: accurate, no_match,
	// not_in_database or unverified.
	AccurateRipStatus *string
	RippedAt          *time.Time

	Content *Content `gorm:"foreignKey:ContentID"`
}

func (Rip) TableName() string { return "rips" }
