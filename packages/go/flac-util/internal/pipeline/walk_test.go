package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"homelab/flac-util/internal/progress"
)

// collectPaths drains a job channel into a sorted list of paths.
func collectPaths(jobs <-chan *Job) []string {
	var out []string
	for job := range jobs {
		out = append(out, job.Path)
	}
	slices.Sort(out)
	return out
}

func TestNewWalkerRejectsBadOptions(t *testing.T) {
	tests := []struct {
		name string
		opts WalkOptions
	}{
		{"no roots", WalkOptions{}},
		{"no usable extensions", WalkOptions{Roots: []string{"."}, Extensions: []string{"", "  "}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewWalker(tt.opts, nil); err == nil {
				t.Fatal("NewWalker() succeeded, want error")
			}
		})
	}
}

func TestWalkerMatches(t *testing.T) {
	tests := []struct {
		name       string
		extensions []string
		filename   string
		want       bool
	}{
		{"default extension", nil, "track.flac", true},
		{"upper case", nil, "TRACK.FLAC", true},
		{"mixed case", nil, "Track.Flac", true},
		{"another format", nil, "track.mp3", false},
		{"no extension", nil, "track", false},
		{"sidecar report", nil, "track.report.json", false},
		{"extension without a dot", []string{"wav"}, "track.wav", true},
		{"several extensions", []string{".flac", ".wav"}, "track.wav", true},
		{"spaced configuration", []string{" .flac "}, "track.flac", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := NewWalker(WalkOptions{Roots: []string{"."}, Extensions: tt.extensions}, nil)
			if err != nil {
				t.Fatalf("NewWalker() error = %v", err)
			}
			if got := w.Matches(tt.filename); got != tt.want {
				t.Errorf("Matches(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestWalkerDefaultsAreApplied(t *testing.T) {
	w, err := NewWalker(WalkOptions{Roots: []string{"."}, Concurrency: -1, Buffer: -5}, nil)
	if err != nil {
		t.Fatalf("NewWalker() error = %v", err)
	}
	if w.opts.Concurrency != DefaultWalkConcurrency {
		t.Errorf("Concurrency = %d, want %d", w.opts.Concurrency, DefaultWalkConcurrency)
	}
	if w.opts.Buffer != 0 {
		t.Errorf("Buffer = %d, want 0", w.opts.Buffer)
	}
}

func TestWalkerFindsMatchingFiles(t *testing.T) {
	counters := progress.NewCounters(nil, time.Now())
	w, err := NewWalker(WalkOptions{Roots: []string{"testdata"}, Buffer: 8}, counters)
	if err != nil {
		t.Fatalf("NewWalker() error = %v", err)
	}

	got := collectPaths(w.Start(t.Context()))
	want := []string{
		filepath.Join("testdata", "album", "01 Track.flac"),
		filepath.Join("testdata", "album", "02 Track.flac"),
	}
	if !slices.Equal(got, want) {
		t.Errorf("walked %v, want %v", got, want)
	}
	if err := w.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
	if n := counters.Snapshot(time.Now()).Discovered; n != 2 {
		t.Errorf("Discovered = %d, want 2", n)
	}
}

func TestWalkerAttachesFileInfo(t *testing.T) {
	w, err := NewWalker(WalkOptions{Roots: []string{"testdata"}, Buffer: 8}, nil)
	if err != nil {
		t.Fatalf("NewWalker() error = %v", err)
	}

	for job := range w.Start(t.Context()) {
		if job.Info == nil {
			t.Fatalf("job %q has no file info", job.Path)
		}
		if job.Info.Size() == 0 {
			t.Errorf("job %q reports a size of zero", job.Path)
		}
		if job.Started.IsZero() {
			t.Errorf("job %q has no start time", job.Path)
		}
	}
}

func TestWalkerDescendsIntoNestedDirectories(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(root, "top.flac"),
		filepath.Join(deep, "deep.flac"),
		filepath.Join(deep, "ignored.txt"),
	} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	w, err := NewWalker(WalkOptions{Roots: []string{root}, Buffer: 8}, nil)
	if err != nil {
		t.Fatalf("NewWalker() error = %v", err)
	}

	got := collectPaths(w.Start(t.Context()))
	want := []string{filepath.Join(deep, "deep.flac"), filepath.Join(root, "top.flac")}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("walked %v, want %v", got, want)
	}
}

func TestWalkerAcceptsIndividualFiles(t *testing.T) {
	track := filepath.Join("testdata", "album", "01 Track.flac")

	w, err := NewWalker(WalkOptions{Roots: []string{track}, Buffer: 4}, nil)
	if err != nil {
		t.Fatalf("NewWalker() error = %v", err)
	}
	if got := collectPaths(w.Start(t.Context())); !slices.Equal(got, []string{track}) {
		t.Errorf("walked %v, want just the named file", got)
	}
}

func TestWalkerIgnoresANamedFileOfTheWrongType(t *testing.T) {
	cover := filepath.Join("testdata", "album", "cover.jpg")

	w, err := NewWalker(WalkOptions{Roots: []string{cover}, Buffer: 4}, nil)
	if err != nil {
		t.Fatalf("NewWalker() error = %v", err)
	}
	if got := collectPaths(w.Start(t.Context())); len(got) != 0 {
		t.Errorf("walked %v, want nothing", got)
	}
}

// TestWalkerRecordsButSurvivesBadRoots is the behaviour a large library needs:
// one unreadable path must not cost the rest of the run.
func TestWalkerRecordsButSurvivesBadRoots(t *testing.T) {
	good := filepath.Join("testdata", "album")
	missing := filepath.Join("testdata", "does-not-exist")

	w, err := NewWalker(WalkOptions{Roots: []string{missing, good}, Buffer: 8}, nil)
	if err != nil {
		t.Fatalf("NewWalker() error = %v", err)
	}

	got := collectPaths(w.Start(t.Context()))
	if len(got) != 2 {
		t.Errorf("walked %v, want the two tracks under the good root", got)
	}
	if w.Err() == nil {
		t.Error("Err() = nil, want the missing root to be recorded")
	}
}

func TestWalkerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	w, err := NewWalker(WalkOptions{Roots: []string{"testdata"}}, nil)
	if err != nil {
		t.Fatalf("NewWalker() error = %v", err)
	}

	// The channel must close rather than hang; whether anything got through
	// before cancellation took effect is a race not worth asserting on.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range w.Start(ctx) {
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the walker did not stop after cancellation")
	}
}

func TestWalkerEmptyDirectory(t *testing.T) {
	w, err := NewWalker(WalkOptions{Roots: []string{t.TempDir()}}, nil)
	if err != nil {
		t.Fatalf("NewWalker() error = %v", err)
	}
	if got := collectPaths(w.Start(t.Context())); len(got) != 0 {
		t.Errorf("walked %v, want nothing", got)
	}
	if err := w.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}
