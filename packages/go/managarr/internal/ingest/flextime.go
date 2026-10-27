package ingest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// flexTime is a timestamp in whichever shape the sender used.
//
// JSON has no timestamp type, so every sender picks a rendering and none of
// them agree. unpackerr hands Go's time.Time straight to its template, which
// produces String() output — "2026-09-15 00:53:00.578075324 -0400 EDT
// m=+180.010956413" — rather than RFC3339. encoding/json only accepts RFC3339,
// so a plain time.Time field fails the whole notification on that one value and
// every unpackerr event is dropped.
//
// Tolerant on the way in, exact on the way out: whatever arrives is parsed if
// it can be, and anything unrecognised leaves the zero time rather than failing
// the event. A missing extraction timestamp is worth far less than the
// extraction record it would have taken down with it.
type flexTime struct {
	time.Time
}

// flexLayouts are the renderings observed from real senders, most specific
// first. Go's String() format has to precede RFC3339 because neither is a
// prefix of the other and trying them in the wrong order costs only a failed
// parse, but keeping the list ordered by how often each is seen keeps the
// common case to one attempt.
var flexLayouts = []string{
	// unpackerr: Go's time.Time.String(), monotonic suffix already stripped.
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05 -0700 MST",
	time.RFC3339Nano,
	time.RFC3339,
	// Bare local timestamps, seen from shell hooks that echo `date`.
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
}

// UnmarshalJSON accepts a quoted string in any known layout, a JSON null, or a
// numeric epoch.
func (f *flexTime) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == `""` || s == "" {
		return nil
	}

	// Epoch seconds, which qBittorrent-style senders use.
	if s[0] != '"' {
		var secs int64
		if err := json.Unmarshal(b, &secs); err != nil {
			return fmt.Errorf("flextime: %s is neither a string nor a number", s)
		}
		if secs > 0 {
			f.Time = time.Unix(secs, 0).UTC()
		}
		return nil
	}

	var raw string
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("flextime: %w", err)
	}
	f.Time = parseFlexTime(raw)
	return nil
}

// parseFlexTime returns the instant a rendering denotes, or the zero time.
func parseFlexTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}

	// Go's String() appends the monotonic clock reading, which is meaningless
	// outside the process that produced it and parses in no layout.
	if i := strings.Index(raw, " m=+"); i >= 0 {
		raw = raw[:i]
	}
	if i := strings.Index(raw, " m=-"); i >= 0 {
		raw = raw[:i]
	}

	for _, layout := range flexLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
