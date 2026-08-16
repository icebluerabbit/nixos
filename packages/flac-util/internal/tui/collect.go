package tui

import (
	"fmt"
	"strings"
	"time"

	bubbleprogress "github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	bar := bubbleprogress.New(bubbleprogress.WithDefaultGradient())
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
func (m *CollectModel) Title() string { return "flac-util collect" }

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
	if e.Outcome == progress.OutcomeFailed {
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

	m.bar.Width = max(width-20, 10)
	bar := m.bar.ViewAs(s.Fraction())

	counts := fmt.Sprintf("%s  %s  %s  %s",
		m.theme.Value.Render(fmt.Sprintf("%d/%d", s.Finished(), s.Discovered)),
		m.theme.Good.Render(fmt.Sprintf("%d done", s.Completed)),
		m.theme.Subtle.Render(fmt.Sprintf("%d skipped", s.Skipped)),
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

// stages renders the per-stage table, which is what makes the worker counts
// tunable: whichever stage is holding items is the one to give more workers.
func (m *CollectModel) stages(width int) string {
	if len(m.snapshot.Stages) == 0 {
		return m.theme.Subtle.Render("waiting for the first file")
	}

	nameWidth := 0
	for _, s := range m.snapshot.Stages {
		nameWidth = max(nameWidth, len(s.Name))
	}

	rows := []string{m.theme.Label.Render(fmt.Sprintf(
		"%-*s  %8s  %8s  %8s  %10s", nameWidth, "stage", "queued", "done", "failed", "busy"))}

	for _, s := range m.snapshot.Stages {
		queued := fmt.Sprintf("%8d", s.Queued)
		if s.Queued > 0 {
			queued = m.theme.Warn.Render(queued)
		}
		failed := fmt.Sprintf("%8d", s.Failed)
		if s.Failed > 0 {
			failed = m.theme.Bad.Render(failed)
		}

		rows = append(rows, fmt.Sprintf("%-*s  %s  %8d  %s  %10s",
			nameWidth, s.Name, queued, s.Out, failed, s.Busy.Round(time.Millisecond)))
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(strings.Join(rows, "\n"))
}

// failureList renders the most recent failures.
func (m *CollectModel) failureList(width int) string {
	rows := []string{m.theme.Bad.Render("recent failures")}
	for _, e := range m.failures {
		rows = append(rows, fmt.Sprintf("  %s  %s: %v",
			m.theme.Subtle.Render(e.Stage), truncatePath(e.Path, width/2), e.Err))
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(strings.Join(rows, "\n"))
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
