package model

import (
	"testing"

	"github.com/google/uuid"
)

// TestNewIDIsVersion7 pins the id version.
//
// Worth asserting rather than assuming: the whole reason this model has no
// sequences is that a v7 id is time-ordered, so index locality follows insert
// order. A silent fall back to v4 would keep everything compiling and working
// while turning every primary-key index into random writes.
func TestNewIDIsVersion7(t *testing.T) {
	id := NewID()
	if got := id.Version(); got != 7 {
		t.Fatalf("version = %d, want 7", got)
	}
	if id == uuid.Nil {
		t.Fatal("NewID returned the nil UUID")
	}
}

// TestNewIDIsUnique guards the obvious catastrophe.
func TestNewIDIsUnique(t *testing.T) {
	const n = 10000
	seen := make(map[ID]struct{}, n)
	for range n {
		id := NewID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = struct{}{}
	}
}

// TestNewIDIsTimeOrdered checks that ids minted in sequence sort in sequence.
//
// This is the property the schema depends on. v7 encodes milliseconds, so ids
// minted inside the same millisecond may tie — the assertion is therefore
// non-decreasing rather than strictly increasing.
func TestNewIDIsTimeOrdered(t *testing.T) {
	prev := NewID()
	for range 1000 {
		next := NewID()
		if next.String() < prev.String() {
			t.Fatalf("id went backwards: %s then %s", prev, next)
		}
		prev = next
	}
}

// TestBeforeCreateFillsZeroID covers the normal path.
func TestBeforeCreateFillsZeroID(t *testing.T) {
	var b Base
	if err := b.BeforeCreate(nil); err != nil {
		t.Fatal(err)
	}
	if b.ID == uuid.Nil {
		t.Fatal("id still nil after BeforeCreate")
	}
}

// TestBeforeCreateKeepsExplicitID is what makes a re-import reproducible.
//
// The catalog pass mints ids itself so it can link a season to its series
// before either row exists. If the hook overwrote them, every child would point
// at a parent that was never inserted under that id.
func TestBeforeCreateKeepsExplicitID(t *testing.T) {
	want := NewID()
	b := Base{ID: want}
	if err := b.BeforeCreate(nil); err != nil {
		t.Fatal(err)
	}
	if b.ID != want {
		t.Fatalf("id = %s, want %s", b.ID, want)
	}
}
