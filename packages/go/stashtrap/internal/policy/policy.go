// Package policy decides what to do with an announce.
//
// It is a pure function of the announce, the parsed torrent and what the
// catalogue recognised: no I/O, no clock, no database. That is what makes it
// testable, and it is also what lets a rule set be replayed over the recorded
// corpus to see what it would have grabbed before it is enabled.
package policy

import (
	"homelab/stashtrap/internal/torrent"
	"homelab/stashtrap/internal/types"
)

// Reasons a release was accepted or rejected. A small closed set, since each
// one is stored on every row and carried as a metrics label.
const (
	// ReasonFavouritePerformer is a release naming a followed performer.
	ReasonFavouritePerformer = "favourite-performer"
	// ReasonFavouriteStudio is a release naming a followed studio.
	ReasonFavouriteStudio = "favourite-studio"
	// ReasonNoFavourite is everything else: recognised or not, it features
	// nobody followed.
	ReasonNoFavourite = "no-favourite"
	// ReasonRecordOnly is the decision when the catalogue is empty, which
	// means a sync has not run. Rejecting for want of data must be
	// distinguishable from rejecting on the merits.
	ReasonRecordOnly = "record-only"
)

// Match is what the catalogue recognised in a release, reduced to the part a
// decision turns on.
type Match struct {
	// Performers and Studios are the followed entities the release names.
	Performers []string
	Studios    []string
	// CatalogueEmpty reports that nothing has been synced yet, so an absence
	// of favourites means nothing.
	CatalogueEmpty bool
}

// Decision is the outcome for one announce.
type Decision struct {
	// Accept reports whether autobrr should proceed with the release.
	Accept bool
	// Reason names why, in both directions.
	Reason string
}

// Decider evaluates announces.
type Decider struct{}

// New builds a Decider.
func New() *Decider { return &Decider{} }

// Decide returns the outcome for one announce. meta may be nil.
//
// Followed performers and studios only. A release is worth a grab because
// somebody wanted it, and nothing else in the name — not the studio being
// recognised, not the scene being identified — is evidence of that.
func (d *Decider) Decide(_ types.Announce, _ *torrent.Meta, m Match) Decision {
	switch {
	case m.CatalogueEmpty:
		return Decision{Accept: false, Reason: ReasonRecordOnly}
	case len(m.Performers) > 0:
		return Decision{Accept: true, Reason: ReasonFavouritePerformer}
	case len(m.Studios) > 0:
		return Decision{Accept: true, Reason: ReasonFavouriteStudio}
	default:
		return Decision{Accept: false, Reason: ReasonNoFavourite}
	}
}
