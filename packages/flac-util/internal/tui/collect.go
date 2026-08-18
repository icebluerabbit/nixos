package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	bubbleprogress "charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"flac-util/internal/progress"
)

// RefreshInterval is how often the display re-reads the counters.
//
// The workers publish their state continuously through atomics; the display
// samples it. That is what stops a fast run from turning into a message storm.
const RefreshInterval = 50 * time.Millisecond

// recentLimit is how many finished files are kept for the scrolling list.
const recentLimit = 8

// tickMsg drives the periodic refresh.
type tickMsg time.Time

// tick returns a command that fires after the refresh interval.
func tick() tea.Cmd {
	return tea.Tick(RefreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// CollectModel is the body shown during a collect run.
type CollectModel struct {
	counters *progress.Counters
	theme    Theme
	bar      bubbleprogress.Model
	spin     spinner.Model
	now      func() time.Time

	snapshot progress.Snapshot
	recent   []progress.Event
	failures []progress.Event
	dropped  int64
}

// NewCollectModel returns the body for a collect run.
func NewCollectModel(counters *progress.Counters) *CollectModel {
	bar := bubbleprogress.New(bubbleprogress.WithDefaultBlend())
	bar.ShowPercentage = true

	spin := spinner.New()
	spin.Spinner = spinner.Dot

	return &CollectModel{
		counters: counters,
		theme:    NewTheme(),
		bar:      bar,
		spin:     spin,
		now:      time.Now,
	}
}

// Title implements Body.
func (m *CollectModel) Title() string { return "collect" }

// Keys implements Body. A run has no keys of its own: it is watched, not
// driven.
func (m *CollectModel) Keys() []key.Binding { return nil }

// Status implements Body, giving the title bar the run's clock.
//
// It comes from the counters rather than from the wall, so it measures the
// work rather than the process, and it is only ever shown while there is work
// to measure.
func (m *CollectModel) Status() string {
	if m.counters == nil {
		return ""
	}
	return m.snapshot.Elapsed.Round(time.Second).String()
}

// Init implements Body.
func (m *CollectModel) Init() tea.Cmd { return tea.Batch(tick(), m.spin.Tick) }

// Update implements Body.
func (m *CollectModel) Update(msg tea.Msg) (Body, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.refresh()
		return m, tick()

	case EventMsg:
		m.record(progress.Event(msg))
		return m, nil

	case DroppedMsg:
		m.dropped = int64(msg)
		return m, nil

	case FinishedMsg:
		// The counters are final by the time this arrives, so this is the
		// reading the last frame should show.
		m.refresh()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}
	return m, nil
}

// refresh re-reads the counters.
func (m *CollectModel) refresh() {
	if m.counters == nil {
		return
	}
	m.snapshot = m.counters.Snapshot(m.now())
}

// record keeps an event for display, holding failures separately since they
// are what a reader actually needs to see.
func (m *CollectModel) record(e progress.Event) {
	m.recent = append(m.recent, e)
	if len(m.recent) > recentLimit {
		m.recent = m.recent[len(m.recent)-recentLimit:]
	}
	if e.Outcome == progress.OutcomeFailed || e.Outcome == progress.OutcomeFaulted {
		m.failures = append(m.failures, e)
		if len(m.failures) > recentLimit {
			m.failures = m.failures[len(m.failures)-recentLimit:]
		}
	}
}

// View implements Body.
func (m *CollectModel) View(width int) string {
	width = max(width, 40)

	var b strings.Builder
	b.WriteString(m.overview(width))
	b.WriteString("\n\n")
	b.WriteString(m.stages(width))

	if len(m.failures) > 0 {
		b.WriteString("\n\n")
		b.WriteString(m.failureList(width))
	}
	if m.dropped > 0 {
		b.WriteString("\n")
		b.WriteString(m.theme.Subtle.Render(
			fmt.Sprintf("%d progress updates dropped to keep the run at full speed", m.dropped)))
	}
	return b.String()
}

// overview renders the progress bar and headline counts.
func (m *CollectModel) overview(width int) string {
	s := m.snapshot

	m.bar.SetWidth(max(width-20, 10))
	bar := m.bar.ViewAs(s.Fraction())

	counts := fmt.Sprintf("%s  %s  %s  %s  %s",
		m.theme.Value.Render(fmt.Sprintf("%d/%d", s.Finished(), s.Discovered)),
		m.theme.Good.Render(fmt.Sprintf("%d done", s.Completed)),
		m.theme.Subtle.Render(fmt.Sprintf("%d skipped", s.Skipped)),
		m.theme.Warn.Render(fmt.Sprintf("%d partial", s.Faulted)),
		m.theme.Bad.Render(fmt.Sprintf("%d failed", s.Failed)),
	)

	rate := m.theme.Subtle.Render(fmt.Sprintf("%.1f files/s  %s read",
		s.Rate(), humanBytes(s.Bytes)))

	return strings.Join([]string{
		m.spin.View() + " " + bar,
		counts,
		rate,
	}, "\n")
}

// stageColumns are the columns of the per-stage table.
func stageColumns() []column {
	return []column{
		{Title: "stage", Width: 10, Primary: true},
		{Title: "queued", Width: 8, Numeric: true},
		{Title: "done", Width: 8, Numeric: true},
		{Title: "failed", Width: 8, Numeric: true},
		{Title: "busy", Width: 12, Numeric: true},
	}
}

// stageRows turns the counters into the table's rows.
//
// Two values are coloured, and both for the same reason: they are the ones
// that mean something has to change. A stage holding items is where the run is
// bottlenecked and where more workers would go; a stage with failures is where
// the files are breaking.
func (m *CollectModel) stageRows() [][]cell {
	rows := make([][]cell, 0, len(m.snapshot.Stages))
	for _, s := range m.snapshot.Stages {
		queued := cell{Text: strconv.FormatInt(s.Queued, 10)}
		if s.Queued > 0 {
			queued = styled(queued.Text, m.theme.Warn)
		}
		failed := cell{Text: strconv.FormatInt(s.Failed, 10)}
		if s.Failed > 0 {
			failed = styled(failed.Text, m.theme.Bad)
		}

		rows = append(rows, []cell{
			{Text: s.Name},
			queued,
			{Text: strconv.FormatInt(s.Out, 10)},
			failed,
			{Text: s.Busy.Round(time.Millisecond).String()},
		})
	}
	return rows
}

// stages renders the per-stage table, which is what makes the worker counts
// tunable: whichever stage is holding items is the one to give more workers.
//
// Nothing here is selectable, so the table is built unfocused and redrawn from
// the counters on every tick rather than being kept between frames.
func (m *CollectModel) stages(width int) string {
	if len(m.snapshot.Stages) == 0 {
		return m.theme.Subtle.Render("waiting for the first file")
	}

	rows := m.stageRows()
	// Fixed rather than fitted: this table is redrawn as the run proceeds, and
	// a table that changes shape while it is on screen leaves the geometry of
	// whichever rows did not change behind.
	styler := newFixedTableStyler(m.theme, stageColumns())

	// The widget keeps one line of its height for itself, so a table of n rows
	// asks for n+1; anything more leaves blank rows under the last stage.
	widget := styler.newTable(rows, len(rows)+1, false)

	return lipgloss.NewStyle().MaxWidth(width).Render(widget.View())
}

// failureList renders what went wrong, and where.
//
// The track is named by its album and file rather than by its path, and the
// reason is stated without the path in it. The previous version showed the
// path truncated as a label and then the error, which repeated the path -
// usually leaving no room for the one part that says what actually happened.
func (m *CollectModel) failureList(width int) string {
	rows := []string{m.theme.Bad.Render("recent faults")}

	// The reason is what a reader is here for, so the track gets whatever is
	// left after the reason has room, not the other way round.
	nameWidth := max(min(width/3, 44), 12)

	for _, e := range m.failures {
		style := m.theme.Bad
		if e.Outcome == progress.OutcomeFaulted {
			style = m.theme.Warn
		}

		rows = append(rows, fmt.Sprintf("  %s  %s  %s",
			style.Render(fmt.Sprintf("%-7s", outcomeLabel(e.Outcome))),
			m.theme.Value.Render(runewidth.FillRight(
				runewidth.Truncate(trackLabel(e.Path), nameWidth, "…"), nameWidth)),
			m.theme.Subtle.Render(reasonOf(e))))
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(strings.Join(rows, "\n"))
}

// outcomeLabel names an outcome for the fault list.
func outcomeLabel(o progress.Outcome) string {
	if o == progress.OutcomeFaulted {
		return "partial"
	}
	return "failed"
}

// trackLabel names a track by its album and filename, which is what identifies
// it to a reader. The rest of the path is the same for every row on screen and
// only crowds out the reason.
func trackLabel(path string) string {
	dir, file := filepath.Split(path)
	album := filepath.Base(filepath.Clean(dir))
	if album == "." || album == string(filepath.Separator) {
		return file
	}
	return filepath.Join(album, file)
}

// reasonOf renders an event's error with the path taken back out of it. The
// decoder puts the path in front of every message, and repeating it here
// leaves no room for what it is in front of.
func reasonOf(e progress.Event) string {
	if e.Err == nil {
		return ""
	}

	reason := e.Err.Error()
	if _, after, found := strings.Cut(reason, e.Path+": "); found {
		reason = after
	}
	if e.Stage != "" {
		return e.Stage + ": " + reason
	}
	return reason
}

// truncatePath shortens a path from the left, keeping the part that identifies
// the track.
func truncatePath(path string, width int) string {
	if width < 8 || len(path) <= width {
		return path
	}
	return "..." + path[len(path)-width+3:]
}

// humanBytes renders a byte count in the largest unit that keeps it readable.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}

	div, exp := int64(unit), 0
	for size := n / unit; size >= unit && exp < 4; size /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTP"[exp])
}
