package progress

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestOutcomeString(t *testing.T) {
	tests := []struct {
		outcome Outcome
		want    string
	}{
		{OutcomeCompleted, "completed"},
		{OutcomeSkipped, "skipped"},
		{OutcomeFailed, "failed"},
		{Outcome(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.outcome.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiscardIgnoresEverything(t *testing.T) {
	var r Reporter = Discard{}
	r.Send(Event{Outcome: OutcomeFailed, Err: errors.New("ignored")})
	if err := r.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestRecorderCollectsEvents(t *testing.T) {
	var r Recorder

	r.Send(Event{Outcome: OutcomeCompleted, Path: "a.flac", Elapsed: time.Second})
	r.Send(Event{Outcome: OutcomeSkipped, Path: "b.flac", Reason: "already reported"})
	r.Send(Event{Outcome: OutcomeFailed, Path: "c.flac", Err: errors.New("broken")})

	events := r.Events()
	if len(events) != 3 {
		t.Fatalf("len(Events()) = %d, want 3", len(events))
	}
	if events[0].Path != "a.flac" || events[0].Elapsed != time.Second {
		t.Errorf("first event = %+v", events[0])
	}
	if events[1].Reason != "already reported" {
		t.Errorf("second event reason = %q", events[1].Reason)
	}
	if events[2].Err == nil {
		t.Error("third event has no error")
	}
}

func TestRecorderCount(t *testing.T) {
	var r Recorder
	r.Send(Event{Outcome: OutcomeCompleted})
	r.Send(Event{Outcome: OutcomeCompleted})
	r.Send(Event{Outcome: OutcomeFailed})

	tests := []struct {
		outcome Outcome
		want    int
	}{
		{OutcomeCompleted, 2},
		{OutcomeFailed, 1},
		{OutcomeSkipped, 0},
	}
	for _, tt := range tests {
		t.Run(tt.outcome.String(), func(t *testing.T) {
			if got := r.Count(tt.outcome); got != tt.want {
				t.Errorf("Count(%v) = %d, want %d", tt.outcome, got, tt.want)
			}
		})
	}
}

func TestRecorderEventsIsACopy(t *testing.T) {
	var r Recorder
	r.Send(Event{Path: "a.flac"})

	events := r.Events()
	events[0].Path = "tampered"

	if got := r.Events()[0].Path; got != "a.flac" {
		t.Errorf("Events()[0].Path = %q, want the original", got)
	}
}

func TestRecorderClose(t *testing.T) {
	var r Recorder
	if r.Closed() {
		t.Error("Closed() = true before Close()")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !r.Closed() {
		t.Error("Closed() = false after Close()")
	}
}

// TestRecorderIsConcurrencySafe matches the contract every Reporter has to
// meet, since the pipeline sends from many workers at once.
func TestRecorderIsConcurrencySafe(t *testing.T) {
	const workers, each = 8, 200
	var r Recorder

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				r.Send(Event{Outcome: OutcomeCompleted})
				_ = r.Events()
			}
		}()
	}
	wg.Wait()

	if got := r.Count(OutcomeCompleted); got != workers*each {
		t.Errorf("Count() = %d, want %d", got, workers*each)
	}
}
