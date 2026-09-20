package model

import "time"

// BeetsItem is beets' tagging of one audio file.
//
// The richest music identity in the stack, and the piece that closes a gap
// nothing else could: MbReleaseGroupID is what Lidarr calls foreignAlbumId, so
// a ripped album resolves to the same Content as the Lidarr album. The whipper
// log alone could not do that — it records the specific pressing, which is a
// different MusicBrainz id.
//
// Covers the rip staging tree only: beets is pointed at /mnt/media/complete/rips,
// not at the whole music library.
type BeetsItem struct {
	Path string `gorm:"primaryKey"`

	LibraryFileID *ID `gorm:"type:uuid;index"`
	ContentID     *ID `gorm:"type:uuid;index"`

	MbTrackID        *string `gorm:"index"`
	MbAlbumID        *string `gorm:"index"`
	MbReleaseGroupID *string `gorm:"index"`
	MbArtistID       *string
	AcoustID         *string
	ISRC             *string `gorm:"column:isrc"`

	Album       *string
	AlbumArtist *string
	Artist      *string
	Title       *string
	TrackNumber *int
	DiscNumber  *int
	Year        *int
	Label       *string
	Country     *string
	Media       *string
	Format      *string
	Bitrate     *int
	BitrateMode *string
	SampleRate  *int
	Channels    *int
	LengthSec   *float64
	AddedAt     *time.Time

	LibraryFile *LibraryFile `gorm:"foreignKey:LibraryFileID"`
	Content     *Content     `gorm:"foreignKey:ContentID"`
}

func (BeetsItem) TableName() string { return "beets_items" }
