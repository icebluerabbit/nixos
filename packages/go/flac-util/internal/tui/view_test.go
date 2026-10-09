package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"homelab/flac-util/internal/inventory"
	"homelab/flac-util/internal/report"
)

// viewTable builds a release of the given track names, each reaching the
// given fraction of its ceiling, so that a sort has something to reorder.
func viewTable(dir string, reaches map[string]float64) *inventory.Table {
	entries := make([]inventory.Entry, 0, len(reaches))
	for _, name := range sortedKeys(reaches) {
		entries = append(entries, inventory.Entry{
			AudioPath: dir + "/" + name + ".flac",
			State:     report.StateCurrent,
			Report: &report.Report{
				Stream:     report.Stream{SampleRate: 44100, BitsPerSample: 16, Channels: 2},
				Resolution: report.Resolution{EffectiveBits: 16},
				Integrity:  report.Integrity{StoredMD5Present: true, Match: true},
				Decode:     report.Decode{Complete: true},
				Levels:     []report.ChannelLevels{{}},
				Stereo:     &report.Stereo{Correlation: 0.5},
				Spectrum:   reaching(44100, reaches[name]),
			},
		})
	}
	return inventory.NewTable(inventory.Group{Dir: dir, Entries: entries}, inventory.Options{})
}

// reaching returns a spectrum whose content stops at the given fraction of
// Nyquist, with silence above it.
func reaching(rate int, fraction float64) *report.Spectrum {
	const bins = 101

	levels := make([]report.Number, bins)
	last := int(fraction * float64(bins-1))
	for i := range levels {
		levels[i] = -10
		if i > last {
			levels[i] = -400
		}
	}
	return &report.Spectrum{
		BinHz:       report.Number(float64(rate) / 2 / float64(bins-1)),
		Bins:        bins,
		AverageDBFS: levels,
	}
}

// sortedKeys returns a map's keys in order, so tests are repeatable.
func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// twoReleases returns a model over two releases.
func twoReleases() *ViewModel {
	return NewViewModel([]*inventory.Table{
		viewTable("/music/A/One", map[string]float64{"01 a": 0.7, "02 b": 0.5}),
		viewTable("/music/A/Two", map[string]float64{"01 c": 0.6, "02 d": 0.4}),
	})
}

// press sends a keystroke to the model.
func press(m *ViewModel, key string) *ViewModel {
	body, _ := m.Update(keyPress(key))
	return body.(*ViewModel)
}

// cutoffColumn is the index of the cutoff column, the one numeric column the
// list carries.
func cutoffColumn(t *testing.T) int {
	t.Helper()
	for i, c := range inventory.Columns() {
		if c.Title == "cutoff" {
			return i
		}
	}
	t.Fatal("there is no cutoff column")
	return -1
}

func TestViewModelTitle(t *testing.T) {
	if got := twoReleases().Title(); got == "" {
		t.Error("Title() is empty")
	}
}

func TestViewModelShowsOneReleaseAtATime(t *testing.T) {
	m := twoReleases()

	if got := m.View(120); !strings.Contains(got, "/music/A/One") {
		t.Errorf("the first release is not on screen:\n%s", got)
	}
	if got := m.View(120); strings.Contains(got, "/music/A/Two") {
		t.Errorf("the second release is on screen at the same time:\n%s", got)
	}
}

func TestViewModelStepsBetweenReleases(t *testing.T) {
	m := press(twoReleases(), "tab")
	if got := m.View(120); !strings.Contains(got, "/music/A/Two") {
		t.Errorf("tab did not move to the second release:\n%s", got)
	}

	m = press(m, "shift+tab")
	if got := m.View(120); !strings.Contains(got, "/music/A/One") {
		t.Errorf("shift+tab did not move back:\n%s", got)
	}
}

func TestViewModelStopsAtEitherEnd(t *testing.T) {
	// Holding a key must not cycle past the release being read.
	m := twoReleases()
	for range 5 {
		m = press(m, "shift+tab")
	}
	if m.index != 0 {
		t.Errorf("index = %d after moving back repeatedly, want 0", m.index)
	}

	for range 5 {
		m = press(m, "tab")
	}
	if want := len(m.tables) - 1; m.index != want {
		t.Errorf("index = %d after moving on repeatedly, want %d", m.index, want)
	}
}

func TestViewModelSortsEveryRelease(t *testing.T) {
	// A sort is a question held steady while stepping through releases, so it
	// has to apply to all of them rather than to the one on screen.
	m := twoReleases()
	cutoff := cutoffColumn(t)
	m = press(m, string(rune('1'+cutoff)))

	for i, table := range m.tables {
		first := table.Rows[0][cutoff].Value
		last := table.Rows[len(table.Rows)-1][cutoff].Value
		if first > last {
			t.Errorf("release %d runs %v to %v, want ascending", i, first, last)
		}
	}
}

func TestViewModelSortTogglesDirection(t *testing.T) {
	m := twoReleases()
	cutoff := cutoffColumn(t)
	key := string(rune('1' + cutoff))

	m = press(m, key)
	if !m.sortAsc {
		t.Fatal("the first press did not sort ascending")
	}

	m = press(m, key)
	if m.sortAsc {
		t.Error("pressing the same column again did not turn the order around")
	}
	if got := m.tables[0].Rows[0][cutoff].Value; got != 0.7 {
		t.Errorf("first row = %v, want the largest value 0.7", got)
	}
}

func TestViewModelSortIgnoresAColumnThatIsNotThere(t *testing.T) {
	m := twoReleases()
	before := m.tables[0].Rows[0][0].Text

	// There are nine columns and nine sort keys, so the guard is reached by
	// asking for a column directly rather than by a keystroke.
	m.sort(len(inventory.Columns()), inventory.Columns())
	m.sort(-1, inventory.Columns())

	if got := m.tables[0].Rows[0][0].Text; got != before {
		t.Errorf("a column past the last one reordered the table: %q became %q", before, got)
	}
	if m.sortCol != -1 {
		t.Errorf("sortCol = %d, want it left alone", m.sortCol)
	}
}

func TestViewModelSortsByTheLastColumn(t *testing.T) {
	// Nine columns and nine keys: the last key has to reach the last column,
	// or a column is unsortable with no way to tell.
	m := twoReleases()
	columns := inventory.Columns()

	m = press(m, string(rune('0'+len(columns))))
	if got, want := m.sortCol, len(columns)-1; got != want {
		t.Errorf("sortCol = %d, want %d (%s)", got, want, columns[want].Title)
	}
}

func TestViewModelSortLabel(t *testing.T) {
	m := twoReleases()
	if got := m.sortLabel(); got != "in track order" {
		t.Errorf("sortLabel() = %q before any sort", got)
	}

	cutoff := cutoffColumn(t)
	m = press(m, string(rune('1'+cutoff)))
	if got, want := m.sortLabel(), "by cutoff ascending"; got != want {
		t.Errorf("sortLabel() = %q, want %q", got, want)
	}
}

func TestViewModelWithNothingToShow(t *testing.T) {
	m := NewViewModel(nil)

	if got := m.View(80); !strings.Contains(got, "no tracks found") {
		t.Errorf("View() = %q", got)
	}
	// None of these have anything to act on and none may panic.
	m = press(m, "tab")
	m = press(m, "1")
	if got := m.sortLabel(); got != "in track order" {
		t.Errorf("sortLabel() = %q", got)
	}
}

// longRelease returns a release with more tracks than a small window can show.
func longRelease() *ViewModel {
	corrs := map[string]float64{}
	for i := range 40 {
		corrs[fmt.Sprintf("%02d track", i)] = 0.5 + float64(i)/100
	}
	return NewViewModel([]*inventory.Table{viewTable("/music/A/Long", corrs)})
}

func TestViewModelResizes(t *testing.T) {
	m := longRelease()
	body, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = body.(*ViewModel)

	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
	tall := m.widgets[0].Height()

	body, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = body.(*ViewModel)
	if short := m.widgets[0].Height(); short >= tall {
		t.Errorf("the table is %d rows tall on a 20-line terminal and %d on a 40-line one", short, tall)
	}
}

// TestViewModelDoesNotPadAShortRelease covers the difference between a box
// with room to spare and a box that looks like it lost its contents.
func TestViewModelDoesNotPadAShortRelease(t *testing.T) {
	m := twoReleases()
	body, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	m = body.(*ViewModel)

	rendered := m.widgets[0].View()
	if got, want := strings.Count(rendered, "\n")+1, len(m.tables[0].Rows)+2; got > want {
		t.Errorf("a %d-track release drew %d lines, want no more than %d",
			len(m.tables[0].Rows), got, want)
	}
}

func TestViewModelKeepsATallEnoughTableOnATinyTerminal(t *testing.T) {
	m := twoReleases()
	body, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 3})
	m = body.(*ViewModel)

	if got := m.rows(); got < 1 {
		t.Errorf("rows() = %d on a three-line terminal, want at least 1", got)
	}
}

// TestViewModelColouredCellsSurviveTheWidget is the regression this whole
// table was rebuilt for.
//
// Bubbles v1 truncated a cell with a function that counted escape sequences as
// visible characters, so a coloured "0.549" came out as "0.5…" in a column
// wide enough to hold it. v2 measures with an escape-aware function, and this
// checks the values arrive whole.
func TestViewModelColouredCellsSurviveTheWidget(t *testing.T) {
	m := twoReleases()
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})

	rendered := m.widgets[0].View()
	for _, want := range []string{"15.4k", "11.0k", "44.1", "16bit"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the table does not contain %q whole:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "…") {
		t.Errorf("a value was truncated:\n%s", rendered)
	}
}

// TestViewModelRowsAreNoWiderThanTheirColumns checks the escape sequences cost
// nothing in width, which is what lets a cell be coloured before the widget
// ever sees it.
func TestViewModelRowsAreNoWiderThanTheirColumns(t *testing.T) {
	m := twoReleases()
	columns := m.widgets[0].Columns()

	for i, row := range m.widgets[0].Rows() {
		for j, cell := range row {
			if got := lipgloss.Width(cell); got > columns[j].Width {
				t.Errorf("row %d cell %d is %d wide against a column of %d",
					i, j, got, columns[j].Width)
			}
		}
	}
}
