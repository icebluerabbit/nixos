package tui

import (
	"errors"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"homelab/flac-util/internal/progress"
)

// fakeSender records what the reporter forwards, and can be made to block so
// that a slow display can be simulated.
type fakeSender struct {
	mu   sync.Mutex
	msgs []tea.Msg

	gate chan struct{}
}

func (s *fakeSender) Send(msg tea.Msg) {
	if s.gate != nil {
		<-s.gate
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs = append(s.msgs, msg)
}

func (s *fakeSender) Messages() []tea.Msg {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]tea.Msg(nil), s.msgs...)
}

func (s *fakeSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.msgs)
}

func TestReporterForwardsEvents(t *testing.T) {
	sender := &fakeSender{}
	r := NewReporter(sender, 0)

	r.Send(progress.Event{Outcome: progress.OutcomeCompleted, Path: "a.flac"})
	r.Send(progress.Event{Outcome: progress.OutcomeSkipped, Path: "b.flac"})
	if err := r.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	msgs := sender.Messages()
	if len(msgs) != 2 {
		t.Fatalf("forwarded %d messages, want 2", len(msgs))
	}
	first, ok := msgs[0].(EventMsg)
	if !ok {
		t.Fatalf("message is %T, want EventMsg", msgs[0])
	}
	if first.Path != "a.flac" {
		t.Errorf("first message path = %q, want a.flac", first.Path)
	}
}

func TestReporterUsesTheDefaultDepth(t *testing.T) {
	r := NewReporter(&fakeSender{}, 0)
	defer r.Close()

	if got := cap(r.queue); got != DefaultQueueDepth {
		t.Errorf("queue depth = %d, want %d", got, DefaultQueueDepth)
	}
}

// TestReporterDropsRatherThanBlocking is the property the whole type exists
// for: a display that cannot keep up must cost detail, never throughput.
func TestReporterDropsRatherThanBlocking(t *testing.T) {
	sender := &fakeSender{gate: make(chan struct{})}
	r := NewReporter(sender, 1)

	// Fill the queue and jam the forwarder.
	for range 50 {
		r.Send(progress.Event{Outcome: progress.OutcomeCompleted, Path: "track.flac"})
	}

	if got := r.Dropped(); got == 0 {
		t.Fatal("Dropped() = 0, want the reporter to have discarded events")
	}

	close(sender.gate)
	if err := r.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

// TestReporterNeverDropsFailures covers the exception: a failure is the one
// thing a reader must not miss.
func TestReporterNeverDropsFailures(t *testing.T) {
	sender := &fakeSender{gate: make(chan struct{})}
	r := NewReporter(sender, 1)

	// One event is taken by the forwarder, which then blocks on the gate; the
	// second fills the queue. Only now is the reporter genuinely backed up.
	r.Send(progress.Event{Outcome: progress.OutcomeCompleted})
	waitForForwarder(t, r)
	r.Send(progress.Event{Outcome: progress.OutcomeCompleted})

	sent := make(chan struct{})
	go func() {
		defer close(sent)
		r.Send(progress.Event{Outcome: progress.OutcomeFailed, Err: errors.New("broken")})
	}()

	// The failure must still be waiting rather than discarded.
	select {
	case <-sent:
		t.Fatal("the failure returned immediately, so it was discarded")
	case <-time.After(50 * time.Millisecond):
	}

	close(sender.gate)
	<-sent
	if err := r.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var failures int
	for _, msg := range sender.Messages() {
		if e, ok := msg.(EventMsg); ok && e.Outcome == progress.OutcomeFailed {
			failures++
		}
	}
	if failures != 1 {
		t.Errorf("forwarded %d failures, want 1", failures)
	}
}

// waitForForwarder blocks until the forwarding goroutine has taken an event
// off the queue, which is what makes the queue's depth predictable in tests.
func waitForForwarder(t *testing.T, r *Reporter) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for len(r.queue) > 0 {
		if time.Now().After(deadline) {
			t.Fatal("the forwarding goroutine never drained the queue")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestReporterAnnouncesDroppedEvents(t *testing.T) {
	sender := &fakeSender{gate: make(chan struct{})}
	r := NewReporter(sender, 1)

	for range 20 {
		r.Send(progress.Event{Outcome: progress.OutcomeCompleted})
	}
	close(sender.gate)
	if err := r.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var announced bool
	for _, msg := range sender.Messages() {
		if n, ok := msg.(DroppedMsg); ok && n > 0 {
			announced = true
		}
	}
	if !announced {
		t.Error("no DroppedMsg was sent, so the reader would never know detail was lost")
	}
}

func TestReporterCloseIsIdempotent(t *testing.T) {
	r := NewReporter(&fakeSender{}, 4)

	if err := r.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestReporterIgnoresSendsAfterClose(t *testing.T) {
	sender := &fakeSender{}
	r := NewReporter(sender, 4)

	if err := r.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	before := sender.count()

	// This must not panic on the closed channel.
	r.Send(progress.Event{Outcome: progress.OutcomeCompleted})

	if got := sender.count(); got != before {
		t.Errorf("sent %d messages after close, want %d", got, before)
	}
}

func TestReporterIsConcurrencySafe(t *testing.T) {
	const workers, each = 8, 100
	sender := &fakeSender{}
	r := NewReporter(sender, 32)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				r.Send(progress.Event{Outcome: progress.OutcomeCompleted})
			}
		}()
	}
	wg.Wait()

	if err := r.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Nothing may be lost except by the deliberate drop path.
	forwarded := int64(0)
	for _, msg := range sender.Messages() {
		if _, ok := msg.(EventMsg); ok {
			forwarded++
		}
	}
	if forwarded+r.Dropped() != workers*each {
		t.Errorf("forwarded %d and dropped %d, want them to total %d",
			forwarded, r.Dropped(), workers*each)
	}
}
