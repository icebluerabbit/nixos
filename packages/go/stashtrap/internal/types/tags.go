package types

import (
	"encoding/json"
	"strings"
)

// parseTags accepts either a JSON array or a delimited string.
//
// Trackers announce tags in both shapes and autobrr passes on whichever the
// definition produced, so the decoder normalises rather than choosing. An
// unparseable value yields no tags: the announce is still worth recording.
func parseTags(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return clean(list)
	}

	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	// Commas are the common separator; pipes appear on some Cardigann
	// definitions. Splitting on both costs nothing and stops "anal|teen"
	// arriving as a single tag.
	return clean(strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '|'
	}))
}

// clean trims each element and drops the empties a trailing separator leaves.
func clean(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
