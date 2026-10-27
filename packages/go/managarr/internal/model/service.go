package model

import "time"

// Service is one deployed component of the stack.
//
// Recorded so a row can say which instance observed it: two Sonarrs, or a
// Lidarr replaced and re-added, are different sources of truth and their ids do
// not share a namespace.
type Service struct {
	Name string `gorm:"primaryKey"`
	// Kind is the software, e.g. sonarr. Name is the deployment.
	Kind    string `gorm:"not null;index"`
	Version *string
	// LastSeenAt is when this service last answered, which is how a silently
	// dead integration becomes visible.
	LastSeenAt *time.Time
}

func (Service) TableName() string { return "services" }
