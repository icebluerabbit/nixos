package model

import "gorm.io/gorm"

import "time"

// Stages at which an offer is judged.
const (
	// StageFilter is the gatekeeper's own verdict — autobrr deciding whether
	// a release matches a filter at all.
	StageFilter = "filter"
	// StagePush is a downstream target's verdict on something that passed:
	// the *arr, cross-seed or seasonpackarr saying what it did with it.
	StagePush = "push"
)

// Outcomes. Three, and deliberately not a boolean.
//
// Rejected means the decider looked and said no — "Unknown Series", "already
// have this", "score below cutoff", SIZE_MISMATCH. Errored means the decision
// never happened: the push failed, the target was unreachable, a template did
// not render. The first is a judgement about the release; the second is a fault
// in this stack. Collapsing them makes a broken integration read as policy.
const (
	OutcomeAccepted = "accepted"
	OutcomeRejected = "rejected"
	OutcomeErrored  = "errored"
)

// Decision is one verdict on one offer.
//
// An offer collects several: autobrr's filter says yes, then each configured
// target says what it did. Together they are the whole answer to "why did this
// release never reach my library" — which exists nowhere else, because every
// service records only what it accepted.
//
// Decider is the INSTANCE name, not the kind: two Sonarr instances can judge
// the same offer differently, and a decision that cannot say which one spoke is
// not a record of anything.
//
// Keyed and updated in place rather than appended, because re-reading a
// service's history reports the same verdicts again.
type Decision struct {
	OfferID ID     `gorm:"type:uuid;primaryKey"`
	Decider string `gorm:"primaryKey"`
	Stage   string `gorm:"primaryKey"`

	Outcome string `gorm:"not null;index"`
	// Status is the decider's own vocabulary, verbatim, so a value this build
	// does not recognise is still recorded rather than flattened away.
	Status *string
	Reason *string
	// FilterName is set at filter stage: which filter matched.
	FilterName *string
	// ClientName is set when the decision routed the release to a client.
	ClientName *string
	DecidedAt  *time.Time

	Offer  *Offer  `gorm:"foreignKey:OfferID;constraint:OnDelete:CASCADE"`
	Filter *Filter `gorm:"foreignKey:FilterName;references:Name"`
	Client *Client `gorm:"foreignKey:ClientName;references:Name"`
}

func (Decision) TableName() string { return "decisions" }

// BeforeSave strips URLs quoted back inside a rejection reason.
//
// autobrr's rejections and an *arr's push errors both quote the release's
// download link when explaining why it failed.
func (d *Decision) BeforeSave(*gorm.DB) error {
	redactText(d.Reason)
	return nil
}
