package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"homelab/flac-util/internal/measure"
	"homelab/flac-util/internal/progress"
	"homelab/flac-util/internal/report"
)

// faultFixture copies a test file into a temporary directory and returns its
// path there.
func faultFixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "audio", "testdata", name))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}

	path := filepath.Join(t.TempDir(), "01 - Track.flac")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}
	return path
}

// runOne puts one file through a whole collect run and returns what happened.
func runOne(t *testing.T, path string) (*progress.Recorder, *progress.Counters) {
	t.Helper()

	opts := CollectOptions{
		Tool:            report.Tool{Name: "flac-util", Version: "test"},
		Measure:         measure.Options{SkipSpectrum: true},
		SkipSpectrogram: true,
		Now:             func() time.Time { return time.Unix(0, 0).UTC() },
	}
	specs, err := NewCollectPipeline(opts)
	if err != nil {
		t.Fatalf("NewCollectPipeline() error = %v", err)
	}

	counters := progress.NewCounters(CollectStageNames(), time.Unix(0, 0).UTC())
	recorder := &progress.Recorder{}
	runner, err := NewRunner(specs, counters, recorder)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	jobs := make(chan *Job, 1)
	jobs <- &Job{Path: path, Info: info, Started: time.Unix(0, 0).UTC()}
	close(jobs)

	if err := runner.Run(context.Background(), jobs); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	return recorder, counters
}

// readReport loads the report written beside a track.
func readReport(t *testing.T, path string) report.Report {
	t.Helper()

	data, err := os.ReadFile(report.SidecarPath(path, report.DefaultReportSuffix))
	if err != nil {
		t.Fatalf("the report was not written: %v", err)
	}

	var r report.Report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("the report is not valid JSON: %v", err)
	}
	return r
}

// TestCollectReportsAFileWhoseAudioStopsShort is the whole point of the fault
// path: a file that yields frames and then breaks is the one most worth
// measuring, and throwing the measurements away because the last frame was bad
// discards the evidence along with the error.
func TestCollectReportsAFileWhoseAudioStopsShort(t *testing.T) {
	path := faultFixture(t, "truncated16.flac")
	recorder, counters := runOne(t, path)

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("%d events, want 1", len(events))
	}
	if got := events[0].Outcome; got != progress.OutcomeFaulted {
		t.Errorf("outcome = %v, want %v", got, progress.OutcomeFaulted)
	}
	if events[0].Err == nil {
		t.Error("the event carries no error, so nothing says what went wrong")
	}
	if got := events[0].Stage; got != StageDecode {
		t.Errorf("stage = %q, want %q", got, StageDecode)
	}

	s := counters.Snapshot(time.Unix(1, 0).UTC())
	if s.Faulted != 1 {
		t.Errorf("Faulted = %d, want 1", s.Faulted)
	}
	if s.Completed != 0 || s.Failed != 0 {
		t.Errorf("Completed = %d and Failed = %d, want the file counted only as faulted",
			s.Completed, s.Failed)
	}

	r := readReport(t, path)
	if r.Decode.Complete {
		t.Error("decode.complete = true for a file that stopped short")
	}
	if !strings.Contains(r.Decode.Error, "unexpected EOF") {
		t.Errorf("decode.error = %q, want it to say what broke", r.Decode.Error)
	}
	if r.Decode.Stage != StageDecode {
		t.Errorf("decode.stage = %q, want %q", r.Decode.Stage, StageDecode)
	}

	// The measurements have to be there, and the checksum has to disagree:
	// between them that is what says the file is short rather than merely odd.
	if r.Integrity.Match {
		t.Error("integrity.match = true over a partial stream")
	}
	if r.Integrity.FramesDecoded == 0 {
		t.Error("no frames recorded, so the report says nothing about what was read")
	}
	if len(r.Levels) == 0 {
		t.Error("no levels recorded")
	}
}

func TestCollectReportsACleanFileAsComplete(t *testing.T) {
	path := faultFixture(t, "sine16.flac")
	recorder, counters := runOne(t, path)

	if got := recorder.Events()[0].Outcome; got != progress.OutcomeCompleted {
		t.Errorf("outcome = %v, want %v", got, progress.OutcomeCompleted)
	}
	if s := counters.Snapshot(time.Unix(1, 0).UTC()); s.Faulted != 0 || s.Completed != 1 {
		t.Errorf("Faulted = %d and Completed = %d, want 0 and 1", s.Faulted, s.Completed)
	}

	r := readReport(t, path)
	if !r.Decode.Complete {
		t.Errorf("decode.complete = false for a clean file: %q", r.Decode.Error)
	}
	if r.Decode.Error != "" || r.Decode.Stage != "" {
		t.Errorf("decode = %+v, want it empty for a clean file", r.Decode)
	}
	if !r.Integrity.Match {
		t.Error("integrity.match = false for a clean file")
	}
}

// TestCollectFailsWhenNothingCanBeRead covers the other side: a file that
// yields no frames has nothing to describe, so it stays a failure and no
// report is written.
func TestCollectFailsWhenNothingCanBeRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "01 - Track.flac")
	if err := os.WriteFile(path, []byte("this is not a FLAC file at all"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	recorder, counters := runOne(t, path)
	if got := recorder.Events()[0].Outcome; got != progress.OutcomeFailed {
		t.Errorf("outcome = %v, want %v", got, progress.OutcomeFailed)
	}
	if s := counters.Snapshot(time.Unix(1, 0).UTC()); s.Failed != 1 || s.Faulted != 0 {
		t.Errorf("Failed = %d and Faulted = %d, want 1 and 0", s.Failed, s.Faulted)
	}
	if _, err := os.Stat(report.SidecarPath(path, report.DefaultReportSuffix)); err == nil {
		t.Error("a report was written for a file nothing could be read from")
	}
}
