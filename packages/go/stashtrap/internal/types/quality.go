package types

import (
	"regexp"
	"strconv"
	"strings"
)

// ResolutionUnknown is the height of a release whose name says nothing about
// quality. A third of BitPorn's announces are this, so it is a normal outcome
// rather than a parse failure.
const ResolutionUnknown = 0

// resolutionPatterns are tried in order, most specific first.
//
// Explicit dimensions come before bare heights: `1920x1080` contains `1080`,
// but also contains `1920`, and a bare-height pattern reading left to right
// would take the width. Named 4K-style labels come last because they are the
// loosest — `uhd` appears in group names as often as in quality tokens.
var resolutionPatterns = []struct {
	re     *regexp.Regexp
	height int
}{
	// 3840x2160, 1920x1080, 1280x720 — the height is the second number.
	{regexp.MustCompile(`\b\d{3,4}\s?[x×]\s?(\d{3,4})\b`), 0},
	// 2160p, 1080p, 720p, 480p, and the same without the p.
	{regexp.MustCompile(`\b(4320|2880|2160|1440|1080|720|576|480|360)p?\b`), 0},
	{regexp.MustCompile(`(?i)\b(4k|uhd)\b`), 2160},
	{regexp.MustCompile(`(?i)\b(fhd)\b`), 1080},
	{regexp.MustCompile(`(?i)\b(hd)\b`), 720},
	{regexp.MustCompile(`(?i)\b(sd|dvdrip|vhsrip)\b`), 480},
}

// ParseResolution reads the vertical resolution out of a release name.
//
// Name-derived only: autobrr fills ReleaseTags from IRC announce variables, so
// a Torznab feed release arrives with that field empty and the name is the
// sole signal available before the file exists.
//
// Returns ResolutionUnknown when the name claims nothing. It does not guess
// from size or codec — an inferred quality that reads like a stated one is
// worse than an honest absence, since the whole point is deciding which of
// several releases of the same scene to prefer.
func ParseResolution(name string) int {
	if name == "" {
		return ResolutionUnknown
	}
	// Separators vary and brackets abut tokens, so give every pattern word
	// boundaries to anchor on.
	padded := strings.NewReplacer(
		"[", " ", "]", " ", "(", " ", ")", " ", "_", " ", ".", " ", "-", " ",
	).Replace(name)

	for _, p := range resolutionPatterns {
		m := p.re.FindStringSubmatch(padded)
		if m == nil {
			continue
		}
		if p.height != 0 {
			return p.height
		}
		h, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		return h
	}
	return ResolutionUnknown
}
