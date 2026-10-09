// Package progress carries the state of a run from the workers doing it to
// whatever is displaying it.
//
// The collector depends on this package and on nothing that draws. That is
// what lets the same run be watched interactively, logged to a file, or
// observed by a test.
package progress

import (
	"slices"
	"sync/atomic"
	"time"
)

// StageCounters is the running tally for one stage of the pipeline.
//
// Workers update these on every item, so they are atomics rather than
// anything guarded by a lock: a display that falls behind must never be able
// to slow the work down.
type StageCounters struct {
	in     atomic.Int64
	out    atomic.Int64
	failed atomic.Int64
	queued atomic.Int64
	nanos  atomic.Int64
}

// Enter records an item arriving at the stage.
func (c *StageCounters) Enter() {
	c.in.Add(1)
	c.queued.Add(1)
}

// Leave records an item finishing the stage, successfully or not.
func (c *StageCounters) Leave(d time.Duration, err error) {
	c.queued.Add(-1)
	c.nanos.Add(int64(d))
	if err != nil {
		c.failed.Add(1)
		return
	}
	c.out.Add(1)
}

// StageSnapshot is a consistent-enough view of one stage for display.
type StageSnapshot struct {
	Name   string
	In     int64
	Out    int64
	Failed int64
	// Queued is how many items are in the stage right now, which is the
	// number that says where a run is bottlenecked.
	Queued int64
	// Busy is the total time workers spent inside the stage. Divided by the
	// worker count it approximates how hard the stage is working.
	Busy time.Duration
}

// Snapshot reads the stage's counters.
func (c *StageCounters) Snapshot(name string) StageSnapshot {
	return StageSnapshot{
		Name:   name,
		In:     c.in.Load(),
		Out:    c.out.Load(),
		Failed: c.failed.Load(),
		Queued: c.queued.Load(),
		Busy:   time.Duration(c.nanos.Load()),
	}
}

// Counters is the whole run's tally.
type Counters struct {
	discovered atomic.Int64
	completed  atomic.Int64
	skipped    atomic.Int64
	faulted    atomic.Int64
	failed     atomic.Int64
	bytes      atomic.Int64

	// stages is fixed once NewCounters returns, so reading it concurrently
	// needs no synchronisation.
	stages  map[string]*StageCounters
	order   []string
	started time.Time
}

// NewCounters returns counters for a pipeline with the named stages, in order.
func NewCounters(stageNames []string, started time.Time) *Counters {
	c := &Counters{
		stages:  make(map[string]*StageCounters, len(stageNames)),
		order:   slices.Clone(stageNames),
		started: started,
	}
	for _, name := range stageNames {
		c.stages[name] = &StageCounters{}
	}
	return c
}

// Stage returns the counters for a named stage, or nil when there is no such
// stage.
func (c *Counters) Stage(name string) *StageCounters { return c.stages[name] }

// StageNames returns the stage names in pipeline order.
func (c *Counters) StageNames() []string { return slices.Clone(c.order) }

// Discovered records files found by the walker.
func (c *Counters) Discovered(n int64) { c.discovered.Add(n) }

// Completed records a file that produced output.
func (c *Counters) Completed(bytesRead int64) {
	c.completed.Add(1)
	c.bytes.Add(bytesRead)
}

// Skipped records a file that needed no work.
func (c *Counters) Skipped() { c.skipped.Add(1) }

// Faulted records a file that produced a report despite a fault in its audio.
func (c *Counters) Faulted(bytesRead int64) {
	c.faulted.Add(1)
	c.bytes.Add(bytesRead)
}

// Failed records a file that could not be processed.
func (c *Counters) Failed() { c.failed.Add(1) }

// Snapshot is a view of the whole run.
type Snapshot struct {
	Discovered int64
	Completed  int64
	Skipped    int64
	Faulted    int64
	Failed     int64
	Bytes      int64
	Elapsed    time.Duration
	Stages     []StageSnapshot
}

// Finished returns how many files have reached a terminal state.
func (s Snapshot) Finished() int64 {
	return s.Completed + s.Skipped + s.Faulted + s.Failed
}

// Remaining returns how many discovered files are still outstanding.
func (s Snapshot) Remaining() int64 { return max(s.Discovered-s.Finished(), 0) }

// Fraction returns progress through the run, in [0,1]. It is zero before
// anything has been discovered.
func (s Snapshot) Fraction() float64 {
	if s.Discovered <= 0 {
		return 0
	}
	return min(float64(s.Finished())/float64(s.Discovered), 1)
}

// Rate returns completed files per second, or zero before any time has passed.
func (s Snapshot) Rate() float64 {
	if s.Elapsed <= 0 {
		return 0
	}
	return float64(s.Finished()) / s.Elapsed.Seconds()
}

// Snapshot reads the whole tally. The individual counters are read one after
// another rather than under a lock, so a snapshot taken mid-flight can be
// very slightly inconsistent; for a progress display that is a fair trade for
// never blocking a worker.
func (c *Counters) Snapshot(now time.Time) Snapshot {
	s := Snapshot{
		Discovered: c.discovered.Load(),
		Completed:  c.completed.Load(),
		Skipped:    c.skipped.Load(),
		Faulted:    c.faulted.Load(),
		Failed:     c.failed.Load(),
		Bytes:      c.bytes.Load(),
		Stages:     make([]StageSnapshot, 0, len(c.order)),
	}
	if !c.started.IsZero() && now.After(c.started) {
		s.Elapsed = now.Sub(c.started)
	}
	for _, name := range c.order {
		s.Stages = append(s.Stages, c.stages[name].Snapshot(name))
	}
	return s
}
