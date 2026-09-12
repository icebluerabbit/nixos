package policy_test

import (
	"testing"

	"homelab/stashtrap/internal/policy"
)

func TestPlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		// Spelled out, however the site punctuates itself.
		{"dot com", "[OnlyFans.com] A Title", policy.PlatformOnlyFans},
		{"spaced", "[Onlyfans com] Somebody - A Title", policy.PlatformOnlyFans},
		{"with a handle", "[Onlyfans com/thehandle] Somebody Does Something", policy.PlatformOnlyFans},
		{"bare word", "[OnlyFans] Somebody - A Title", policy.PlatformOnlyFans},

		// Abbreviated, which is only safe anchored to the header.
		{"abbreviated with handle", "[OF-SomeHandle] A Title [1080p] mp4", policy.PlatformOnlyFans},
		{"abbreviated alone", "[OF] Somebody x Another - A Title", policy.PlatformOnlyFans},
		{"abbreviated leading", "OF SomeHandle Foursome 1080p", policy.PlatformOnlyFans},

		// "of" is a word. Sixty-nine of the recorded releases contain it, and
		// taking them would be taking most of the tracker.
		{"of mid-title", "BrutalBusting - Destruction Of Testicles By Hard Kicking", ""},
		{"of lower mid-title", "2026.09.07 Intimate album of beauty [Pictures]", ""},
		{"of inside a bracket", "[SomeSite] Double Penetration of a skinny young performer", ""},
		{"of starting a title", "Of Course She Did - A Title", ""},
		{"unrelated", "[FirstSite] Jane Doe - A Title 1080p", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := policy.Platform(tc.in); got != tc.want {
				t.Fatalf("Platform(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A platform is accepted only when it was asked for, and a release that is
// both a platform and a favourite is attributed to the favourite.
func TestDecidePlatform(t *testing.T) {
	t.Parallel()

	name := "[OnlyFans.com] Somebody - A Title"

	if got := policy.New().Decide(announce(name), nil, policy.Match{}); got.Accept {
		t.Fatalf("a platform was taken without being configured: %+v", got)
	}

	enabled := policy.New(policy.PlatformOnlyFans)
	got := enabled.Decide(announce(name), nil, policy.Match{})
	if !got.Accept || got.Reason != policy.ReasonPlatform {
		t.Fatalf("Decide = %+v, want an accept for %q", got, policy.ReasonPlatform)
	}

	// The performer is the more useful reason when a release is both.
	both := enabled.Decide(announce(name), nil, policy.Match{Performers: []string{"p1"}})
	if both.Reason != policy.ReasonFavouritePerformer {
		t.Fatalf("Reason = %q, want the favourite to be credited", both.Reason)
	}

	// An empty catalogue still refuses everything: a platform rule does not
	// make a service that cannot answer start answering.
	empty := enabled.Decide(announce(name), nil, policy.Match{CatalogueEmpty: true})
	if empty.Accept || empty.Reason != policy.ReasonRecordOnly {
		t.Fatalf("Decide = %+v, want %q", empty, policy.ReasonRecordOnly)
	}
}
