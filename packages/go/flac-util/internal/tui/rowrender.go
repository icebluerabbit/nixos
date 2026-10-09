package tui

import (
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

const (
	// cellPadding is the space either side of every value.
	cellPadding = 1

	// columnGap separates one column from the next when there is no widget to
	// pad them.
	columnGap = "  "
)

// column describes one column of a table drawn by this package.
type column struct {
	// Title is the header.
	Title string
	// Width is the most the values need, ignoring any styling.
	Width int
	// Numeric sets the values to the right, so their digits line up.
	Numeric bool
	// Primary marks the column that names the row rather than measuring it.
	Primary bool
}

// cell is one value, already decided how it should look.
//
// The style travels with the value rather than being worked out here, because
// what a colour means belongs to whoever knows what the number is: a spectral
// band and a queue depth are coloured for entirely different reasons, and the
// thing that draws them has no business knowing either.
type cell struct {
	Text string
	// Style is nil for a value with nothing to say about itself, which the
	// table draws in the ordinary colour. A pointer rather than a zero Style,
	// because lipgloss gives no reliable way to ask whether a style was ever
	// set - an unset one answers its getters much as a set one does.
	Style *lipgloss.Style
}

// styled returns a cell drawn in a particular style.
func styled(text string, style lipgloss.Style) cell {
	return cell{Text: text, Style: &style}
}

// tableStyler lays cells out into fixed-width columns.
//
// Cells are styled before they reach the table widget. That works because the
// widget measures a value with an escape-aware function, so a colour costs
// nothing in width.
type tableStyler struct {
	theme   Theme
	columns []column
	widths  []int
}

// newTableStyler measures the columns against the rows they have to hold.
//
// A column is as wide as its widest value, up to the width the column asked
// for, and never narrower than its own title. Fitting the columns to what is
// actually in them keeps a table from reserving room for values that never
// arrive, and is right for a table drawn once from rows that are already all
// known.
//
// It is wrong for a table that is redrawn as its values change: see
// newFixedTableStyler.
func newTableStyler(theme Theme, columns []column, rows [][]cell) tableStyler {
	widths := make([]int, len(columns))
	for j, c := range columns {
		widest := runewidth.StringWidth(c.Title)
		for _, row := range rows {
			if j < len(row) {
				widest = max(widest, runewidth.StringWidth(row[j].Text))
			}
		}
		widths[j] = min(widest, c.Width)
	}
	return tableStyler{theme: theme, columns: columns, widths: widths}
}

// newFixedTableStyler gives every column the width it asked for, whatever the
// rows currently hold.
//
// A table whose values change while it is on screen must not change shape as
// they do. Fitting the columns to the current values would: a run's elapsed
// times grow from "0s" to "3m30.514s" as it goes, and every widening moves
// each following column along the line. The renderer repaints only the lines
// that changed, so the rows that happened not to change keep the old geometry
// and the table is left holding two different layouts at once.
//
// The width a column asks for is therefore treated as what it reserves rather
// than as a limit, and the table costs a few blank columns early in a run in
// exchange for never moving.
func newFixedTableStyler(theme Theme, columns []column) tableStyler {
	widths := make([]int, len(columns))
	for j, c := range columns {
		widths[j] = max(c.Width, runewidth.StringWidth(c.Title))
	}
	return tableStyler{theme: theme, columns: columns, widths: widths}
}

// tableColumns returns the widget's columns.
//
// Each is wide enough for the value plus the space either side of it, because
// that padding is drawn here rather than by the widget: a selected row is
// coloured cell by cell, and padding the widget added afterwards would show as
// unpainted gaps between the columns.
func (s tableStyler) tableColumns() []table.Column {
	out := make([]table.Column, len(s.columns))
	for j, c := range s.columns {
		out[j] = table.Column{Title: pad(c.Title, s.widths[j]), Width: s.widths[j] + 2*cellPadding}
	}
	return out
}

// tableRows returns the widget's rows, each cell styled and aligned, with the
// row at cursor picked out. A cursor outside the rows highlights none of them.
func (s tableStyler) tableRows(rows [][]cell, cursor int) []table.Row {
	out := make([]table.Row, len(rows))
	for i, row := range rows {
		fields := make(table.Row, len(s.columns))
		for j := range s.columns {
			fields[j] = s.cellAt(cellAt(row, j), j, i == cursor)
		}
		out[i] = fields
	}
	return out
}

// cellAt returns a row's cell for a column, or an empty one when the row is
// short.
func cellAt(row []cell, col int) cell {
	if col < len(row) {
		return row[col]
	}
	return cell{}
}

// pad puts a space either side of a value, the way the widget would.
func pad(text string, width int) string {
	return strings.Repeat(" ", cellPadding) +
		runewidth.FillRight(text, width) +
		strings.Repeat(" ", cellPadding)
}

// render styles a value and fits it to its column.
func (s tableStyler) render(c cell, col int) string {
	return s.styleOf(c, col).Render(s.align(c.Text, col))
}

// cellAt renders a cell for the widget, carrying its own padding and, on the
// selected row, its own background.
//
// The highlight is applied per cell rather than around the finished row. Every
// cell ends in a reset, so a style wrapped around the whole row would stop at
// the first one and paint a fraction of the line.
func (s tableStyler) cellAt(c cell, col int, selected bool) string {
	style := s.styleOf(c, col)
	if selected {
		style = style.Background(s.theme.SelectedBackground)
	}
	return style.Render(pad(s.align(c.Text, col), s.widths[col]))
}

// styleOf is the cell's own style with the column's emphasis applied.
//
// The column that names the row is set in bold, so the eye can find its way
// back to which row a line belongs to without reading across the numbers.
func (s tableStyler) styleOf(c cell, col int) lipgloss.Style {
	style := s.theme.Label
	if c.Style != nil {
		style = *c.Style
	}
	if col < len(s.columns) && s.columns[col].Primary {
		style = style.Bold(true)
	}
	return style
}

// align fits a value to its column.
func (s tableStyler) align(text string, col int) string {
	width := s.widths[col]
	text = runewidth.Truncate(text, width, "…")

	fill := strings.Repeat(" ", max(width-runewidth.StringWidth(text), 0))
	if s.columns[col].Numeric {
		return fill + text
	}
	return text + fill
}

// plainHeader renders the column titles for output with no widget behind it.
func (s tableStyler) plainHeader() string {
	fields := make([]string, len(s.columns))
	for j, c := range s.columns {
		fields[j] = s.align(c.Title, j)
	}
	return strings.Join(fields, columnGap)
}

// plainRow renders one row for output with no widget behind it.
func (s tableStyler) plainRow(row []cell) string {
	fields := make([]string, len(s.columns))
	for j := range s.columns {
		fields[j] = s.render(cellAt(row, j), j)
	}
	return strings.Join(fields, columnGap)
}

// width returns the visible width of the whole table, including the padding
// around every cell.
func (s tableStyler) width() int {
	total := 0
	for _, w := range s.widths {
		total += w + 2*cellPadding
	}
	return total
}

// tableStyles returns the widget's own styles.
//
// Padding and selection are both done cell by cell in tableRows, so the
// widget's versions are turned off: its padding would leave unpainted gaps
// through a highlighted row, and its highlight would stop at the first cell.
func (s tableStyler) tableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = s.theme.Subtle.
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(s.theme.Border.GetForeground())
	styles.Cell = lipgloss.NewStyle()
	styles.Selected = styles.Cell
	return styles
}

// newTable builds a table widget over the given columns and rows.
func (s tableStyler) newTable(rows [][]cell, height int, focused bool) table.Model {
	return table.New(
		table.WithColumns(s.tableColumns()),
		table.WithRows(s.tableRows(rows, cursorNone)),
		table.WithHeight(height),
		table.WithWidth(s.width()),
		table.WithFocused(focused),
		table.WithStyles(s.tableStyles()),
	)
}

// cursorNone is a cursor position no row can be at, for a table nothing is
// selected in.
const cursorNone = -1
