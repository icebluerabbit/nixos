package parse

import (
	"fmt"
	"regexp"
	"strings"
)

// Date forms these releases write, and what they mean.
//
// Day-first is not a guess: "MILF Cravings (28/08/2026)" is the 2026-08-28
// scene on StashDB, and every other dated release in the corpus agrees.
var (
	dotted    = regexp.MustCompile(`^(\d{2})\.(\d{2})\.(\d{2})$`)
	dayFirst  = regexp.MustCompile(`^(\d{1,2})[./-](\d{1,2})[./-](\d{4})$`)
	yearFirst = regexp.MustCompile(`^(\d{4})[./-](\d{1,2})[./-](\d{1,2})$`)
	monthName = regexp.MustCompile(`(?i)^([a-z]{3})[a-z]*\.?\s+(\d{1,2}),?\s+(\d{4})$`)
)

var months = map[string]string{
	"jan": "01", "feb": "02", "mar": "03", "apr": "04", "may": "05", "jun": "06",
	"jul": "07", "aug": "08", "sep": "09", "oct": "10", "nov": "11", "dec": "12",
}

// ISODate converts a date as a release wrote it into the form the catalogue
// stores, or returns "" when it is not a date this recognises.
func ISODate(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case dotted.MatchString(s):
		m := dotted.FindStringSubmatch(s)
		// Two-digit years in these names are this century.
		return "20" + m[1] + "-" + m[2] + "-" + m[3]
	case yearFirst.MatchString(s):
		m := yearFirst.FindStringSubmatch(s)
		return fmt.Sprintf("%s-%02s-%02s", m[1], m[2], m[3])
	case dayFirst.MatchString(s):
		m := dayFirst.FindStringSubmatch(s)
		return fmt.Sprintf("%s-%02s-%02s", m[3], m[2], m[1])
	case monthName.MatchString(s):
		m := monthName.FindStringSubmatch(s)
		if mm, ok := months[strings.ToLower(m[1])]; ok {
			return fmt.Sprintf("%s-%s-%02s", m[3], mm, m[2])
		}
	}
	return ""
}

// sameDate reports whether a date off a release name and a catalogue release
// date are the same day.
func sameDate(written, stored string) bool {
	iso := ISODate(written)
	return iso != "" && iso == stored
}
