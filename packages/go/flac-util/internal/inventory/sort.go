package inventory

import (
	"sort"
	"strings"
)

// SortBy orders the table's rows by one column.
//
// The marks are left alone. A cell is marked for being unlike the others in
// its column, which is a fact about the set and not about the order, so
// sorting a table cannot change what it says - only how quickly the outlier
// is found.
//
// A column index outside the table leaves it untouched, since the caller is a
// keystroke and a bad keystroke should do nothing rather than fail.
func (t *Table) SortBy(col int, ascending bool) {
	if t == nil || col < 0 || col >= len(t.Columns) {
		return
	}

	numeric := t.Columns[col].Numeric
	order := make([]int, len(t.Rows))
	for i := range order {
		order[i] = i
	}

	// A stable sort keeps rows that compare equal in the order they came in,
	// which for tracks is the order they play in.
	sort.SliceStable(order, func(a, b int) bool {
		return lessThan(t.Rows[order[a]][col], t.Rows[order[b]][col], numeric, ascending)
	})

	t.reorder(order)
}

// lessThan reports whether left sorts before right.
//
// Cells holding no value sort last in either direction: a row with nothing
// measured is not an extreme, it is a gap, and burying it at the bottom keeps
// it from being read as one.
func lessThan(left, right Cell, numeric, ascending bool) bool {
	if numeric {
		if left.HasValue != right.HasValue {
			return left.HasValue
		}
		if !left.HasValue || left.Value == right.Value {
			return false
		}
		return (left.Value < right.Value) == ascending
	}

	c := strings.Compare(left.Text, right.Text)
	if c == 0 {
		return false
	}
	return (c < 0) == ascending
}

// reorder applies a permutation to the rows and to the entries behind them, so
// that the two stay in step.
func (t *Table) reorder(order []int) {
	rows := make([][]Cell, len(order))
	entries := make([]Entry, len(order))
	for i, from := range order {
		rows[i] = t.Rows[from]
		entries[i] = t.Group.Entries[from]
	}
	t.Rows = rows
	t.Group.Entries = entries
}
