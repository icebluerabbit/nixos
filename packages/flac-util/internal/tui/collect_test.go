package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"

	"flac-util/internal/pipeline"
	"flac-util/internal/progress"
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
	for _, want := range []string{"recent failures", "broken.flac", "truncated frame", pipeline.StageDecode} {
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
