package reconcile

import (
	"testing"
	"time"
)

// The pointer helpers encode one rule: zero means absent. Every nullable column
// in this model goes through them, so getting the rule wrong writes zeroes
// where NULL belongs across the whole schema — a stored 0 seeders reads as "no
// seeders" rather than "not reported", and the two are opposite facts.

func TestStrPtr(t *testing.T) {
	if strPtr("") != nil {
		t.Error(`strPtr("") should be nil`)
	}
	got := strPtr("x")
	if got == nil || *got != "x" {
		t.Errorf(`strPtr("x") = %v`, got)
	}
	// A string of spaces is content, not absence: trimming here would silently
	// destroy a legitimate value.
	if strPtr(" ") == nil {
		t.Error(`strPtr(" ") should not be nil`)
	}
}

func TestIntPtr(t *testing.T) {
	if intPtr(0) != nil {
		t.Error("intPtr(0) should be nil")
	}
	if got := intPtr(-1); got == nil || *got != -1 {
		t.Errorf("intPtr(-1) = %v; negatives are real values", got)
	}
}

func TestInt64Ptr(t *testing.T) {
	if int64Ptr(0) != nil {
		t.Error("int64Ptr(0) should be nil")
	}
	if got := int64Ptr(1 << 40); got == nil || *got != 1<<40 {
		t.Errorf("int64Ptr(1<<40) = %v", got)
	}
}

func TestFloatPtr(t *testing.T) {
	if floatPtr(0) != nil {
		t.Error("floatPtr(0) should be nil")
	}
	if got := floatPtr(0.5); got == nil || *got != 0.5 {
		t.Errorf("floatPtr(0.5) = %v", got)
	}
}

// TestBoolPtrKeepsFalse is the deliberate exception to the zero rule.
//
// false is a measurement: "this indexer is disabled" and "nobody said" are
// different, and collapsing them would make every disabled indexer read as
// unknown.
func TestBoolPtrKeepsFalse(t *testing.T) {
	got := boolPtr(false)
	if got == nil {
		t.Fatal("boolPtr(false) must not be nil")
	}
	if *got {
		t.Error("boolPtr(false) = true")
	}
}

func TestTimePtr(t *testing.T) {
	if timePtr(time.Time{}) != nil {
		t.Error("the zero time should be nil")
	}
	// UTC normalisation matters: every timestamp column is timestamptz and
	// mixing zones makes an ORDER BY correct but unreadable.
	local := time.Date(2026, 9, 13, 12, 0, 0, 0, time.FixedZone("x", 3600))
	got := timePtr(local)
	if got == nil {
		t.Fatal("nil for a real time")
	}
	if got.Location() != time.UTC {
		t.Errorf("location = %s, want UTC", got.Location())
	}
	if !got.Equal(local) {
		t.Errorf("instant changed: %s vs %s", got, local)
	}
}

// TestUnixTime covers the clients that report epoch seconds.
//
// qBittorrent uses a non-positive value for "never" — added_on is always set,
// completion_on is -1 until the torrent finishes — so the guard is <= 0 rather
// than == 0.
func TestUnixTime(t *testing.T) {
	for _, sec := range []int64{0, -1, -3600} {
		if got := unixTime(sec); !got.IsZero() {
			t.Errorf("unixTime(%d) = %s, want zero", sec, got)
		}
	}
	got := unixTime(1_757_800_000)
	if got.IsZero() {
		t.Fatal("a real epoch became zero")
	}
	if got.Location() != time.UTC {
		t.Errorf("location = %s, want UTC", got.Location())
	}
	if got.Unix() != 1_757_800_000 {
		t.Errorf("unix = %d", got.Unix())
	}
}
