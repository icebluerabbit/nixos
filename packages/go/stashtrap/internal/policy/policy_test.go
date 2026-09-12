package policy_test

import (
	"testing"

	"homelab/stashtrap/internal/policy"
	"homelab/stashtrap/internal/types"
)

// announce builds the release under test. Only the name matters to these
// rules; the rest is what autobrr always sends.
func announce(name string) types.Announce {
	return types.Announce{Name: name, Indexer: "bitporn"}
}

func TestDecide(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		match  policy.Match
		accept bool
		reason string
	}{
		{
			name:   "a followed performer is the reason to grab",
			match:  policy.Match{Performers: []string{"p1"}},
			accept: true,
			reason: policy.ReasonFavouritePerformer,
		},
		{
			name:   "a followed studio counts too",
			match:  policy.Match{Studios: []string{"s1"}},
			accept: true,
			reason: policy.ReasonFavouriteStudio,
		},
		{
			name:   "recognising nobody followed is a reject",
			match:  policy.Match{},
			accept: false,
			reason: policy.ReasonNoFavourite,
		},
		{
			// Before a sync there is nothing to recognise anyone against, and
			// rejecting for want of data must not read as rejecting on the
			// merits — otherwise an empty catalogue looks like a working one.
			name:   "an unsynced catalogue rejects for its own reason",
			match:  policy.Match{CatalogueEmpty: true},
			accept: false,
			reason: policy.ReasonRecordOnly,
		},
		{
			name:   "an unsynced catalogue cannot accept",
			match:  policy.Match{CatalogueEmpty: true, Performers: []string{"p1"}},
			accept: false,
			reason: policy.ReasonRecordOnly,
		},
	}

	d := policy.New()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := d.Decide(announce("release"), nil, tc.match)
			if got.Accept != tc.accept || got.Reason != tc.reason {
				t.Fatalf("Decide = %+v, want accept %v reason %q", got, tc.accept, tc.reason)
			}
		})
	}
}
