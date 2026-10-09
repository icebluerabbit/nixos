package progress

import (
	"sync"
	"time"
)

// Outcome is how a file's journey through the pipeline ended.
type Outcome int

// The terminal states.
const (
	// OutcomeCompleted means the file was measured and its reports written.
	OutcomeCompleted Outcome = iota
	// OutcomeSkipped means the file already had a current report.
	OutcomeSkipped
	// OutcomeFaulted means the file was reported, but its audio did not read
	// cleanly and the report says so. It is neither a success nor a failure:
	// the work was done and the result is worth reading, with a caveat that
	// has to travel with it.
	OutcomeFaulted
	// OutcomeFailed means the file could not be processed.
	OutcomeFailed
)

// String implements fmt.Stringer.
func (o Outcome) String() string {
	switch o {
	case OutcomeCompleted:
		return "completed"
	case OutcomeSkipped:
		return "skipped"
	case OutcomeFaulted:
		return "faulted"
	case OutcomeFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Event is one file reaching the end of the pipeline.
//
// Only terminal events travel this way. Per-stage progress lives in the
// counters instead, because at full speed there are far too many of those to
// be worth delivering one at a time.
type Event struct {
	Outcome Outcome
	Path    string
	// Stage names where the work ended, which for a failure is where it broke.
	Stage string
	// Reason explains a skip.
	Reason string
	// Err is set for a failure.
	Err error
	// Elapsed is how long the file took end to end.
	Elapsed time.Duration
}

// Reporter receives the events of a run.
//
// Implementations must be safe to call from many goroutines and must never
// block for long: a slow reporter would become the pipeline's bottleneck.
type Reporter interface {
	// Send delivers one terminal event.
	Send(Event)
	// Close releases whatever the reporter holds. It is called once, after
	// the last event.
	Close() error
}

// Discard is a Reporter that ignores everything.
type Discard struct{}

// Send implements Reporter.
func (Discard) Send(Event) {}

// Close implements Reporter.
func (Discard) Close() error { return nil }

// Recorder is a Reporter that keeps every event, for tests.
type Recorder struct {
	mu     sync.Mutex
	events []Event
	closed bool
}

// Send implements Reporter.
func (r *Recorder) Send(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

// Close implements Reporter.
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

// Events returns a copy of everything received so far.
func (r *Recorder) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}

// Closed reports whether Close has been called.
func (r *Recorder) Closed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

// Count returns how many events of an outcome were received.
func (r *Recorder) Count(o Outcome) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	var n int
	for _, e := range r.events {
		if e.Outcome == o {
			n++
		}
	}
	return n
}
