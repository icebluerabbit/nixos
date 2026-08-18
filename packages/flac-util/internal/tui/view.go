package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"flac-util/internal/inventory"
)

// chromeHeight is how many lines the view spends on everything that is not
// table rows: the release heading, the column header and the box border, plus
// the shell's own banner and footer.
const chromeHeight = 10

// ViewKeyMap are the keys the list responds to, on top of the shell's own.
type ViewKeyMap struct {
	Next key.Binding
	Prev key.Binding
	Sort key.Binding
}

// DefaultViewKeyMap returns the standard bindings.
func DefaultViewKeyMap() ViewKeyMap {
	return ViewKeyMap{
		Next: key.NewBinding(
			key.WithKeys("tab", "right", "l"),
			key.WithHelp("tab", "next release"),
		),
		Prev: key.NewBinding(
			key.WithKeys("shift+tab", "left", "h"),
			key.WithHelp("shift+tab", "previous release"),
		),
		Sort: key.NewBinding(
			key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"),
			key.WithHelp("1-9", "sort by column"),
		),
	}
}

// ShortHelp implements help.KeyMap.
func (k ViewKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Prev, k.Sort}
}

// FullHelp implements help.KeyMap.
func (k ViewKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Next, k.Prev, k.Sort}}
}

// ViewModel shows one release at a time.
//
// One release fills the screen rather than the whole library scrolling past,
// because the comparison worth making is between the tracks of a release. Two
// tables at once would only ever be half of two comparisons.
type ViewModel struct {
	tables   []*inventory.Table
	widgets  []table.Model
	stylers  []tableStyler
	rowCells [][][]cell
	theme    Theme
	keys     ViewKeyMap
	index    int
	sortCol  int
	sortAsc  bool
	width    int
	height   int
}

// NewViewModel returns the body for a view run.
func NewViewModel(tables []*inventory.Table) *ViewModel {
	m := &ViewModel{
		tables:  tables,
		theme:   NewTheme(),
		keys:    DefaultViewKeyMap(),
		sortCol: -1,
		sortAsc: true,
		width:   100,
		height:  30,
	}
	m.build()
	return m
}

// build makes a table widget for each release.
func (m *ViewModel) build() {
	m.widgets = make([]table.Model, len(m.tables))
	m.stylers = make([]tableStyler, len(m.tables))
	m.rowCells = make([][][]cell, len(m.tables))

	for i, t := range m.tables {
		m.rowCells[i] = inventoryRows(m.theme, t.Rows)
		m.stylers[i] = newTableStyler(m.theme, inventoryColumns(t.Columns), m.rowCells[i])
		m.widgets[i] = m.stylers[i].newTable(m.rowCells[i], m.heightFor(i), true)
		m.widgets[i].SetCursor(0)
	}
}

// heightFor is how tall the table for one release should be.
//
// A release is as tall as it has tracks, up to what the window allows. Padding
// a four-track album out to the height of the terminal leaves a box mostly
// full of nothing, and the emptiness reads as missing data rather than as
// spare room.
func (m *ViewModel) heightFor(i int) int {
	// The widget draws its column header and the rule under it outside the
	// height it is given, and keeps one line of that height for itself, so a
	// table of n rows asks for n+1.
	return min(len(m.tables[i].Rows)+1, m.rows())
}

// mark redraws the rows of the release on screen so the cursor marker sits on
// the selected one.
func (m *ViewModel) mark() {
	if len(m.widgets) == 0 {
		return
	}
	i := m.index
	m.widgets[i].SetRows(m.stylers[i].tableRows(m.rowCells[i], m.widgets[i].Cursor()))
}

// rows is how many rows of table fit on screen.
func (m *ViewModel) rows() int {
	return max(m.height-chromeHeight, 1)
}

// Title implements Body.
func (m *ViewModel) Title() string { return "view" }

// Keys implements Body.
func (m *ViewModel) Keys() []key.Binding { return m.keys.ShortHelp() }

// Status implements Body, saying how much there is to read.
//
// There is deliberately no clock. Nothing is running, so a clock would only
// count how long the reader had been looking - and, with nothing to redraw it,
// would sit at zero while it did.
func (m *ViewModel) Status() string {
	if len(m.tables) == 0 {
		return ""
	}
	return fmt.Sprintf("%d releases", len(m.tables))
}

// Init implements Body.
func (m *ViewModel) Init() tea.Cmd { return nil }

// Update implements Body.
func (m *ViewModel) Update(msg tea.Msg) (Body, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Rebuilt rather than resized: the widget's SetHeight and its
		// constructor disagree about how much of a height the header takes, so
		// a table resized in place ends up a row short of one built at the
		// same size. Resizes are rare enough that rebuilding is the cheaper
		// mistake to avoid.
		cursors := m.cursors()
		m.build()
		m.restore(cursors)
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Next):
			m.move(1)
			return m, nil
		case key.Matches(msg, m.keys.Prev):
			m.move(-1)
			return m, nil
		case key.Matches(msg, m.keys.Sort):
			m.sort(int(msg.String()[0]-'1'), m.columns())
			return m, nil
		}
	}

	if len(m.widgets) == 0 {
		return m, nil
	}

	var cmd tea.Cmd
	m.widgets[m.index], cmd = m.widgets[m.index].Update(msg)
	m.mark()
	return m, cmd
}

// columns returns the columns of the release on screen.
func (m *ViewModel) columns() []inventory.Column {
	if len(m.tables) == 0 {
		return nil
	}
	return m.tables[m.index].Columns
}

// move steps to another release, stopping at either end rather than wrapping,
// so that holding a key does not silently cycle past the one being read.
func (m *ViewModel) move(by int) {
	if len(m.tables) == 0 {
		return
	}
	m.index = min(max(m.index+by, 0), len(m.tables)-1)
	m.mark()
}

// sort orders every release by a column.
//
// Sorting applies to all of them rather than to the one on screen: the point
// of a sort is to hold a question steady while stepping through releases, and
// a sort that reset at every tab would not answer one.
func (m *ViewModel) sort(col int, columns []inventory.Column) {
	if col < 0 || col >= len(columns) {
		return
	}

	// Choosing the column already sorted on turns the order around.
	if col == m.sortCol {
		m.sortAsc = !m.sortAsc
	} else {
		m.sortCol, m.sortAsc = col, true
	}

	cursors := m.cursors()
	for _, t := range m.tables {
		t.SortBy(m.sortCol, m.sortAsc)
	}
	m.build()
	m.restore(cursors)
}

// View implements Body.
func (m *ViewModel) View(width int) string {
	if width > 0 {
		m.width = width
	}
	if len(m.tables) == 0 {
		return m.theme.Subtle.Render("no tracks found")
	}

	box := m.theme.Border.
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Render(m.widgets[m.index].View())

	return m.releaseHeader() + "\n" + box
}

// releaseHeader names the release on screen, where it sits in the run and how
// its rows are ordered.
func (m *ViewModel) releaseHeader() string {
	t := m.tables[m.index]

	position := m.theme.Subtle.Render(fmt.Sprintf("%d/%d", m.index+1, len(m.tables)))
	label := m.theme.Title.Render(runewidth.Truncate(
		t.Group.Label(), max(m.width-lipgloss.Width(position)-3, 10), "…"))

	gap := m.width - lipgloss.Width(label) - lipgloss.Width(position)
	title := label + strings.Repeat(" ", max(gap, 1)) + position

	detail := m.theme.Subtle.Render(runewidth.Truncate(t.Group.Dir, max(m.width-24, 10), "…")) +
		m.theme.Subtle.Render("  ·  "+m.sortLabel())

	return title + "\n" + detail
}

// sortLabel says what the rows are ordered by.
func (m *ViewModel) sortLabel() string {
	columns := m.columns()
	if m.sortCol < 0 || m.sortCol >= len(columns) {
		return "in track order"
	}
	direction := "ascending"
	if !m.sortAsc {
		direction = "descending"
	}
	return "by " + columns[m.sortCol].Title + " " + direction
}

// cursors records where the cursor sits in every release, so that rebuilding
// the tables does not lose the reader's place.
func (m *ViewModel) cursors() []int {
	out := make([]int, len(m.widgets))
	for i := range m.widgets {
		out[i] = m.widgets[i].Cursor()
	}
	return out
}

// restore puts the cursors back and redraws the release on screen.
func (m *ViewModel) restore(cursors []int) {
	for i := range m.widgets {
		if i < len(cursors) {
			m.widgets[i].SetCursor(cursors[i])
		}
	}
	m.mark()
}
