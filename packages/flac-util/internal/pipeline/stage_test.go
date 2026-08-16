package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"flac-util/internal/progress"
	"flac-util/internal/report"
)

// recordingStage notes every job it sees and optionally fails.
type recordingStage struct {
	name string
	fail error

	mu   sync.Mutex
	seen []string
	runs atomic.Int64
}

func (s *recordingStage) Name() string { return s.name }

func (s *recordingStage) Process(_ context.Context, j *Job) error {
	s.runs.Add(1)
	s.mu.Lock()
	s.seen = append(s.seen, j.Path)
	s.mu.Unlock()
	return s.fail
}

func (s *recordingStage) Seen() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.seen)
}

// feed returns a closed channel carrying the named jobs.
func feed(paths ...string) <-chan *Job {
	ch := make(chan *Job, len(paths))
	for _, p := range paths {
		ch <- &Job{Path: p, Started: time.Now()}
	}
	close(ch)
	return ch
}

func TestJobTerminal(t *testing.T) {
	tests := []struct {
		name string
		job  Job
		want bool
	}{
		{"fresh", Job{}, false},
		{"failed", Job{Err: errors.New("broken")}, true},
		{"skipped", Job{Skip: report.SkipUpToDate}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.job.Terminal(); got != tt.want {
				t.Errorf("Terminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJobSize(t *testing.T) {
	if got := (&Job{}).Size(); got != 0 {
		t.Errorf("Size() without file info = %d, want 0", got)
	}

	path := filepath.Join(t.TempDir(), "track.flac")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := (&Job{Info: info}).Size(); got != 10 {
		t.Errorf("Size() = %d, want 10", got)
	}
}

func TestStageSpecValidate(t *testing.T) {
	good := &recordingStage{name: "decode"}
	tests := []struct {
		name    string
		spec    StageSpec
		wantErr bool
	}{
		{"valid", StageSpec{Stage: good, Workers: 2, Buffer: 4}, false},
		{"unbuffered is fine", StageSpec{Stage: good, Workers: 1}, false},
		{"nil stage", StageSpec{Workers: 1}, true},
		{"nameless stage", StageSpec{Stage: &recordingStage{}, Workers: 1}, true},
		{"no workers", StageSpec{Stage: good}, true},
		{"negative workers", StageSpec{Stage: good, Workers: -1}, true},
		{"negative buffer", StageSpec{Stage: good, Workers: 1, Buffer: -1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.spec.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewRunnerRejectsBadPipelines(t *testing.T) {
	tests := []struct {
		name  string
		specs []StageSpec
	}{
		{"no stages", nil},
		{"invalid stage", []StageSpec{{Stage: &recordingStage{name: "a"}}}},
		{
			name: "duplicate names",
			specs: []StageSpec{
				{Stage: &recordingStage{name: "a"}, Workers: 1},
				{Stage: &recordingStage{name: "a"}, Workers: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewRunner(tt.specs, nil, nil); err == nil {
				t.Fatal("NewRunner() succeeded, want error")
			}
		})
	}
}

func TestRunnerStageNames(t *testing.T) {
	specs := []StageSpec{
		{Stage: &recordingStage{name: "probe"}, Workers: 1},
		{Stage: &recordingStage{name: "decode"}, Workers: 1},
	}
	r, err := NewRunner(specs, nil, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if got := r.StageNames(); !slices.Equal(got, []string{"probe", "decode"}) {
		t.Errorf("StageNames() = %v, want [probe decode]", got)
	}
}

func TestRunnerPassesEveryJobThroughEveryStage(t *testing.T) {
	first := &recordingStage{name: "first"}
	second := &recordingStage{name: "second"}

	var rec progress.Recorder
	r, err := NewRunner([]StageSpec{
		{Stage: first, Workers: 2, Buffer: 2},
		{Stage: second, Workers: 3, Buffer: 1},
	}, nil, &rec)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if err := r.Run(t.Context(), feed("a", "b", "c")); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, stage := range []*recordingStage{first, second} {
		got := stage.Seen()
		slices.Sort(got)
		if !slices.Equal(got, []string{"a", "b", "c"}) {
			t.Errorf("stage %q saw %v, want all three jobs", stage.Name(), got)
		}
	}
	if got := rec.Count(progress.OutcomeCompleted); got != 3 {
		t.Errorf("completed events = %d, want 3", got)
	}
}

// TestRunnerBypassesLaterStagesAfterAFailure is the property that makes a
// failure cheap: nothing downstream wastes effort on a job that is already
// finished.
func TestRunnerBypassesLaterStagesAfterAFailure(t *testing.T) {
	failing := &recordingStage{name: "failing", fail: errors.New("truncated frame")}
	later := &recordingStage{name: "later"}

	var rec progress.Recorder
	r, err := NewRunner([]StageSpec{
		{Stage: failing, Workers: 1},
		{Stage: later, Workers: 1},
	}, nil, &rec)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if err := r.Run(t.Context(), feed("a", "b")); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := later.runs.Load(); got != 0 {
		t.Errorf("the later stage ran %d times, want 0", got)
	}
	if got := rec.Count(progress.OutcomeFailed); got != 2 {
		t.Errorf("failed events = %d, want 2", got)
	}
	for _, e := range rec.Events() {
		if e.Stage != "failing" {
			t.Errorf("event names stage %q, want the one that failed", e.Stage)
		}
		if e.Err == nil {
			t.Error("failed event carries no error")
		}
	}
}

func TestRunnerBypassesLaterStagesAfterASkip(t *testing.T) {
	skipping := NewStage("screen", func(_ context.Context, j *Job) error {
		j.Skip = report.SkipUpToDate
		return nil
	})
	later := &recordingStage{name: "later"}

	var rec progress.Recorder
	r, err := NewRunner([]StageSpec{
		{Stage: skipping, Workers: 1},
		{Stage: later, Workers: 1},
	}, nil, &rec)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if err := r.Run(t.Context(), feed("a")); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := later.runs.Load(); got != 0 {
		t.Errorf("the later stage ran %d times, want 0", got)
	}
	if got := rec.Count(progress.OutcomeSkipped); got != 1 {
		t.Errorf("skipped events = %d, want 1", got)
	}
	if got := rec.Events()[0].Reason; got != string(report.SkipUpToDate) {
		t.Errorf("skip reason = %q, want %q", got, report.SkipUpToDate)
	}
}

// TestRunnerCarriesOnAfterAFailure checks a bad file does not take the run
// down with it, which matters across a library of fifty thousand.
func TestRunnerCarriesOnAfterAFailure(t *testing.T) {
	stage := NewStage("selective", func(_ context.Context, j *Job) error {
		if j.Path == "bad" {
			return errors.New("broken")
		}
		return nil
	})

	var rec progress.Recorder
	r, err := NewRunner([]StageSpec{{Stage: stage, Workers: 2}}, nil, &rec)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if err := r.Run(t.Context(), feed("good", "bad", "alsogood")); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := rec.Count(progress.OutcomeCompleted); got != 2 {
		t.Errorf("completed = %d, want 2", got)
	}
	if got := rec.Count(progress.OutcomeFailed); got != 1 {
		t.Errorf("failed = %d, want 1", got)
	}
}

func TestRunnerUpdatesCounters(t *testing.T) {
	start := time.Now()
	counters := progress.NewCounters([]string{"first", "second"}, start)

	first := &recordingStage{name: "first"}
	second := &recordingStage{name: "second", fail: errors.New("broken")}

	r, err := NewRunner([]StageSpec{
		{Stage: first, Workers: 1},
		{Stage: second, Workers: 1},
	}, counters, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if err := r.Run(t.Context(), feed("a", "b")); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := counters.Snapshot(start.Add(time.Second))
	if got.Failed != 2 {
		t.Errorf("Failed = %d, want 2", got.Failed)
	}
	if got.Stages[0].Out != 2 {
		t.Errorf("first stage Out = %d, want 2", got.Stages[0].Out)
	}
	if got.Stages[1].Failed != 2 {
		t.Errorf("second stage Failed = %d, want 2", got.Stages[1].Failed)
	}
	for _, s := range got.Stages {
		if s.Queued != 0 {
			t.Errorf("stage %q left %d items queued", s.Name, s.Queued)
		}
	}
}

func TestRunnerWithoutCountersForUnknownStage(t *testing.T) {
	// Counters built for a different pipeline must not panic the runner.
	counters := progress.NewCounters([]string{"elsewhere"}, time.Now())

	r, err := NewRunner([]StageSpec{
		{Stage: &recordingStage{name: "here"}, Workers: 1},
	}, counters, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if err := r.Run(t.Context(), feed("a")); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	stage := &recordingStage{name: "stage"}
	r, err := NewRunner([]StageSpec{{Stage: stage, Workers: 2, Buffer: 1}}, nil, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if err := r.Run(ctx, feed("a", "b", "c")); !errors.Is(err, context.Canceled) {
		t.Errorf("Run() error = %v, want a cancellation", err)
	}
	if got := stage.runs.Load(); got != 0 {
		t.Errorf("stage ran %d times under a cancelled context, want 0", got)
	}
}

// TestRunnerUnwindsWhenCancelledMidRun checks the pipeline drains rather than
// deadlocking when a long run is interrupted.
func TestRunnerUnwindsWhenCancelledMidRun(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	release := make(chan struct{})
	blocking := NewStage("blocking", func(ctx context.Context, _ *Job) error {
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	in := make(chan *Job)
	go func() {
		defer close(in)
		for i := range 100 {
			select {
			case in <- &Job{Path: string(rune('a' + i%26))}:
			case <-ctx.Done():
				return
			}
		}
	}()

	r, err := NewRunner([]StageSpec{
		{Stage: blocking, Workers: 2, Buffer: 1},
		{Stage: &recordingStage{name: "after"}, Workers: 1, Buffer: 1},
	}, nil, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- r.Run(ctx, in) }()

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return after cancellation")
	}
}

func TestRunnerWithoutReporterDoesNotPanic(t *testing.T) {
	r, err := NewRunner([]StageSpec{
		{Stage: &recordingStage{name: "stage"}, Workers: 1},
	}, nil, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	if err := r.Run(t.Context(), feed("a")); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
