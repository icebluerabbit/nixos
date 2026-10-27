package model

import "time"

// FileData is one inode.
//
// Identity is (inode, size_bytes) rather than (device, inode). Inodes are
// recycled after deletion, so size guards against a stale row aliasing a new
// file. st_dev on an NFS client is the mount's number and not the server's, so
// it is not stable across remounts and is recorded as advisory only.
//
// Nlink is a SNAPSHOT. Anything acting on it must re-stat first — the stored
// value is only as good as LastObservedAt, and treating it as current is how
// the predecessor deleted files it should not have.
type FileData struct {
	Inode     int64 `gorm:"primaryKey"`
	SizeBytes int64 `gorm:"primaryKey"`

	Device         *int64
	Nlink          *int
	LastObservedAt time.Time `gorm:"not null"`
}

func (FileData) TableName() string { return "file_data" }
