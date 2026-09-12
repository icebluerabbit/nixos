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
	// ReasonPlatform is a release from a platform taken wholesale, because
	// the catalogue cannot have an opinion about one.
	ReasonPlatform = "platform"
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
type Decider struct {
	platforms map[string]bool
}

// New builds a Decider that takes releases from these platforms wholesale, on
// top of the followed performers and studios.
//
// A platform is configuration rather than a rule in the code: whether every
// OnlyFans rip is worth the disk is a judgement about disk, and it changes
// when freeleech ends.
func New(platforms ...string) *Decider {
	d := &Decider{platforms: make(map[string]bool, len(platforms))}
	for _, p := range platforms {
		d.platforms[p] = true
	}
	return d
}

// Decide returns the outcome for one announce. meta may be nil.
//
// Followed performers and studios, plus the platforms in Platforms. Being
// recognised is not itself a reason to grab: a studio the catalogue knows but
// the account does not follow is somebody else's interest.
func (d *Decider) Decide(a types.Announce, _ *torrent.Meta, m Match) Decision {
	switch {
	case m.CatalogueEmpty:
		return Decision{Accept: false, Reason: ReasonRecordOnly}
	case len(m.Performers) > 0:
		return Decision{Accept: true, Reason: ReasonFavouritePerformer}
	case len(m.Studios) > 0:
		return Decision{Accept: true, Reason: ReasonFavouriteStudio}
	}
	// Checked after the favourites so a release that is both is attributed
	// to the person or studio followed, which is the more useful reason.
	if d.platforms[Platform(a.Name)] {
		return Decision{Accept: true, Reason: ReasonPlatform}
	}
	return Decision{Accept: false, Reason: ReasonNoFavourite}
}
