package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTrack lays out a track and its report the way collect leaves them, and
// returns the path of the track.
func writeTrack(t *testing.T, dir, name string, r *Report) string {
	t.Helper()

	audio := filepath.Join(dir, name+".flac")
	if err := os.WriteFile(audio, []byte("not really flac"), 0o644); err != nil {
		t.Fatalf("writing the track: %v", err)
	}

	info, err := os.Stat(audio)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if r != nil {
		r.File.Path = audio
		r.File.Size = info.Size()
		r.File.Modified = info.ModTime()
		writeReport(t, audio, r)
	}
	return audio
}

// writeReport writes a report beside the given track.
func writeReport(t *testing.T, audio string, r *Report) {
	t.Helper()

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshalling the report: %v", err)
	}
	if err := os.WriteFile(SidecarPath(audio, DefaultReportSuffix), data, 0o644); err != nil {
		t.Fatalf("writing the report: %v", err)
	}
}

func TestReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := &Report{
		Tool:   Tool{Name: "flac-util", Version: "0.1.0"},
		Stream: Stream{SampleRate: 44100, BitsPerSample: 16, Channels: 2},
	}
	audio := writeTrack(t, dir, "01 Track", want)

	got, err := Read(SidecarPath(audio, DefaultReportSuffix))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got.Tool != want.Tool {
		t.Errorf("Tool = %+v, want %+v", got.Tool, want.Tool)
	}
	if got.Stream.SampleRate != 44100 {
		t.Errorf("SampleRate = %d, want 44100", got.Stream.SampleRate)
	}
}

func TestReadMissingFile(t *testing.T) {
	if _, err := Read(filepath.Join(t.TempDir(), "absent.report.json")); err == nil {
		t.Fatal("Read() of an absent file returned no error")
	}
}

func TestReadRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.report.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("Read() of malformed JSON returned no error")
	}
}

func TestDescribes(t *testing.T) {
	when := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	r := &Report{File: File{Size: 4096, Modified: when}}

	tests := []struct {
		name     string
		report   *Report
		size     int64
		modified time.Time
		want     bool
	}{
		{"same file", r, 4096, when, true},
		{"different size", r, 4097, when, false},
		{"different time", r, 4096, when.Add(time.Second), false},
		{"no report", nil, 4096, when, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Describes(tt.report, tt.size, tt.modified); got != tt.want {
				t.Errorf("Describes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadCurrent(t *testing.T) {
	dir := t.TempDir()
	audio := writeTrack(t, dir, "01 Track", &Report{Stream: Stream{SampleRate: 44100}})

	r, state, err := Load(audio, "")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if state != StateCurrent {
		t.Errorf("state = %q, want %q", state, StateCurrent)
	}
	if r == nil || r.Stream.SampleRate != 44100 {
		t.Errorf("the report was not returned: %+v", r)
	}
}

func TestLoadMissingReport(t *testing.T) {
	dir := t.TempDir()
	audio := writeTrack(t, dir, "01 Track", nil)

	r, state, err := Load(audio, DefaultReportSuffix)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if state != StateMissing {
		t.Errorf("state = %q, want %q", state, StateMissing)
	}
	if r != nil {
		t.Errorf("a report was returned for a track that has none: %+v", r)
	}
}

func TestLoadStaleReport(t *testing.T) {
	dir := t.TempDir()
	audio := writeTrack(t, dir, "01 Track", &Report{})

	// The track is replaced by a longer one, which is what an upgraded rip or
	// a re-download looks like from here.
	if err := os.WriteFile(audio, []byte("a rather longer file than before"), 0o644); err != nil {
		t.Fatalf("replacing the track: %v", err)
	}

	_, state, err := Load(audio, "")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if state != StateStale {
		t.Errorf("state = %q, want %q", state, StateStale)
	}
}

func TestLoadOrphanedReport(t *testing.T) {
	dir := t.TempDir()
	audio := writeTrack(t, dir, "01 Track", &Report{})
	if err := os.Remove(audio); err != nil {
		t.Fatalf("removing the track: %v", err)
	}

	r, state, err := Load(audio, "")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if state != StateOrphan {
		t.Errorf("state = %q, want %q", state, StateOrphan)
	}
	if r == nil {
		t.Error("the orphaned report was not returned, so its contents cannot be shown")
	}
}

func TestLoadUnreadableReport(t *testing.T) {
	dir := t.TempDir()
	audio := writeTrack(t, dir, "01 Track", nil)
	if err := os.WriteFile(SidecarPath(audio, DefaultReportSuffix), []byte("{"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	_, state, err := Load(audio, "")
	if err != nil {
		t.Fatalf("Load() error = %v, want a state rather than a failure", err)
	}
	if state != StateUnreadable {
		t.Errorf("state = %q, want %q", state, StateUnreadable)
	}
}
