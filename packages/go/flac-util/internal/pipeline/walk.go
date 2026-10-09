package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"homelab/flac-util/internal/progress"
)

// DefaultExtension is the file extension collected unless told otherwise.
const DefaultExtension = ".flac"

// DefaultWalkConcurrency is how many directories are read at once.
//
// Reading a directory on network storage costs a round trip rather than a
// seek, so several at a time is a large win there and harmless locally.
const DefaultWalkConcurrency = 16

// WalkOptions configures directory traversal.
type WalkOptions struct {
	// Roots are the directories, or individual files, to collect.
	Roots []string
	// Extensions are the file extensions to collect, compared case
	// insensitively. Empty selects DefaultExtension.
	Extensions []string
	// Concurrency is how many directories are read at once. Zero selects
	// DefaultWalkConcurrency.
	Concurrency int
	// Buffer is the depth of the channel jobs are emitted on.
	Buffer int
}

// Walker finds the files to be collected.
//
// It walks directories concurrently, because on network storage the cost of a
// traversal is dominated by per-directory latency rather than by any work.
type Walker struct {
	opts       WalkOptions
	counters   *progress.Counters
	extensions []string
	now        func() time.Time

	mu   sync.Mutex
	errs []error
}

// NewWalker returns a walker for the given options.
func NewWalker(opts WalkOptions, counters *progress.Counters) (*Walker, error) {
	if len(opts.Roots) == 0 {
		return nil, fmt.Errorf("pipeline: no roots to walk")
	}

	extensions := opts.Extensions
	if len(extensions) == 0 {
		extensions = []string{DefaultExtension}
	}
	normalised := make([]string, 0, len(extensions))
	for _, ext := range extensions {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		normalised = append(normalised, ext)
	}
	if len(normalised) == 0 {
		return nil, fmt.Errorf("pipeline: no usable extensions")
	}

	if opts.Concurrency <= 0 {
		opts.Concurrency = DefaultWalkConcurrency
	}
	if opts.Buffer < 0 {
		opts.Buffer = 0
	}

	return &Walker{
		opts:       opts,
		counters:   counters,
		extensions: normalised,
		now:        time.Now,
	}, nil
}

// Matches reports whether a filename is one the walker collects.
func (w *Walker) Matches(name string) bool {
	return slices.Contains(w.extensions, strings.ToLower(filepath.Ext(name)))
}

// Err returns the errors met during traversal, joined. It is meaningful only
// once the job channel has closed.
//
// Traversal errors are collected rather than fatal: an unreadable directory
// somewhere in a large library should not stop the rest of it being measured.
func (w *Walker) Err() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return errors.Join(w.errs...)
}

// record notes a traversal error.
func (w *Walker) record(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.errs = append(w.errs, err)
}

// Start walks the roots and returns the jobs found. The channel closes when
// traversal finishes or the context is cancelled.
func (w *Walker) Start(ctx context.Context) <-chan *Job {
	out := make(chan *Job, w.opts.Buffer)

	go func() {
		defer close(out)

		// The semaphore bounds how many directory reads are outstanding; the
		// wait group tracks how many directories are still to be visited.
		sem := make(chan struct{}, w.opts.Concurrency)
		var wg sync.WaitGroup

		for _, root := range w.opts.Roots {
			info, err := os.Stat(root)
			if err != nil {
				w.record(err)
				continue
			}
			if !info.IsDir() {
				if w.Matches(root) {
					w.emit(ctx, out, root, info)
				}
				continue
			}
			wg.Add(1)
			go w.walkDir(ctx, out, root, sem, &wg)
		}
		wg.Wait()
	}()

	return out
}

// walkDir reads one directory, emitting the files it wants and recursing into
// subdirectories.
func (w *Walker) walkDir(ctx context.Context, out chan<- *Job, dir string, sem chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	if ctx.Err() != nil {
		return
	}

	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return
	}
	entries, err := os.ReadDir(dir)
	<-sem

	if err != nil {
		w.record(err)
		return
	}

	for _, entry := range entries {
		if ctx.Err() != nil {
			return
		}
		path := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			wg.Add(1)
			go w.walkDir(ctx, out, path, sem, wg)
			continue
		}
		if !w.Matches(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			w.record(err)
			continue
		}
		if !w.emit(ctx, out, path, info) {
			return
		}
	}
}

// emit sends one job, reporting whether it got through before cancellation.
func (w *Walker) emit(ctx context.Context, out chan<- *Job, path string, info os.FileInfo) bool {
	job := &Job{Path: path, Info: info, Started: w.now()}

	select {
	case out <- job:
		if w.counters != nil {
			w.counters.Discovered(1)
		}
		return true
	case <-ctx.Done():
		return false
	}
}
