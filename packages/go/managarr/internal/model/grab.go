package model

import "time"

// Grab is an *arr deciding to fetch a release.
//
// The strongest correlation in the model: the *arr recorded the release name it
// grabbed AND the download client confirmed the infohash, so this row ties a
// release to a torrent on evidence rather than on a title comparison. Every
// weaker join in this schema exists only for the cases this one cannot cover.
type Grab struct {
	Base
	ServiceName string    `gorm:"not null;uniqueIndex:idx_grab,priority:1"`
	SourceRef   string    `gorm:"not null;uniqueIndex:idx_grab,priority:2"`
	GrabbedAt   time.Time `gorm:"not null;index:,sort:desc"`

	// OfferID ties a grab back to the offer it came from, where there
	// was one. Null for a grab that came out of a search rather than an
	// announce.
	OfferID *ID `gorm:"type:uuid;index"`

	ReleaseID       *ID     `gorm:"type:uuid;index"`
	ContentID       *ID     `gorm:"type:uuid;index"`
	TorrentInfohash *string `gorm:"index"`
	IndexerName     *string

	QualityName       *string
	QualityRevision   *int
	CustomFormatScore *int
	SizeBytes         *int64
	IndexerFlags      *string
	ReleaseGroup      *string

	Release *Release `gorm:"foreignKey:ReleaseID"`
	Content *Content `gorm:"foreignKey:ContentID"`
	Torrent *Torrent `gorm:"foreignKey:TorrentInfohash;references:Infohash"`
	Offer   *Offer   `gorm:"foreignKey:OfferID"`
	Service *Service `gorm:"foreignKey:ServiceName;references:Name"`
	Indexer *Indexer `gorm:"foreignKey:IndexerName;references:Name"`
	Quality *Quality `gorm:"foreignKey:QualityName;references:Name"`
}

func (Grab) TableName() string { return "grabs" }
