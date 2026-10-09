package inventory

import (
	"testing"

	"homelab/flac-util/internal/report"
)

// tableOf returns a table whose tracks reach the given fractions of their
// ceiling, named so that the original order can be recognised afterwards.
func tableOf(t *testing.T, reaches ...float64) *Table {
	t.Helper()

	entries := make([]Entry, len(reaches))
	for i, reach := range reaches {
		r := trackReport()
		r.Spectrum = reaching(44100, reach)
		entries[i] = currentEntry(trackName(i), r)
	}
	return NewTable(Group{Entries: entries}, Options{})
}

// reaching returns a spectrum whose content stops at the given fraction of
// Nyquist, with silence above it.
func reaching(rate int, fraction float64) *report.Spectrum {
	const bins = 101

	levels := make([]float64, bins)
	last := int(fraction * float64(bins-1))
	for i := range levels {
		levels[i] = -10.0
		if i > last {
			levels[i] = -400
		}
	}
	return spectrum(Nyquist(rate)/float64(bins-1), levels...)
}

// trackName names the nth track the way a library does.
func trackName(i int) string {
	return "/a/" + string(rune('a'+i)) + ".flac"
}

// order returns the text of one column, top to bottom.
func order(rows [][]Cell, col int) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row[col].Text
	}
	return out
}

// sameOrder compares two lists of cell text.
func sameOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortByNumericColumn(t *testing.T) {
	tbl := tableOf(t, 0.700, 0.500, 0.600)
	tbl.SortBy(column(t, "cutoff"), true)

	if got, want := order(tbl.Rows, column(t, "cutoff")), []string{"11.0k", "13.2k", "15.4k"}; !sameOrder(got, want) {
		t.Errorf("ascending = %v, want %v", got, want)
	}

	tbl.SortBy(column(t, "cutoff"), false)
	if got, want := order(tbl.Rows, column(t, "cutoff")), []string{"15.4k", "13.2k", "11.0k"}; !sameOrder(got, want) {
		t.Errorf("descending = %v, want %v", got, want)
	}
}

func TestSortKeepsRowsAndEntriesTogether(t *testing.T) {
	tbl := tableOf(t, 0.700, 0.500, 0.600)
	tbl.SortBy(column(t, "cutoff"), true)

	for i, entry := range tbl.Group.Entries {
		if got, want := tbl.Rows[i][column(t, "track")].Text, entry.Track(); got != want {
			t.Errorf("row %d names %q but its entry is %q", i, got, want)
		}
	}
}

func TestSortByTextColumn(t *testing.T) {
	tbl := tableOf(t, 0.7, 0.5, 0.6)
	tbl.SortBy(column(t, "track"), false)

	if got, want := order(tbl.Rows, column(t, "track")), []string{"c", "b", "a"}; !sameOrder(got, want) {
		t.Errorf("descending by track = %v, want %v", got, want)
	}
}

func TestSortPutsMissingValuesLastInBothDirections(t *testing.T) {
	tbl := tableOf(t, 0.700, 0.500)
	tbl.Group.Entries = append(tbl.Group.Entries, Entry{AudioPath: "/a/c.flac", State: report.StateMissing})
	tbl = NewTable(tbl.Group, Options{})

	for _, ascending := range []bool{true, false} {
		tbl.SortBy(column(t, "cutoff"), ascending)
		last := tbl.Rows[len(tbl.Rows)-1][column(t, "cutoff")].Text
		if last != Absent {
			t.Errorf("ascending=%v put %q last, want the row with no value", ascending, last)
		}
	}
}

func TestSortIsStable(t *testing.T) {
	// Equal values keep the order they arrived in, which for tracks is the
	// order they play in.
	tbl := tableOf(t, 0.5, 0.5, 0.5)
	tbl.SortBy(column(t, "cutoff"), true)

	if got, want := order(tbl.Rows, column(t, "track")), []string{"a", "b", "c"}; !sameOrder(got, want) {
		t.Errorf("order = %v, want the original %v", got, want)
	}
}

func TestSortIgnoresAColumnThatIsNotThere(t *testing.T) {
	tbl := tableOf(t, 0.7, 0.5)
	before := order(tbl.Rows, column(t, "track"))

	tbl.SortBy(-1, true)
	tbl.SortBy(len(tbl.Columns), true)

	if got := order(tbl.Rows, column(t, "track")); !sameOrder(got, before) {
		t.Errorf("order = %v, want it unchanged at %v", got, before)
	}
}

func TestSortOfNothing(t *testing.T) {
	var tbl *Table
	tbl.SortBy(0, true) // must not panic

	empty := NewTable(Group{}, Options{})
	empty.SortBy(0, true)
	if len(empty.Rows) != 0 {
		t.Errorf("an empty table grew %d rows", len(empty.Rows))
	}
}
