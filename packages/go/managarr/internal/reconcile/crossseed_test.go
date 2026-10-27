package reconcile

import (
	"testing"

	"homelab/managarr/internal/model"
)

// TestCrossSeedOutcome pins the mapping of cross-seed's decision vocabulary.
//
// Everything that is not MATCH is a rejection: cross-seed looked at the
// candidate and declined it. None of its outcomes are errors — a failed fetch
// is autobrr's push decision, not this.
func TestCrossSeedOutcome(t *testing.T) {
	cases := map[string]string{
		"MATCH":                    model.OutcomeAccepted,
		"SIZE_MISMATCH":            model.OutcomeRejected,
		"FILE_TREE_MISMATCH":       model.OutcomeRejected,
		"INFO_HASH_ALREADY_EXISTS": model.OutcomeRejected,
		"SAME_INFO_HASH":           model.OutcomeRejected,
		"":                         model.OutcomeRejected,
	}
	for decision, want := range cases {
		if got := crossSeedOutcome(decision); got != want {
			t.Errorf("crossSeedOutcome(%q) = %q, want %q", decision, got, want)
		}
	}
}

// The snapshot tests that used to live here moved to crossseed-api, along with
// the reading itself. Copying a live WAL database is the sidecar's problem now;
// this side only sees JSON.
