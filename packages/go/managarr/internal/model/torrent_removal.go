package model

import "time"

// TorrentRemoval is a torrent managarr deleted, and why.
//
// The only destructive thing managarr does, written down as a fact rather than
// left in a log line. Logs rotate; this has to outlive them, because it is the
// evidence for what comes next.
//
// What comes next is deciding whether a library file is orphaned. That is
// defensible when every torrent that provided it was removed BECAUSE its
// tracker dropped the record, and indefensible when they merely happen to be
// absent — and a reconcile pass cannot tell those apart, since both look
// identical once the torrent is gone from the client.
//
// This model exists because that distinction was already lost once: 39 imports
// in this database have a torrent that no client ever held, no import failure
// and no blocklist entry, and nothing recorded anywhere says what happened to
// them.
type TorrentRemoval struct {
	Base
	// Infohash is the torrent that was deleted. Not a foreign key: the
	// torrents row may be pruned later, and this record should outlive it.
	Infohash   string `gorm:"not null;index"`
	ClientName string `gorm:"not null;index"`
	// Reason is managarr's own vocabulary for why — "unregistered" today.
	Reason string `gorm:"not null;index"`
	// Tracker and Message are the evidence, verbatim. The message is what the
	// tracker actually said, and keeping it is what makes a wrong deletion
	// diagnosable rather than merely regrettable.
	Tracker string `gorm:"not null"`
	Message string `gorm:"not null"`
	Path    *string
	// SizeBytes is what the client had downloaded, which is the space the
	// deletion could reclaim — not necessarily what it did, since another
	// hardlink keeps the bytes in place.
	SizeBytes *int64
	// DeletedData records whether the data went with the torrent.
	DeletedData bool      `gorm:"not null"`
	RemovedAt   time.Time `gorm:"not null;index"`
}

func (TorrentRemoval) TableName() string { return "torrent_removals" }
