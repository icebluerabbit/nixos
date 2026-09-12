package policy

import (
	"regexp"
	"strings"
)

// PlatformOnlyFans is the one platform recognised from a release's own text.
//
// A platform is not a studio and will never match the catalogue: "OnlyFans" is
// an alias of seventy-five studios, which identifies none of them, and the
// performer is a handle nobody follows. So a release from one is recognised
// here or not at all — a deliberate exception to the rule that the catalogue
// decides what a release is about.
const PlatformOnlyFans = "onlyfans"

// onlyFansHeader matches the platform in the leading bracket: "[OF]",
// "[OF-LilyPhillips]", "(OF ...)".
//
// Anchored to the header on purpose. "of" is a word, and 69 of the recorded
// releases contain it — "Destruction Of Testicles", "album of beauty",
// "Double Penetration of a skinny…" — so an unanchored test would take them
// all.
var onlyFansHeader = regexp.MustCompile(`(?i)^\s*[\[(]\s*of\b[-\s\]/)]`)

// onlyFansPrefix matches the platform leading the name with no bracket at all:
// "OF SummerRose Foursome".
//
// Case-sensitive, unlike every other test here, because that is the only thing
// separating the abbreviation from the preposition: "OF SummerRose" is a
// platform and "Of Course She Did" is a title, and they are otherwise the same
// shape. An upper-case "OF" at the start of a release name is not English.
var onlyFansPrefix = regexp.MustCompile(`^OF\s+\S`)

// spelledOut finds the platform written in full, however it is punctuated:
// "OnlyFans.com", "Onlyfans com", "Onlyfans com/theivoryfox".
const spelledOut = "onlyfans"

// Platform reports the platform a release names, or "" for none.
func Platform(name string) string {
	if strings.Contains(collapse(name), spelledOut) {
		return PlatformOnlyFans
	}
	if onlyFansHeader.MatchString(name) || onlyFansPrefix.MatchString(name) {
		return PlatformOnlyFans
	}
	return ""
}

// collapse folds a name to lower-case letters and digits, so that every way a
// site writes its own name — dots, spaces, slashes — reduces to one string.
func collapse(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
