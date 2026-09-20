package model

import "time"

// SoularrEvent is one soularr album event.
//
// soularr bridges Lidarr's wanted list to Soulseek. Lidarr records the import
// that eventually results, but nothing on the Lidarr side distinguishes an
// album soularr fetched from one that came off a tracker, and soularr keeps no
// history of its own.
//
// It sends neither an id nor a timestamp, so identity is the event itself and
// LastSeenAt is when it was last observed. Including arrival time in the key
// meant every Vector redelivery inserted a fresh row.
type SoularrEvent struct {
	ContentID ID     `gorm:"type:uuid;primaryKey"`
	Event     string `gorm:"primaryKey"`

	// Count is set only on batch summaries, so a pointer keeps absent
	// distinguishable from zero.
	Count      *int
	LastSeenAt time.Time `gorm:"not null;index:,sort:desc"`

	Content *Content `gorm:"foreignKey:ContentID;constraint:OnDelete:CASCADE"`
}

func (SoularrEvent) TableName() string { return "soularr_events" }
