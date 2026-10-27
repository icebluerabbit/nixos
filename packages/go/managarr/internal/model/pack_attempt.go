package model

import "time"

// PackAttempt is seasonpackarr trying to assemble a season pack.
//
// Most attempts fail with "no matching releases in client", meaning the
// announced pack had no episodes locally to match against. Those are the
// interesting rows: they are the pack grabs that did not happen, and nothing
// else in the stack records them.
//
// Keyed with the timestamp because an attempt genuinely repeats for the same
// release, and seasonpackarr's own embed timestamp is what distinguishes them.
type PackAttempt struct {
	ReleaseID   ID        `gorm:"type:uuid;primaryKey"`
	AttemptedAt time.Time `gorm:"primaryKey"`

	Outcome *string
	Reason  *string
	Client  *string

	Release *Release `gorm:"foreignKey:ReleaseID;constraint:OnDelete:CASCADE"`
}

func (PackAttempt) TableName() string { return "pack_attempts" }
