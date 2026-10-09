package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

// testColumns are a small stand-in for the real table.
func testColumns() []column {
	return []column{
		{Title: "track", Width: 12},
		{Title: "cutoff", Width: 7, Numeric: true},
	}
}

// cells returns a row of plain cells.
func cells(texts ...string) []cell {
	row := make([]cell, len(texts))
	for i, text := range texts {
		row[i] = cell{Text: text}
	}
	return row
}

// styler returns a styler over the given rows.
func styler(rows ...[]cell) tableStyler {
	return newTableStyler(NewTheme(), testColumns(), rows)
}

func TestTableStylerSizesColumnsToTheirContent(t *testing.T) {
	s := styler(cells("01 - a", "22.1k"), cells("02 - bb", "19.5k"))

	// The widest track is 7 cells; the column asked for 12 but only needs 7.
	if got, want := s.widths[0], 7; got != want {
		t.Errorf("track width = %d, want %d", got, want)
	}
	// No value is wider than the title, so the title sets the width.
	if got, want := s.widths[1], len("cutoff"); got != want {
		t.Errorf("cutoff width = %d, want %d", got, want)
	}
}

func TestTableStylerNeverExceedsTheColumnWidth(t *testing.T) {
	row := cells("a track name far longer than the column", "22.1k")
	s := styler(row)

	if got, want := s.widths[0], 12; got != want {
		t.Errorf("track width = %d, want the column's %d", got, want)
	}
	if got, want := lipgloss.Width(s.plainRow(row)), 12+len(columnGap)+6; got != want {
		t.Errorf("row is %d wide, want %d", got, want)
	}
}

func TestTableStylerAlignsNumbersRightAndTextLeft(t *testing.T) {
	// Digits that do not line up cannot be read down, which is the only thing
	// a column of numbers is for.
	s := styler(cells("01 - a", "22.1k"), cells("02 - bb", "9.5k"))

	if got, want := s.align("22.1k", 1), " 22.1k"; got != want {
		t.Errorf("align(numeric) = %q, want %q", got, want)
	}
	if got, want := s.align("01 - a", 0), "01 - a "; got != want {
		t.Errorf("align(text) = %q, want %q", got, want)
	}
}

func TestTableStylerTruncatesWithAMark(t *testing.T) {
	s := styler(cells("a track name far longer than the column", "22.1k"))
	got := s.align("a track name far longer than the column", 0)

	if runewidth.StringWidth(got) != 12 {
		t.Fatalf("align() produced %q, %d cells wide", got, runewidth.StringWidth(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("align() = %q, want it to end in an ellipsis so the cut is visible", got)
	}
}

func TestTableStylerCountsWideCharactersAsTwoCells(t *testing.T) {
	// Korean track names are the ordinary case in this library, and a name
	// measured by rune count instead of display width breaks every column
	// after it.
	rows := [][]cell{cells("클락션", "22.1k"), cells("Bloom", "22.1k")}
	s := newTableStyler(NewTheme(), testColumns(), rows)

	if got, want := s.widths[0], runewidth.StringWidth("클락션"); got != want {
		t.Errorf("track width = %d, want %d", got, want)
	}
	want := lipgloss.Width(s.plainRow(rows[0]))
	for i, row := range rows {
		if got := lipgloss.Width(s.plainRow(row)); got != want {
			t.Errorf("row %d is %d wide, want %d", i, got, want)
		}
	}
}

func TestTableStylerHeaderMatchesTheRows(t *testing.T) {
	row := cells("01 - a", "22.1k")
	s := styler(row)

	if got, want := lipgloss.Width(s.plainHeader()), lipgloss.Width(s.plainRow(row)); got != want {
		t.Errorf("header is %d wide and its rows are %d; the columns would not line up", got, want)
	}
}

// TestTableStylerColumnsCarryTheirOwnPadding covers the reason the widget's
// padding is turned off: a selected row is coloured cell by cell, and padding
// added around a coloured cell afterwards would be left unpainted.
func TestTableStylerColumnsCarryTheirOwnPadding(t *testing.T) {
	s := styler(cells("01 - a", "22.1k"))
	columns := s.tableColumns()

	if len(columns) != len(testColumns()) {
		t.Fatalf("%d columns, want %d", len(columns), len(testColumns()))
	}
	for j, column := range columns {
		if got, want := column.Width, s.widths[j]+2*cellPadding; got != want {
			t.Errorf("column %d is %d wide, want %d", j, got, want)
		}
		if got := runewidth.StringWidth(column.Title); got != column.Width {
			t.Errorf("column %d's title is %d wide against a column of %d", j, got, column.Width)
		}
	}
}

func TestTableStylerHighlightsOnlyTheRowAtTheCursor(t *testing.T) {
	rows := [][]cell{cells("01 - a", "22.1k"), cells("02 - b", "19.5k"), cells("03 - c", "16.0k")}
	s := newTableStyler(NewTheme(), testColumns(), rows)

	plain := s.tableRows(rows, -1)
	for cursor := range rows {
		built := s.tableRows(rows, cursor)
		for i, row := range built {
			highlighted := row[0] != plain[i][0]
			if highlighted != (i == cursor) {
				t.Errorf("with the cursor on %d, row %d highlighted = %v", cursor, i, highlighted)
			}
		}
	}
}

// TestTableStylerHighlightsEveryCellOfTheRow checks the highlight is applied
// per cell. Wrapped around the whole row it would stop at the first reset and
// paint a fraction of the line.
func TestTableStylerHighlightsEveryCellOfTheRow(t *testing.T) {
	rows := [][]cell{cells("01 - a", "22.1k")}
	s := newTableStyler(NewTheme(), testColumns(), rows)

	plain, selected := s.tableRows(rows, -1)[0], s.tableRows(rows, 0)[0]
	for j := range selected {
		if selected[j] == plain[j] {
			t.Errorf("cell %d is unchanged when its row is selected", j)
		}
	}
}

func TestTableStylerWidthCoversThePadding(t *testing.T) {
	s := styler(cells("01 - a", "22.1k"))

	want := 0
	for _, w := range s.widths {
		want += w + 2*cellPadding
	}
	if got := s.width(); got != want {
		t.Errorf("width() = %d, want %d", got, want)
	}
}

func TestTableStylerHandlesAShortRow(t *testing.T) {
	s := styler(cells("01 - a", "22.1k"))
	full := lipgloss.Width(s.plainRow(cells("01 - a", "22.1k")))

	if got := lipgloss.Width(s.plainRow(cells("only one"))); got != full {
		t.Errorf("a row missing a cell rendered %d wide, want %d", got, full)
	}
}

func TestTableStylerLeavesTheWidgetsOwnStylingOff(t *testing.T) {
	// Padding and selection are both done cell by cell. The widget's padding
	// would leave unpainted gaps through a highlighted row, and its highlight
	// would stop at the first cell's reset.
	s := styler(cells("01 - a", "22.1k"))
	styles := s.tableStyles()

	if styles.Selected.Render("x") != styles.Cell.Render("x") {
		t.Error("the widget's selected style differs from its cell style; it would half-paint the row")
	}
	if got := styles.Cell.Render("x"); got != "x" {
		t.Errorf("the widget's cell style renders %q, want the value untouched", got)
	}
}

func TestTableStylerFallsBackToTheThemeWhenACellHasNoStyle(t *testing.T) {
	theme := NewTheme()
	s := newTableStyler(theme, testColumns(), nil)

	if got, want := s.styleOf(cell{}, 1).Render("x"), theme.Label.Render("x"); got != want {
		t.Errorf("an unstyled cell rendered %q, want %q", got, want)
	}
}

func TestTableStylerKeepsACellsOwnStyle(t *testing.T) {
	theme := NewTheme()
	s := newTableStyler(theme, testColumns(), nil)

	if got, want := s.styleOf(styled("", theme.Bad), 1).Render("x"), theme.Bad.Render("x"); got != want {
		t.Errorf("a styled cell rendered %q, want %q", got, want)
	}
}

// TestTableStylerBoldsThePrimaryColumn checks the column that names the row is
// set apart from the values beside it, whatever colour the cell carries.
func TestTableStylerBoldsThePrimaryColumn(t *testing.T) {
	theme := NewTheme()
	columns := []column{{Title: "name", Width: 8, Primary: true}, {Title: "value", Width: 8}}
	s := newTableStyler(theme, columns, nil)

	if !s.styleOf(cell{}, 0).GetBold() {
		t.Error("the primary column is not bold")
	}
	if s.styleOf(cell{}, 1).GetBold() {
		t.Error("a column that only measures is bold")
	}
	if !s.styleOf(styled("", theme.Bad), 0).GetBold() {
		t.Error("the primary column loses its weight when the cell carries a colour")
	}
}

// TestTableStylerHighlightKeepsACellsColour checks selecting a row does not
// throw away the colour that made it worth looking at.
func TestTableStylerHighlightKeepsACellsColour(t *testing.T) {
	theme := NewTheme()
	rows := [][]cell{{{Text: "01 - a"}, styled("16.0k", theme.Bands[0])}}
	s := newTableStyler(theme, testColumns(), rows)

	selected := s.tableRows(rows, 0)[0]
	want := theme.Bands[0].Background(theme.SelectedBackground).Render(pad(s.align("16.0k", 1), s.widths[1]))

	if selected[1] != want {
		t.Errorf("the coloured cell rendered %q, want %q", selected[1], want)
	}
}

// TestNewTableFitsItsRowsExactly guards the height arithmetic. The widget
// draws its column header and rule outside the height it is given and keeps a
// line of that height for itself, so getting this wrong leaves blank rows
// under the last one - which reads as missing data rather than as spare room.
func TestNewTableFitsItsRowsExactly(t *testing.T) {
	for _, n := range []int{1, 3, 10} {
		rows := make([][]cell, n)
		for i := range rows {
			rows[i] = cells("track", "22.1k")
		}
		s := newTableStyler(NewTheme(), testColumns(), rows)

		rendered := s.newTable(rows, n+1, false).View()
		lines := strings.Split(rendered, "\n")

		// The column header and the rule beneath it, then one line per row.
		if got, want := len(lines), n+2; got != want {
			t.Errorf("%d rows drew %d lines, want %d", n, got, want)
		}
		if strings.TrimSpace(lines[len(lines)-1]) == "" {
			t.Errorf("%d rows left a blank line at the bottom", n)
		}
	}
}

func TestNewTableSelectsNothingUntilAsked(t *testing.T) {
	// A table nobody is navigating must not draw a highlight, or a static
	// readout looks like something with a cursor in it.
	rows := [][]cell{cells("a", "1"), cells("b", "2")}
	s := newTableStyler(NewTheme(), testColumns(), rows)

	unfocused := s.newTable(rows, len(rows)+1, false).View()
	plainRows := s.tableRows(rows, cursorNone)
	for _, row := range plainRows {
		for _, field := range row {
			if !strings.Contains(unfocused, field) {
				t.Errorf("a row was drawn differently from its unselected form")
			}
		}
	}
}

func TestFixedTableStylerIgnoresContent(t *testing.T) {
	s := newFixedTableStyler(NewTheme(), testColumns())
	for j, c := range testColumns() {
		if s.widths[j] != c.Width {
			t.Errorf("column %d width = %d, want the declared %d", j, s.widths[j], c.Width)
		}
	}
}

func TestFixedTableStylerWidthDoesNotMoveWithTheValues(t *testing.T) {
	// The same columns holding short values and then long ones must lay out
	// identically, which is what keeps a table redrawn mid-run from shifting.
	s := newFixedTableStyler(NewTheme(), testColumns())
	short := s.plainRow(cells("a", "0s"))
	long := s.plainRow(cells("a much longer track name", "3m30.514s"))

	if a, b := lipgloss.Width(short), lipgloss.Width(long); a != b {
		t.Errorf("row widths %d and %d differ; a fixed table must not move", a, b)
	}
}

func TestFixedTableStylerKeepsTheTitleWhenItIsWiderThanTheColumn(t *testing.T) {
	columns := []column{{Title: "cutoff", Width: 2}}
	s := newFixedTableStyler(NewTheme(), columns)

	if s.widths[0] != runewidth.StringWidth("cutoff") {
		t.Fatalf("width = %d, want room for the title", s.widths[0])
	}
	if got := s.plainHeader(); !strings.Contains(got, "cutoff") {
		t.Errorf("plainHeader() = %q, want the title intact", got)
	}
}

func TestFittedTableStylerStillShrinksToContent(t *testing.T) {
	// The fitting styler is what the static tables use, and must keep fitting.
	s := styler(cells("a", "0s"))
	if s.widths[0] >= 12 {
		t.Errorf("width = %d, want it fitted below the declared 12", s.widths[0])
	}
}
