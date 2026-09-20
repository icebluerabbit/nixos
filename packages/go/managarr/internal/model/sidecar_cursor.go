package model

import "time"

// SidecarCursor is how far a pass has read one sidecar's /changes.
//
// The whole reason the sidecars exist. Every upstream here pages its history
// with no server-side filter, so without a stored position each hourly pass
// re-walks everything from the beginning — 28,000 *arr history records, 48,000
// cross-seed decisions, 64,000 slskd events, every hour, to learn about the
// handful that are new.
//
// Keyed by source rather than by sidecar name because the source is what the
// events carry: a sidecar renamed or rebuilt resumes where the last one stopped,
// which is correct, since the cursor describes a position in the UPSTREAM's log
// rather than in the sidecar's lifetime.
//
// The value is opaque and each sidecar defines its own encoding — an offset, a
// (timestamp, id) pair, a two-stage walk position. managarr stores it and hands
// it back; interpreting it here would put the sidecar's paging semantics in two
// places at once.
type SidecarCursor struct {
	// Source is the service the events came from: "sonarr", "cross-seed".
	Source string `gorm:"primaryKey"`
	// Cursor is the sidecar's opaque position. Empty means the walk has not
	// started, which is also what a reset looks like.
	Cursor string `gorm:"not null"`
	// LastEventAt is the OccurredAt of the newest event read. Diagnostic
	// rather than load-bearing: it is how a cursor that has quietly stopped
	// advancing becomes visible, which the cursor itself cannot show because
	// its encoding is opaque here.
	LastEventAt *time.Time
	// EventsRead counts everything ever read through this cursor, across
	// passes. A pass reporting zero new events is normal; this is what says
	// whether it has ever read any.
	EventsRead int64 `gorm:"not null;default:0"`
	UpdatedAt  time.Time
}

func (SidecarCursor) TableName() string { return "sidecar_cursors" }
