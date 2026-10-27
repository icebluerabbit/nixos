package model

import "gorm.io/gorm"

import "time"

// QueueItem is something an *arr is currently working on.
//
// A snapshot of in-flight work: overwritten each pass, deleted when it leaves
// the queue. The only place a stuck import or a failing unpack is visible while
// it is still happening rather than after the fact.
type QueueItem struct {
	ServiceName string `gorm:"primaryKey"`
	ExternalID  string `gorm:"primaryKey"`

	ContentID       *ID     `gorm:"type:uuid;index"`
	ReleaseID       *ID     `gorm:"type:uuid"`
	TorrentInfohash *string `gorm:"index"`
	Status          *string
	TrackedStatus   *string
	ErrorMessage    *string
	SizeBytes       *int64
	SizeLeftBytes   *int64
	EstimatedAt     *time.Time
	ObservedAt      time.Time `gorm:"not null"`

	Service *Service `gorm:"foreignKey:ServiceName;references:Name"`
	Content *Content `gorm:"foreignKey:ContentID"`
	Torrent *Torrent `gorm:"foreignKey:TorrentInfohash;references:Infohash"`
	Release *Release `gorm:"foreignKey:ReleaseID"`
}

func (QueueItem) TableName() string { return "queue_items" }

// BeforeSave strips URLs quoted inside a queue item's complaint.
func (q *QueueItem) BeforeSave(*gorm.DB) error {
	redactText(q.ErrorMessage)
	return nil
}
