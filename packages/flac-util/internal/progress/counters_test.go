package progress

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

var testStart = time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)

func TestNewCountersHasEveryStage(t *testing.T) {
	names := []string{"screen", "probe", "decode"}
	c := NewCounters(names, testStart)

	if got := c.StageNames(); !slices.Equal(got, names) {
		t.Errorf("StageNames() = %v, want %v", got, names)
	}
	for _, name := range names {
		if c.Stage(name) == nil {
			t.Errorf("Stage(%q) = nil, want counters", name)
		}
	}
	if c.Stage("absent") != nil {
		t.Error("Stage() returned counters for an unknown stage")
	}
}

func TestStageNamesIsACopy(t *testing.T) {
	c := NewCounters([]string{"decode"}, testStart)

	names := c.StageNames()
	names[0] = "tampered"

	if got := c.StageNames(); got[0] != "decode" {
		t.Errorf("StageNames() = %v, want the original names", got)
	}
}

func TestStageCounters(t *testing.T) {
	c := NewCounters([]string{"decode"}, testStart)
	stage := c.Stage("decode")

	stage.Enter()
	stage.Enter()
	if got := stage.Snapshot("decode").Queued; got != 2 {
		t.Errorf("Queued = %d, want 2", got)
	}

	stage.Leave(time.Second, nil)
	stage.Leave(2*time.Second, errors.New("broken"))

	got := stage.Snapshot("decode")
	if got.Name != "decode" {
		t.Errorf("Name = %q, want decode", got.Name)
	}
	if got.In != 2 {
		t.Errorf("In = %d, want 2", got.In)
	}
	if got.Out != 1 {
		t.Errorf("Out = %d, want 1", got.Out)
	}
	if got.Failed != 1 {
		t.Errorf("Failed = %d, want 1", got.Failed)
	}
	if got.Queued != 0 {
		t.Errorf("Queued = %d, want 0", got.Queued)
	}
	if got.Busy != 3*time.Second {
		t.Errorf("Busy = %v, want 3s", got.Busy)
	}
}

func TestSnapshotTotals(t *testing.T) {
	c := NewCounters([]string{"decode"}, testStart)

	c.Discovered(10)
	c.Completed(1000)
	c.Completed(2000)
	c.Skipped()
	c.Failed()

	got := c.Snapshot(testStart.Add(4 * time.Second))
	if got.Discovered != 10 {
		t.Errorf("Discovered = %d, want 10", got.Discovered)
	}
	if got.Completed != 2 {
		t.Errorf("Completed = %d, want 2", got.Completed)
	}
	if got.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", got.Skipped)
	}
	if got.Failed != 1 {
		t.Errorf("Failed = %d, want 1", got.Failed)
	}
	if got.Bytes != 3000 {
		t.Errorf("Bytes = %d, want 3000", got.Bytes)
	}
	if got.Elapsed != 4*time.Second {
		t.Errorf("Elapsed = %v, want 4s", got.Elapsed)
	}
	if got.Finished() != 4 {
		t.Errorf("Finished() = %d, want 4", got.Finished())
	}
	if got.Remaining() != 6 {
		t.Errorf("Remaining() = %d, want 6", got.Remaining())
	}
	if want := 1.0; got.Rate() != want {
		t.Errorf("Rate() = %v, want %v", got.Rate(), want)
	}
}

func TestSnapshotFraction(t *testing.T) {
	tests := []struct {
		name       string
		discovered int64
		completed  int
		want       float64
	}{
		{"nothing discovered yet", 0, 0, 0},
		{"half done", 4, 2, 0.5},
		{"all done", 4, 4, 1},
		{"more finished than discovered clamps", 2, 4, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCounters(nil, testStart)
			c.Discovered(tt.discovered)
			for range tt.completed {
				c.Completed(0)
			}

			if got := c.Snapshot(testStart).Fraction(); got != tt.want {
				t.Errorf("Fraction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSnapshotRemainingNeverGoesNegative(t *testing.T) {
	c := NewCounters(nil, testStart)
	c.Discovered(1)
	c.Completed(0)
	c.Completed(0)

	if got := c.Snapshot(testStart).Remaining(); got != 0 {
		t.Errorf("Remaining() = %d, want 0", got)
	}
}

func TestSnapshotBeforeAnyTimePasses(t *testing.T) {
	c := NewCounters(nil, testStart)
	c.Completed(0)

	got := c.Snapshot(testStart)
	if got.Elapsed != 0 {
		t.Errorf("Elapsed = %v, want 0", got.Elapsed)
	}
	if got.Rate() != 0 {
		t.Errorf("Rate() = %v, want 0", got.Rate())
	}
}

func TestSnapshotWithZeroStartTime(t *testing.T) {
	c := NewCounters(nil, time.Time{})
	if got := c.Snapshot(time.Now()).Elapsed; got != 0 {
		t.Errorf("Elapsed = %v, want 0 when the run has no start time", got)
	}
}

// TestCountersAreConcurrencySafe is what the atomics exist for. Run with -race
// this is the test that would catch a lock creeping into the hot path.
func TestCountersAreConcurrencySafe(t *testing.T) {
	const workers, each = 8, 500
	c := NewCounters([]string{"decode"}, testStart)
	stage := c.Stage("decode")

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				stage.Enter()
				stage.Leave(time.Millisecond, nil)
				c.Completed(1)
			}
		}()
	}

	// Snapshots taken while the work runs must not race with it.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 100 {
			_ = c.Snapshot(time.Now())
		}
	}()

	wg.Wait()
	<-done

	got := c.Snapshot(testStart.Add(time.Second))
	if want := int64(workers * each); got.Completed != want {
		t.Errorf("Completed = %d, want %d", got.Completed, want)
	}
	if got.Stages[0].Out != int64(workers*each) {
		t.Errorf("stage Out = %d, want %d", got.Stages[0].Out, workers*each)
	}
}
