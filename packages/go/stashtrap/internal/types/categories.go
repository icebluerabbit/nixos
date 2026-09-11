package types

import (
	"strconv"
	"strings"
)

// Category is one tracker category: its own numeric id and the name the
// indexer's caps give it.
type Category struct {
	ID   int
	Name string
}

// ParseCategories de-interleaves autobrr's Categories field.
//
// internal/feed/torznab.go appends each category as the pair
// [Name, strconv.Itoa(ID)], so the slice is positional: even indices are
// names, odd indices are ids. BitPorn always sends exactly two categories —
// the constant standard bucket ["XXX","6000"] plus one site-specific pair —
// but nothing here assumes that count.
//
// A pair whose id is not a number is dropped rather than guessed at: the
// position is the only thing that identifies which half is which, so a
// malformed slice cannot be realigned, only discarded.
//
// Names are trimmed of stray quotes. Prowlarr ships a BitPorn definition whose
// every category description carries an unbalanced double quote — the YAML
// reads `- {id: 20, cat: XXX, desc: Fansite"}`, so the quote is part of the
// value rather than a parser artefact. It is an upstream defect, but it
// arrives on every announce, and a category label is a metrics dimension and a
// UI string; carrying it through would make `Fansite"` the name everything
// downstream knows.
func ParseCategories(raw []string) []Category {
	out := make([]Category, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		id, err := strconv.Atoi(raw[i+1])
		if err != nil {
			continue
		}
		out = append(out, Category{ID: id, Name: cleanCategoryName(raw[i])})
	}
	return out
}

// cleanCategoryName strips the quotes and surrounding space an indexer
// definition may leave on a category description.
//
// Both ends, not just the trailing one: a definition that mismatched its
// quoting on one side can as easily do it on the other, and a name legitimately
// containing a quote at its edge does not exist.
func cleanCategoryName(name string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(name), `"'`))
}
