package model

import "gorm.io/gorm"

import "time"

// IndexerGrab is a release pulled through Prowlarr.
//
// Separate from Grab: Prowlarr proxies the fetch and never learns the infohash,
// so it cannot name the torrent it produced. What it does answer, and nothing
// else does, is which indexer a release was actually pulled through and which
// client asked for it.
type IndexerGrab struct {
	Base
	ServiceName string    `gorm:"not null;uniqueIndex:idx_igrab,priority:1"`
	SourceRef   string    `gorm:"not null;uniqueIndex:idx_igrab,priority:2"`
	At          time.Time `gorm:"not null;index:,sort:desc"`

	IndexerName *string `gorm:"index"`
	ReleaseID   *ID     `gorm:"type:uuid;index"`
	// Source is the asking client as Prowlarr saw it: autobrr, CrossSeed, an
	// *arr. SourceHost distinguishes two instances of the same kind.
	Source     *string
	SourceHost *string
	GrabMethod *string
	ElapsedMs  *int
	URL        *string `gorm:"column:url"`

	Indexer *Indexer `gorm:"foreignKey:IndexerName;references:Name"`
	Release *Release `gorm:"foreignKey:ReleaseID"`
	Service *Service `gorm:"foreignKey:ServiceName;references:Name"`
}

func (IndexerGrab) TableName() string { return "indexer_grabs" }

// BeforeSave strips the passkey on a grab's download link.
func (g *IndexerGrab) BeforeSave(*gorm.DB) error {
	redactURL(g.URL)
	return nil
}
