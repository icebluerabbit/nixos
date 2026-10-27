package model

import "gorm.io/gorm"

import "time"

// BlocklistEntry is a release an *arr refuses to grab again.
//
// Worth capturing because it is the reason a wanted item stays unfetched while
// the indexer keeps offering it — a state that otherwise looks like nothing
// happening at all.
type BlocklistEntry struct {
	ServiceName string `gorm:"primaryKey"`
	ExternalID  string `gorm:"primaryKey"`

	ReleaseID       *ID `gorm:"type:uuid;index"`
	ContentID       *ID `gorm:"type:uuid"`
	TorrentInfohash *string
	Reason          *string
	IndexerName     *string
	BlockedAt       *time.Time

	Service *Service `gorm:"foreignKey:ServiceName;references:Name"`
	Release *Release `gorm:"foreignKey:ReleaseID"`
	Content *Content `gorm:"foreignKey:ContentID"`
	Indexer *Indexer `gorm:"foreignKey:IndexerName;references:Name"`
	Torrent *Torrent `gorm:"foreignKey:TorrentInfohash;references:Infohash"`
}

func (BlocklistEntry) TableName() string { return "blocklist_entries" }

// BeforeSave strips URLs quoted inside a blocklist reason.
func (b *BlocklistEntry) BeforeSave(*gorm.DB) error {
	redactText(b.Reason)
	return nil
}
