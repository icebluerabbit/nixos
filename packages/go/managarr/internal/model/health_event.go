package model

import "gorm.io/gorm"

import "time"

// HealthEvent is a service reporting a problem about itself.
//
// Keyed on (service, type, source) and updated in place: an *arr repeats the
// same health check every few minutes, and what matters is that the condition
// exists and when it started — not that it was reported nine hundred times.
type HealthEvent struct {
	ServiceName string `gorm:"primaryKey"`
	Type        string `gorm:"primaryKey"`
	Source      string `gorm:"primaryKey"`

	Message     *string
	FirstSeenAt time.Time `gorm:"not null"`
	LastSeenAt  time.Time `gorm:"not null"`

	Service *Service `gorm:"foreignKey:ServiceName;references:Name"`
}

func (HealthEvent) TableName() string { return "health_events" }

// BeforeSave strips URLs quoted inside a health check message.
//
// "Indexer unavailable" names the indexer by its full URL, passkey included.
func (h *HealthEvent) BeforeSave(*gorm.DB) error {
	redactText(h.Message)
	return nil
}
