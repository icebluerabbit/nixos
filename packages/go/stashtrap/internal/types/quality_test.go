package types_test

import (
	"testing"

	"homelab/stashtrap/internal/types"
)

func TestParseResolution(t *testing.T) {
	t.Parallel()

	// The names are invented, but their shape is taken from the recorded
	// corpus: the bracket-and-dot soup is what the parser has to survive.
	cases := map[string]struct {
		name string
		want int
	}{
		"trailing p":       {"[SiteOne] 26.09.08 First Performer [1080p]", 1080},
		"bare height":      {"Some Release 1080 WEB", 1080},
		"2160p":            {"[SiteOne] First Performer - A Title [2160p]", 2160},
		"4k label":         {"'By Someone' Compilation 1-4 + Vertical 4K", 2160},
		"dimensions":       {"[SiteTwo] Second Performer - A Title_3840x2160", 2160},
		"dimensions 1080":  {"[SiteTwo] Second Performer - A Title 1920x1080", 1080},
		"portrait":         {"[SiteTwo] Second Performer - Vertical 1080x1920", 1920},
		"720":              {"[SiteThree] Third Performer - Title 2026 [720p] mp4", 720},
		"sd":               {"An Old Release [Group - 2008] -- Title - SD", 480},
		"none":             {"(SiteFour.example) Fourth Performer", types.ResolutionUnknown},
		"empty":            {"", types.ResolutionUnknown},
		"year is not size": {"A Feature Title (Some Director, A Studio)", types.ResolutionUnknown},
	}

	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			if got := types.ParseResolution(tc.name); got != tc.want {
				t.Errorf("ParseResolution(%q) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

// A four-digit year sits in many of these names and must not be read as a
// height — 2026 is not a resolution, and neither is 1999.
func TestParseResolutionIgnoresYears(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"[SiteOne] Fifth Performer (20.02.2020)",
		"Some Release 1999 DVD",
		"[SiteOne] Title (2026-09-06)",
	} {
		if got := types.ParseResolution(name); got != types.ResolutionUnknown {
			t.Errorf("ParseResolution(%q) = %d, want unknown", name, got)
		}
	}
}
