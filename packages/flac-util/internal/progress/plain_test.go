package progress

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPlainAlwaysReportsFailures(t *testing.T) {
	var buf bytes.Buffer
	p := NewPlain(&buf, nil, PlainOptions{})

	p.Send(Event{Outcome: OutcomeFailed, Path: "bad.flac", Err: errors.New("truncated frame")})

	got := buf.String()
	if !strings.Contains(got, "bad.flac") || !strings.Contains(got, "truncated frame") {
		t.Errorf("output = %q, want the path and the error", got)
	}
}

func TestPlainIsQuietAboutSuccessByDefault(t *testing.T) {
	var buf bytes.Buffer
	p := NewPlain(&buf, nil, PlainOptions{})

	p.Send(Event{Outcome: OutcomeCompleted, Path: "good.flac"})
	p.Send(Event{Outcome: OutcomeSkipped, Path: "old.flac", Reason: "already reported"})

	if buf.Len() != 0 {
		t.Errorf("output = %q, want nothing without verbose", buf.String())
	}
}

func TestPlainVerboseReportsEverything(t *testing.T) {
	var buf bytes.Buffer
	p := NewPlain(&buf, nil, PlainOptions{Verbose: true})

	p.Send(Event{Outcome: OutcomeCompleted, Path: "good.flac", Elapsed: 1500 * time.Millisecond})
	p.Send(Event{Outcome: OutcomeSkipped, Path: "old.flac", Reason: "already reported"})

	got := buf.String()
	for _, want := range []string{"good.flac", "1.5s", "old.flac", "already reported"} {
		if !strings.Contains(got, want) {
			t.Errorf("output = %q, want it to contain %q", got, want)
		}
	}
}

func TestPlainWritesPeriodicSummaries(t *testing.T) {
	var buf bytes.Buffer
	c := NewCounters(nil, testStart)
	c.Discovered(10)

	p := NewPlain(&buf, c, PlainOptions{Interval: time.Second})
	now := testStart
	p.now = func() time.Time { return now }

	// The first event establishes the baseline and prints a summary.
	p.Send(Event{Outcome: OutcomeCompleted})
	first := strings.Count(buf.String(), "elapsed")
	if first != 1 {
		t.Fatalf("wrote %d summaries on the first event, want 1", first)
	}

	// A second event too soon afterwards must not print another.
	now = testStart.Add(100 * time.Millisecond)
	p.Send(Event{Outcome: OutcomeCompleted})
	if got := strings.Count(buf.String(), "elapsed"); got != 1 {
		t.Errorf("wrote %d summaries, want the interval to suppress the second", got)
	}

	// Once the interval has passed, another appears.
	now = testStart.Add(2 * time.Second)
	p.Send(Event{Outcome: OutcomeCompleted})
	if got := strings.Count(buf.String(), "elapsed"); got != 2 {
		t.Errorf("wrote %d summaries, want 2 after the interval elapsed", got)
	}
}

func TestPlainWithoutIntervalWritesNoSummaries(t *testing.T) {
	var buf bytes.Buffer
	c := NewCounters(nil, testStart)

	p := NewPlain(&buf, c, PlainOptions{})
	p.Send(Event{Outcome: OutcomeCompleted})

	if strings.Contains(buf.String(), "elapsed") {
		t.Errorf("output = %q, want no summary when the interval is zero", buf.String())
	}
}

func TestPlainCloseWritesAFinalSummary(t *testing.T) {
	var buf bytes.Buffer
	c := NewCounters(nil, testStart)
	c.Discovered(3)
	c.Completed(0)
	c.Skipped()
	c.Failed()

	p := NewPlain(&buf, c, PlainOptions{})
	p.now = func() time.Time { return testStart.Add(3 * time.Second) }

	if err := p.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	got := buf.String()
	for _, want := range []string{"3/3 files", "1 done", "1 skipped", "1 failed", "3s elapsed"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary = %q, want it to contain %q", got, want)
		}
	}
}

func TestPlainCloseWithoutCounters(t *testing.T) {
	var buf bytes.Buffer
	p := NewPlain(&buf, nil, PlainOptions{})

	if err := p.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("output = %q, want nothing without counters", buf.String())
	}
}

func TestSummarise(t *testing.T) {
	got := Summarise(Snapshot{
		Discovered: 100,
		Completed:  60,
		Skipped:    30,
		Failed:     2,
		Elapsed:    46 * time.Second,
	})

	for _, want := range []string{"92/100 files", "60 done", "30 skipped", "2 failed", "46s elapsed", "2.0 files/s"} {
		if !strings.Contains(got, want) {
			t.Errorf("Summarise() = %q, want it to contain %q", got, want)
		}
	}
}

func TestPlainIsConcurrencySafe(t *testing.T) {
	const workers, each = 8, 100
	var buf bytes.Buffer
	p := NewPlain(&buf, nil, PlainOptions{Verbose: true})

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				p.Send(Event{Outcome: OutcomeCompleted, Path: "track.flac"})
			}
		}()
	}
	wg.Wait()

	if got := strings.Count(buf.String(), "track.flac"); got != workers*each {
		t.Errorf("wrote %d lines, want %d", got, workers*each)
	}
}
