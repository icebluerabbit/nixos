package progress

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// PlainOptions configures the non-interactive reporter.
type PlainOptions struct {
	// Verbose logs every file rather than only the ones that went wrong.
	Verbose bool
	// Interval is how often a summary line is written. Zero disables them.
	Interval time.Duration
}

// Plain is a Reporter that writes lines to a writer.
//
// It exists because the interactive display is useless anywhere its escape
// codes are not interpreted: a pipe, a log file, a systemd unit. Those are
// where a run of this length is most likely to actually happen.
type Plain struct {
	mu       sync.Mutex
	w        io.Writer
	opts     PlainOptions
	counters *Counters
	now      func() time.Time
	lastLine time.Time
}

// NewPlain returns a reporter writing to w. The counters may be nil, in which
// case no periodic summaries are written.
func NewPlain(w io.Writer, counters *Counters, opts PlainOptions) *Plain {
	return &Plain{w: w, opts: opts, counters: counters, now: time.Now}
}

// Send implements Reporter.
func (p *Plain) Send(e Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch {
	case e.Outcome == OutcomeFailed:
		fmt.Fprintf(p.w, "failed  %s: %v\n", e.Path, e.Err)
	case e.Outcome == OutcomeFaulted:
		fmt.Fprintf(p.w, "faulted %s: %v\n", e.Path, e.Err)
	case p.opts.Verbose && e.Outcome == OutcomeSkipped:
		fmt.Fprintf(p.w, "skipped %s: %s\n", e.Path, e.Reason)
	case p.opts.Verbose:
		fmt.Fprintf(p.w, "done    %s (%s)\n", e.Path, e.Elapsed.Round(time.Millisecond))
	}

	p.maybeSummaryLocked()
}

// maybeSummaryLocked writes a summary if enough time has passed. The caller
// holds the lock.
func (p *Plain) maybeSummaryLocked() {
	if p.counters == nil || p.opts.Interval <= 0 {
		return
	}

	now := p.now()
	if !p.lastLine.IsZero() && now.Sub(p.lastLine) < p.opts.Interval {
		return
	}
	p.lastLine = now
	fmt.Fprintln(p.w, Summarise(p.counters.Snapshot(now)))
}

// Close implements Reporter, writing a final summary.
func (p *Plain) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.counters == nil {
		return nil
	}
	_, err := fmt.Fprintln(p.w, Summarise(p.counters.Snapshot(p.now())))
	return err
}

// Summarise renders a snapshot as one line.
func Summarise(s Snapshot) string {
	return fmt.Sprintf("%d/%d files  %d done  %d skipped  %d faulted  %d failed  %s elapsed  %.1f files/s",
		s.Finished(), s.Discovered,
		s.Completed, s.Skipped, s.Faulted, s.Failed,
		s.Elapsed.Round(time.Second), s.Rate())
}
