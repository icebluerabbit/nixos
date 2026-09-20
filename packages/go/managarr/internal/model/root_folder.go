package model

import "time"

// RootFolder is a library root an *arr manages, with its free space.
//
// Recorded because "why did the import fail" is often "the disk is full", and
// that answer exists only in the *arr's own view of its root folders.
type RootFolder struct {
	ServiceName string `gorm:"primaryKey"`
	Path        string `gorm:"primaryKey"`

	FreeBytes  *int64
	TotalBytes *int64
	Accessible *bool
	ObservedAt time.Time `gorm:"not null"`

	Service *Service `gorm:"foreignKey:ServiceName;references:Name"`
}

func (RootFolder) TableName() string { return "root_folders" }
