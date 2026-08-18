package tui

import (
	"sync"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"

	"flac-util/internal/progress"
)

// DefaultQueueDepth is how many events the reporter will hold before it starts
// discarding the unimportant ones.
const DefaultQueueDepth = 256

// EventMsg carries one terminal event into the display.
type EventMsg progress.Event

// DroppedMsg reports that events were discarded because the display could not
// keep up.
type DroppedMsg int64

// sender is the part of a Bubble Tea program the reporter needs, kept as an
// interface so the reporter can be tested without a terminal.
type sender interface {
	Send(msg tea.Msg)
}

// Reporter forwards pipeline events into a running display.
//
// It exists to enforce one rule: the workers must never wait for the screen.
// Events are handed to a bounded queue and a forwarding goroutine drains it,
// so a display that falls behind loses detail rather than throughput. Failures
// are the exception and are never discarded, because they are the whole point
// of watching.
type Reporter struct {
	program sender
	queue   chan progress.Event
	done    chan struct{}
	closing chan struct{}
	once    sync.Once
	dropped atomic.Int64
}

// NewReporter returns a reporter forwarding to the given program. A depth of
// zero selects DefaultQueueDepth.
func NewReporter(program sender, depth int) *Reporter {
	if depth <= 0 {
		depth = DefaultQueueDepth
	}

	r := &Reporter{
		program: program,
		queue:   make(chan progress.Event, depth),
		done:    make(chan struct{}),
		closing: make(chan struct{}),
	}
	go r.forward()
	return r
}

// forward drains the queue into the program.
func (r *Reporter) forward() {
	defer close(r.done)
	for event := range r.queue {
		r.program.Send(EventMsg(event))
	}
	if n := r.dropped.Load(); n > 0 {
		r.program.Send(DroppedMsg(n))
	}
}

// Send implements progress.Reporter.
func (r *Reporter) Send(e progress.Event) {
	select {
	case <-r.closing:
		return
	default:
	}

	select {
	case r.queue <- e:
		return
	default:
	}

	// The queue is full. A failure is worth waiting for; anything else is
	// detail the reader will not miss.
	if e.Outcome == progress.OutcomeFailed {
		select {
		case r.queue <- e:
		case <-r.closing:
		}
		return
	}
	r.dropped.Add(1)
}

// Dropped returns how many events were discarded.
func (r *Reporter) Dropped() int64 { return r.dropped.Load() }

// Close implements progress.Reporter, flushing whatever is still queued.
func (r *Reporter) Close() error {
	r.once.Do(func() {
		close(r.closing)
		close(r.queue)
		<-r.done
	})
	return nil
}
