// Package parse takes a release name apart and asks the catalogue what the
// pieces are.
//
// Nothing here decides that a piece is a studio, a performer or a scene. These
// names are not written to a grammar that would support such a decision — a
// header holds a site on one release and a performer's handle on the next, a
// body holds a cast, a title, both, or neither. So the name is cleaned of the
// parts that are certainly not names (dates, resolutions, codecs, containers),
// partitioned into every plausible chunk, and each chunk is looked up. What the
// catalogue recognises is what the release is about.
package parse

import (
	"regexp"
	"strings"
)

// headerPattern matches the leading bracket or parenthesis.
var headerPattern = regexp.MustCompile(`^\s*[\[\(]([^\]\)]+)[\]\)]\s*`)

// dates, in the forms these releases write them. Lifted from anywhere: a date
// lands before the cast, after the title, or at the very end.
var dates = []*regexp.Regexp{
	// "26.09.09" leading the body, "2020-12-27", "(28/08/2026)"
	regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2}|\d{1,2}[./-]\d{1,2}[./-]\d{4}|\d{4}[./-]\d{1,2}[./-]\d{1,2}|\d{2}\.\d{2}\.\d{2})\b`),
	// "Aug 14, 2026", "August 14 2026"
	regexp.MustCompile(`(?i)\b((?:jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*\.?\s+\d{1,2},?\s+\d{4})\b`),
}

// technical tokens, pruned wherever they appear. None is ever part of a
// performer's name or a scene's title, so removing them everywhere — not just
// from the end — is what leaves the rest matchable.
var technical = regexp.MustCompile(
	`(?i)\b(\d{3,4}[pi]|\d{3,4}x\d{3,4}|4k|uhd|fhd|web-?dl|webrip|bdrip|dvdrip|hdrip|siterip|bluray|hevc|x26[45]|h\.?26[45]|avc|aac|mp4|mkv|avi|wmv|m4v|repack|proper)\b`)

// ambiguous tokens are technical only in a bracket or at the very end.
//
// "HD", "SD" and "XXX" are also words in real scene titles — "XXX Confessions"
// — and titles are matched exactly, so pruning one mid-title destroys the match
// it was meant to help.
var ambiguous = regexp.MustCompile(
	`(?i)[\[\(][^\]\)]*\b(hd|sd|xxx|compressed|pictures|iso|dvd9)\b[^\]\)]*[\]\)]|\s+\b(hd|sd|xxx|compressed)\b\s*$`)

// leftovers of pruning.
var (
	emptyGroups = regexp.MustCompile(`[\[\(]\s*[\]\)]`)
	runsOfSpace = regexp.MustCompile(`\s{2,}`)
	danglingSep = regexp.MustCompile(`^[\s\-–—,;:\]\)]+|[\s\-–—,;:\[\(]+$`)
	strayCommas = regexp.MustCompile(`\s*,\s*(,\s*)+`)
	spaceComma  = regexp.MustCompile(`\s+([,;])`)
)

// prune removes the technical tokens and tidies what their removal leaves.
func prune(text string) string {
	text = technical.ReplaceAllString(text, " ")
	text = ambiguous.ReplaceAllString(text, " ")
	text = emptyGroups.ReplaceAllString(text, " ")
	text = dropOrphanBrackets(text)
	text = strayCommas.ReplaceAllString(text, ", ")
	text = spaceComma.ReplaceAllString(text, "$1")
	text = runsOfSpace.ReplaceAllString(text, " ")
	return danglingSep.ReplaceAllString(strings.TrimSpace(text), "")
}

// dropOrphanBrackets removes brackets left without their partner.
//
// Pruning a token out of "…, 1080p, SiteRip]" takes the content but not the
// bracket that closed it, and a lone "]" then travels into a chunk and breaks
// the exact match it is part of.
func dropOrphanBrackets(s string) string {
	pairs := map[rune]rune{']': '[', ')': '('}
	drop := make(map[int]bool)
	var open []int

	for i, r := range s {
		switch r {
		case '[', '(':
			open = append(open, i)
		case ']', ')':
			matched := -1
			for j := len(open) - 1; j >= 0; j-- {
				if rune(s[open[j]]) == pairs[r] {
					matched = j
					break
				}
			}
			if matched < 0 {
				drop[i] = true
				continue
			}
			open = open[:matched]
		}
	}
	for _, i := range open {
		drop[i] = true
	}
	if len(drop) == 0 {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for i, r := range s {
		if !drop[i] {
			b.WriteRune(r)
		}
	}
	return b.String()
}
