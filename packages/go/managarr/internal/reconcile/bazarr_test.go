package reconcile

import (
	"testing"
	"time"
)

func TestParseScore(t *testing.T) {
	cases := []struct {
		in   string
		want *float64
	}{
		{"", nil},
		{"  ", nil},
		{"n/a", nil},
		{"98.5", f(98.5)},
		{"100%", f(100)},
		{" 72 % ", nil}, // a space before the % is not a shape Bazarr emits
		{"0", f(0)},
	}
	for _, c := range cases {
		got := parseScore(c.in)
		switch {
		case c.want == nil && got != nil:
			t.Errorf("parseScore(%q) = %v, want nil", c.in, *got)
		case c.want != nil && got == nil:
			t.Errorf("parseScore(%q) = nil, want %v", c.in, *c.want)
		case c.want != nil && *got != *c.want:
			t.Errorf("parseScore(%q) = %v, want %v", c.in, *got, *c.want)
		}
	}
}

func f(v float64) *float64 { return &v }

// TestParseBazarrTime covers a rendered timestamp.
//
// Bazarr formats parsed_timestamp for display rather than serialising it, so
// the layout follows the instance's date setting. An unparseable value must be
// NULL: storing a wrong time is worse than storing none, because a downloaded_at
// nobody can trust is a column nobody can use.
func TestParseBazarrTime(t *testing.T) {
	if parseBazarrTime("") != nil {
		t.Error("empty should be nil")
	}
	if parseBazarrTime("2 days ago") != nil {
		t.Error("Bazarr's relative rendering should be nil, not a guess")
	}

	got := parseBazarrTime("2026-09-13 14:05:06")
	if got == nil {
		t.Fatal("ISO-ish layout did not parse")
	}
	want := time.Date(2026, 9, 13, 14, 5, 6, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %s, want %s", got, want)
	}
	if got.Location() != time.UTC {
		t.Errorf("location = %s, want UTC", got.Location())
	}

	// The US-style layout Bazarr defaults to.
	if parseBazarrTime("09/13/26 14:05:06") == nil {
		t.Error("MM/DD/YY layout did not parse")
	}
	// RFC 3339, which newer builds send.
	if parseBazarrTime("2026-09-13T14:05:06Z") == nil {
		t.Error("RFC 3339 did not parse")
	}
}

func TestDeref(t *testing.T) {
	if deref(nil) != "" {
		t.Error("nil should read as empty")
	}
	s := "x"
	if deref(&s) != "x" {
		t.Error("value not returned")
	}
}
