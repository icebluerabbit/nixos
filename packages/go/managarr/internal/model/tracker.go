package model

// Tracker is a BitTorrent tracker, identified by announce host.
type Tracker struct {
	AnnounceHost string `gorm:"primaryKey"`
	Name         *string
}

func (Tracker) TableName() string { return "trackers" }
