package model

import "time"

// Content kinds in scope. Books, comics and adult media extend this list
// without changing anything else in the model.
const (
	KindMovie   = "movie"
	KindSeries  = "series"
	KindSeason  = "season"
	KindEpisode = "episode"
	KindArtist  = "artist"
	KindAlbum   = "album"
	KindTrack   = "track"
)

// Content is the work itself, independent of how it was obtained.
//
// Self-referencing: series owns seasons, season owns episodes, artist owns
// albums, album owns tracks. That hierarchy is what makes "every torrent behind
// this series" one query rather than three.
//
// A UUID key because there is no natural one — a work is identified by external
// ids, and it has several, so they live in ContentExternalID.
//
// Rows here are created by RESOLUTION, never blindly. A service reports its own
// id plus whatever shared ids it knows; the resolver looks those up and either
// finds the existing content or creates one. Inserting on sight is what
// produces two rows for one work that nothing can merge afterwards.
type Content struct {
	Base
	Kind  string `gorm:"not null;index"`
	Title string `gorm:"not null;index"`

	ParentID  *ID `gorm:"type:uuid;index"`
	SortTitle *string
	Year      *int
	Overview  *string
	RuntimeMs *int64
	// Monitored is the *arr's flag: whether it is still wanted.
	Monitored *bool
	AddedAt   *time.Time

	StudioID *ID `gorm:"type:uuid;index"`

	Parent *Content `gorm:"foreignKey:ParentID;references:ID"`
	Studio *Studio  `gorm:"foreignKey:StudioID"`
}

func (Content) TableName() string { return "contents" }
