// Package pipeline moves files through the stages that measure them.
//
// Every file is one Job, and every Job is owned by exactly one goroutine at a
// time: ownership passes along a channel from stage to stage, so no field on a
// Job ever needs a lock. Each stage has its own pool of workers and its own
// queue, which is what makes the expensive stage tunable without disturbing
// the cheap ones.
package pipeline

import (
	"context"
	"fmt"
	"image"
	"io/fs"
	"sync"
	"time"

	"homelab/flac-util/internal/audio"
	"homelab/flac-util/internal/progress"
	"homelab/flac-util/internal/report"
	"homelab/flac-util/internal/spectrogram"
)

// Job is one file and everything learned about it so far.
//
// Fields are filled in by the stage that produces them and cleared by the
// stage that last needs them, which is how the memory held by work in flight
// stays bounded.
type Job struct {
	Path    string
	Info    fs.FileInfo
	Started time.Time

	// Skip is set when the file needs no work.
	Skip report.SkipReason
	// Err is set when a stage failed, along with the stage that failed.
	Err         error
	FailedStage string

	Metadata *audio.Metadata
	Decode   audio.DecodeResult
	// DecodeErr is set when the audio stopped short but enough of it was read
	// to be worth reporting. The job carries on to write that report, so this
	// is a fault recorded rather than a failure raised.
	DecodeErr error

	// FileHash, Verify and Fingerprint come from the stages that read the file
	// again rather than from the decode.
	FileHash    string
	Verify      audio.VerifyResult
	Fingerprint audio.FingerprintResult
	Levels      *spectrogram.Levels
	Image       *image.Paletted
	PNG         []byte
	Report      *report.Report
	JSON        []byte
}

// Terminal reports whether the job has already finished, either by failing or
// by being skipped. Terminal jobs pass through the remaining stages untouched.
func (j *Job) Terminal() bool { return j.Err != nil || j.Skip != report.SkipNone }

// Size returns the file's size, or zero when it is unknown.
func (j *Job) Size() int64 {
	if j.Info == nil {
		return 0
	}
	return j.Info.Size()
}

// fail records an error against the job.
func (j *Job) fail(stage string, err error) {
	j.Err = err
	j.FailedStage = stage
}

// Stage is one step of the pipeline.
//
// Implementations receive jobs that are not terminal, one at a time, from
// however many workers the stage was configured with. They must be safe to
// call concurrently, but need no synchronisation of their own: each job
// belongs to its caller for the duration.
type Stage interface {
	// Name identifies the stage in configuration and in progress reporting.
	Name() string
	// Process advances the job by one step.
	Process(ctx context.Context, j *Job) error
}

// StageFunc adapts a function into a Stage.
type StageFunc struct {
	name string
	fn   func(context.Context, *Job) error
}

// NewStage returns a Stage backed by a function.
func NewStage(name string, fn func(context.Context, *Job) error) *StageFunc {
	return &StageFunc{name: name, fn: fn}
}

// Name implements Stage.
func (s *StageFunc) Name() string { return s.name }

// Process implements Stage.
func (s *StageFunc) Process(ctx context.Context, j *Job) error { return s.fn(ctx, j) }

// StageSpec is a stage together with the resources given to it.
type StageSpec struct {
	Stage Stage
	// Workers is how many goroutines run the stage at once.
	Workers int
	// Buffer is the depth of the queue feeding the stage. Shallow queues after
	// an expensive stage keep memory down; deep queues before one keep it fed.
	Buffer int
}

// Validate reports whether the specification can be run.
func (s StageSpec) Validate() error {
	switch {
	case s.Stage == nil:
		return fmt.Errorf("pipeline: stage is nil")
	case s.Stage.Name() == "":
		return fmt.Errorf("pipeline: stage has no name")
	case s.Workers < 1:
		return fmt.Errorf("pipeline: stage %q needs at least one worker, got %d", s.Stage.Name(), s.Workers)
	case s.Buffer < 0:
		return fmt.Errorf("pipeline: stage %q has a negative buffer", s.Stage.Name())
	}
	return nil
}

// Runner wires a sequence of stages together and drives jobs through them.
type Runner struct {
	specs    []StageSpec
	counters *progress.Counters
	reporter progress.Reporter
	now      func() time.Time
}

// NewRunner returns a runner for the given stages, which are run in order.
//
// The counters and reporter may be nil, in which case the run is silent.
func NewRunner(specs []StageSpec, counters *progress.Counters, reporter progress.Reporter) (*Runner, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("pipeline: no stages")
	}
	seen := make(map[string]bool, len(specs))
	for _, spec := range specs {
		if err := spec.Validate(); err != nil {
			return nil, err
		}
		if seen[spec.Stage.Name()] {
			return nil, fmt.Errorf("pipeline: duplicate stage %q", spec.Stage.Name())
		}
		seen[spec.Stage.Name()] = true
	}

	if reporter == nil {
		reporter = progress.Discard{}
	}
	return &Runner{specs: specs, counters: counters, reporter: reporter, now: time.Now}, nil
}

// StageNames returns the runner's stages in order.
func (r *Runner) StageNames() []string {
	out := make([]string, 0, len(r.specs))
	for _, spec := range r.specs {
		out = append(out, spec.Stage.Name())
	}
	return out
}

// Run drives every job from in through all the stages, returning once the
// input is exhausted and every job has reached the end.
//
// A job that fails is not retried and does not stop the run: the failure is
// recorded against the file and the pipeline carries on. Cancelling the
// context unwinds every stage in order.
func (r *Runner) Run(ctx context.Context, in <-chan *Job) error {
	current := in
	for _, spec := range r.specs {
		current = r.runStage(ctx, spec, current)
	}
	for job := range current {
		r.finish(job)
	}
	return ctx.Err()
}

// runStage starts one stage's workers and returns the channel they feed.
func (r *Runner) runStage(ctx context.Context, spec StageSpec, in <-chan *Job) <-chan *Job {
	out := make(chan *Job, spec.Buffer)

	var counters *progress.StageCounters
	if r.counters != nil {
		counters = r.counters.Stage(spec.Stage.Name())
	}

	var wg sync.WaitGroup
	for range spec.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range in {
				r.process(ctx, spec.Stage, counters, job)
				select {
				case out <- job:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// process runs one stage against one job, timing it and recording the outcome.
func (r *Runner) process(ctx context.Context, stage Stage, counters *progress.StageCounters, job *Job) {
	if job.Terminal() {
		return
	}
	if err := ctx.Err(); err != nil {
		job.fail(stage.Name(), err)
		return
	}

	if counters != nil {
		counters.Enter()
	}
	start := r.now()
	err := stage.Process(ctx, job)
	if counters != nil {
		counters.Leave(r.now().Sub(start), err)
	}
	if err != nil {
		job.fail(stage.Name(), err)
	}
}

// finish records a job's terminal state and reports it.
func (r *Runner) finish(job *Job) {
	event := progress.Event{
		Path:  job.Path,
		Stage: job.FailedStage,
	}
	if !job.Started.IsZero() {
		event.Elapsed = r.now().Sub(job.Started)
	}

	switch {
	case job.Err != nil:
		event.Outcome = progress.OutcomeFailed
		event.Err = job.Err
		if r.counters != nil {
			r.counters.Failed()
		}
	case job.DecodeErr != nil:
		// The report was written and the fault is in it. Counting this as a
		// success would bury a damaged file among the good ones; counting it
		// as a failure would say nothing was produced, when something was.
		event.Outcome = progress.OutcomeFaulted
		event.Err = job.DecodeErr
		event.Stage = StageDecode
		if r.counters != nil {
			r.counters.Faulted(job.Size())
		}
	case job.Skip != report.SkipNone:
		event.Outcome = progress.OutcomeSkipped
		event.Reason = string(job.Skip)
		if r.counters != nil {
			r.counters.Skipped()
		}
	default:
		event.Outcome = progress.OutcomeCompleted
		if r.counters != nil {
			r.counters.Completed(job.Size())
		}
	}
	r.reporter.Send(event)
}
