package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"

	"homelab/flac-util/internal/pipeline"
	"homelab/flac-util/internal/progress"
)

var modelStart = time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)

func newTestCollectModel() (*CollectModel, *progress.Counters) {
	counters := progress.NewCounters(pipeline.CollectStageNames(), modelStart)
	m := NewCollectModel(counters)
	m.now = func() time.Time { return modelStart.Add(10 * time.Second) }
	return m, counters
}

func TestCollectModelTitle(t *testing.T) {
	m, _ := newTestCollectModel()
	if got := m.Title(); got == "" {
		t.Error("Title() is empty")
	}
}

func TestCollectModelInitStartsTheRefresh(t *testing.T) {
	m, _ := newTestCollectModel()
	if m.Init() == nil {
		t.Error("Init() returned no command, so the display would never refresh")
	}
}

func TestCollectModelTickReadsTheCounters(t *testing.T) {
	m, counters := newTestCollectModel()
	counters.Discovered(10)
	counters.Completed(2048)

	updated, cmd := m.Update(tickMsg(modelStart))
	m = updated.(*CollectModel)

	if cmd == nil {
		t.Error("the tick did not schedule the next refresh")
	}
	if m.snapshot.Discovered != 10 {
		t.Errorf("Discovered = %d, want 10", m.snapshot.Discovered)
	}
	if m.snapshot.Completed != 1 {
		t.Errorf("Completed = %d, want 1", m.snapshot.Completed)
	}
}

func TestCollectModelFinishedReadsTheCountersOneLastTime(t *testing.T) {
	m, counters := newTestCollectModel()
	counters.Discovered(4)
	counters.Completed(1024)
	m.Update(tickMsg(modelStart))

	// What the last three files did lands after that tick, which is exactly
	// the case that used to leave the finished run showing 1/4.
	for range 3 {
		counters.Completed(1024)
	}

	updated, _ := m.Update(FinishedMsg{})
	m = updated.(*CollectModel)

	if m.snapshot.Completed != 4 {
		t.Errorf("Completed = %d, want 4", m.snapshot.Completed)
	}
	if got := m.snapshot.Fraction(); got != 1 {
		t.Errorf("Fraction() = %v, want 1", got)
	}
}

func TestCollectModelWithoutCounters(t *testing.T) {
	m := NewCollectModel(nil)
	if _, cmd := m.Update(tickMsg(modelStart)); cmd == nil {
		t.Error("the tick did not schedule the next refresh")
	}
	if got := m.View(80); got == "" {
		t.Error("View() is empty without counters")
	}
}

func TestCollectModelRecordsEvents(t *testing.T) {
	m, _ := newTestCollectModel()

	for i := range recentLimit + 5 {
		path := strings.Repeat("a", i+1) + ".flac"
		updated, _ := m.Update(EventMsg{Outcome: progress.OutcomeCompleted, Path: path})
		m = updated.(*CollectModel)
	}

	if len(m.recent) != recentLimit {
		t.Errorf("kept %d recent events, want at most %d", len(m.recent), recentLimit)
	}
	if len(m.failures) != 0 {
		t.Errorf("kept %d failures, want none", len(m.failures))
	}
}

func TestCollectModelKeepsFailuresSeparately(t *testing.T) {
	m, _ := newTestCollectModel()

	updated, _ := m.Update(EventMsg{
		Outcome: progress.OutcomeFailed,
		Path:    "/music/broken.flac",
		Stage:   pipeline.StageDecode,
		Err:     errors.New("truncated frame"),
	})
	m = updated.(*CollectModel)

	if len(m.failures) != 1 {
		t.Fatalf("kept %d failures, want 1", len(m.failures))
	}

	got := m.View(100)
	for _, want := range []string{"recent faults", "broken.flac", "truncated frame", pipeline.StageDecode} {
		if !strings.Contains(got, want) {
			t.Errorf("view does not contain %q:\n%s", want, got)
		}
	}
}

func TestCollectModelBoundsTheFailureList(t *testing.T) {
	m, _ := newTestCollectModel()

	for range recentLimit + 5 {
		updated, _ := m.Update(EventMsg{Outcome: progress.OutcomeFailed, Err: errors.New("broken")})
		m = updated.(*CollectModel)
	}
	if len(m.failures) != recentLimit {
		t.Errorf("kept %d failures, want at most %d", len(m.failures), recentLimit)
	}
}

func TestCollectModelReportsDroppedEvents(t *testing.T) {
	m, _ := newTestCollectModel()

	updated, _ := m.Update(DroppedMsg(42))
	m = updated.(*CollectModel)

	if m.dropped != 42 {
		t.Errorf("dropped = %d, want 42", m.dropped)
	}
	if got := m.View(100); !strings.Contains(got, "42 progress updates dropped") {
		t.Errorf("view does not mention dropped updates:\n%s", got)
	}
}

func TestCollectModelAdvancesTheSpinner(t *testing.T) {
	m, _ := newTestCollectModel()

	_, cmd := m.Update(spinner.TickMsg{})
	if cmd == nil {
		t.Error("the spinner did not schedule its next frame")
	}
}

func TestCollectModelIgnoresUnknownMessages(t *testing.T) {
	m, _ := newTestCollectModel()

	updated, cmd := m.Update("something else entirely")
	if updated != Body(m) {
		t.Error("an unknown message replaced the model")
	}
	if cmd != nil {
		t.Error("an unknown message produced a command")
	}
}

func TestCollectModelViewShowsProgress(t *testing.T) {
	m, counters := newTestCollectModel()
	counters.Discovered(4)
	counters.Completed(1024)
	counters.Skipped()
	counters.Failed()

	updated, _ := m.Update(tickMsg(modelStart))
	m = updated.(*CollectModel)

	got := m.View(100)
	for _, want := range []string{"3/4", "1 done", "1 skipped", "1 failed", "files/s"} {
		if !strings.Contains(got, want) {
			t.Errorf("view does not contain %q:\n%s", want, got)
		}
	}
}

// TestCollectModelViewShowsQueueDepths covers the display's real job: showing
// which stage is holding work, which is how the worker counts get tuned.
func TestCollectModelViewShowsQueueDepths(t *testing.T) {
	m, counters := newTestCollectModel()
	counters.Stage(pipeline.StageDecode).Enter()

	updated, _ := m.Update(tickMsg(modelStart))
	m = updated.(*CollectModel)

	got := m.View(120)
	for _, want := range []string{"stage", "queued", "busy", pipeline.StageDecode} {
		if !strings.Contains(got, want) {
			t.Errorf("view does not contain %q:\n%s", want, got)
		}
	}
}

func TestCollectModelViewBeforeAnyWork(t *testing.T) {
	m := NewCollectModel(progress.NewCounters(nil, modelStart))

	updated, _ := m.Update(tickMsg(modelStart))
	m = updated.(*CollectModel)

	if got := m.View(80); !strings.Contains(got, "waiting for the first file") {
		t.Errorf("view does not say it is waiting:\n%s", got)
	}
}

func TestCollectModelViewAtNarrowWidths(t *testing.T) {
	m, counters := newTestCollectModel()
	counters.Discovered(1)

	updated, _ := m.Update(tickMsg(modelStart))
	m = updated.(*CollectModel)

	for _, width := range []int{0, 1, 10, 39} {
		if got := m.View(width); got == "" {
			t.Errorf("View(%d) is empty", width)
		}
	}
}

func TestTruncatePath(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		width int
		want  string
	}{
		{"short enough", "a.flac", 20, "a.flac"},
		{"exactly the width", "abcdefgh", 8, "abcdefgh"},
		{"trimmed from the left", "/music/artist/album/track.flac", 13, "...track.flac"},
		{"width too small to trim", "/music/artist/album/track.flac", 4, "/music/artist/album/track.flac"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncatePath(tt.path, tt.width); got != tt.want {
				t.Errorf("truncatePath(%q, %d) = %q, want %q", tt.path, tt.width, got, tt.want)
			}
		})
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{3 * 1024 * 1024 * 1024, "3.0 GiB"},
		{2 * 1024 * 1024 * 1024 * 1024, "2.0 TiB"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := humanBytes(tt.bytes); got != tt.want {
				t.Errorf("humanBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestCollectModelKeepsFaultsWithFailures(t *testing.T) {
	// A file that produced a report despite a bad frame is the one a reader is
	// most likely to want to look at, so it belongs in the same list.
	m, _ := newTestCollectModel()

	updated, _ := m.Update(EventMsg{
		Outcome: progress.OutcomeFaulted,
		Path:    "/music/Album/03 - short.flac",
		Stage:   pipeline.StageDecode,
		Err:     errors.New("frame 1696: invalid sync-code"),
	})
	m = updated.(*CollectModel)

	if len(m.failures) != 1 {
		t.Fatalf("kept %d entries, want the fault", len(m.failures))
	}
	got := m.View(120)
	for _, want := range []string{"partial", "Album/03 - short.flac", "invalid sync-code"} {
		if !strings.Contains(got, want) {
			t.Errorf("the fault list does not contain %q:\n%s", want, got)
		}
	}
}

func TestTrackLabel(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"album and file", "/mnt/media/library/music/A/Album (2024)/01 - Track.flac", "Album (2024)/01 - Track.flac"},
		{"no directory", "track.flac", "track.flac"},
		{"at the root", "/track.flac", "track.flac"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trackLabel(tt.path); got != tt.want {
				t.Errorf("trackLabel(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestReasonOfDropsTheRepeatedPath covers the display fault that made the list
// useless: the decoder puts the path in front of every message, so showing the
// error verbatim beside a truncated path printed the path twice and pushed the
// actual reason off the end of the line.
func TestReasonOfDropsTheRepeatedPath(t *testing.T) {
	path := "/mnt/media/library/music/A/Album/01 - Track.flac"
	e := progress.Event{
		Path:  path,
		Stage: pipeline.StageDecode,
		Err:   fmt.Errorf("decode %s: frame 1696: invalid sync-code", path),
	}

	got := reasonOf(e)
	if strings.Contains(got, path) {
		t.Errorf("reasonOf() still carries the path: %q", got)
	}
	if want := "decode: frame 1696: invalid sync-code"; got != want {
		t.Errorf("reasonOf() = %q, want %q", got, want)
	}
}

func TestReasonOfWithoutAnError(t *testing.T) {
	if got := reasonOf(progress.Event{}); got != "" {
		t.Errorf("reasonOf() = %q, want empty", got)
	}
}

func TestOutcomeLabel(t *testing.T) {
	if got := outcomeLabel(progress.OutcomeFaulted); got != "partial" {
		t.Errorf("outcomeLabel(faulted) = %q", got)
	}
	if got := outcomeLabel(progress.OutcomeFailed); got != "failed" {
		t.Errorf("outcomeLabel(failed) = %q", got)
	}
}

// frameWidths returns the visible width of every line of the stage table.
func frameWidths(t *testing.T, m *CollectModel) []int {
	t.Helper()
	m.Update(tickMsg(modelStart))

	var out []int
	for _, line := range strings.Split(m.stages(120), "\n") {
		out = append(out, lipgloss.Width(line))
	}
	return out
}

func TestCollectStageTableKeepsItsShapeAsTheRunProceeds(t *testing.T) {
	// The stage table is redrawn on every tick while its values grow. If its
	// geometry followed those values the renderer would leave the rows that
	// happened not to change holding the previous layout, and the table would
	// show two different column positions at once.
	counters := progress.NewCounters(pipeline.CollectStageNames(), modelStart)
	counters.Discovered(69)
	m := NewCollectModel(counters)
	m.now = func() time.Time { return modelStart.Add(10 * time.Second) }

	early := frameWidths(t, m)

	// Time the stages, giving each a different duration so that the run ends
	// with a wide spread of values in the busy column.
	for i, name := range pipeline.CollectStageNames() {
		stage := counters.Stage(name)
		stage.Enter()
		stage.Leave(time.Duration(i)*47*time.Second, nil)
	}

	late := frameWidths(t, m)

	if len(early) != len(late) {
		t.Fatalf("frame has %d lines early and %d late", len(early), len(late))
	}
	for i := range early {
		if early[i] != late[i] {
			t.Errorf("line %d is %d wide early and %d wide late; the table moved",
				i, early[i], late[i])
		}
	}
}

func TestCollectStageTableRowsAreAllTheSameWidth(t *testing.T) {
	counters := progress.NewCounters(pipeline.CollectStageNames(), modelStart)
	counters.Discovered(69)
	for i, name := range pipeline.CollectStageNames() {
		stage := counters.Stage(name)
		stage.Enter()
		stage.Leave(time.Duration(i)*47*time.Second, nil)
	}
	m := NewCollectModel(counters)
	m.now = func() time.Time { return modelStart.Add(10 * time.Second) }

	widths := frameWidths(t, m)
	for i, got := range widths {
		if got != widths[0] {
			t.Errorf("line %d is %d wide, want %d like the rest", i, got, widths[0])
		}
	}
}
