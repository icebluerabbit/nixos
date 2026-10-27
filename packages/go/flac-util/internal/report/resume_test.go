package report

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSidecarPath(t *testing.T) {
	tests := []struct {
		name   string
		audio  string
		suffix string
		want   string
	}{
		{
			name:   "report beside a track",
			audio:  "/music/Album/01 Track.flac",
			suffix: DefaultReportSuffix,
			want:   "/music/Album/01 Track.report.json",
		},
		{
			name:   "image beside a track",
			audio:  "/music/Album/01 Track.flac",
			suffix: DefaultSpecSuffix,
			want:   "/music/Album/01 Track.spec.png",
		},
		{
			name:   "dots inside the name are kept",
			audio:  "/music/Album/01. Track feat. Someone.flac",
			suffix: DefaultReportSuffix,
			want:   "/music/Album/01. Track feat. Someone.report.json",
		},
		{
			name:   "no extension",
			audio:  "/music/Album/track",
			suffix: DefaultReportSuffix,
			want:   "/music/Album/track.report.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SidecarPath(tt.audio, tt.suffix); got != tt.want {
				t.Errorf("SidecarPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

// writeTrackAndReport lays down a fake track with a report describing it, and
// returns the track's path and file info.
func writeTrackAndReport(t *testing.T, r *Report) (trackPath string, info os.FileInfo) {
	t.Helper()

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "track.flac")
	if err := os.WriteFile(audioPath, []byte("pretend audio"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	info, err := os.Stat(audioPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	if r != nil {
		if r.File.Size == 0 {
			r.File.Size = info.Size()
		}
		if r.File.Modified.IsZero() {
			r.File.Modified = info.ModTime()
		}
		data, err := Marshal(r)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if err := Write(SidecarPath(audioPath, DefaultReportSuffix), data); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	return audioPath, info
}

func TestCheckSkipsWhenTheReportIsCurrent(t *testing.T) {
	audioPath, info := writeTrackAndReport(t, &Report{Tool: Tool{Version: "1.0.0"}})

	got, err := Check(audioPath, info, CheckOptions{ToolVersion: "1.0.0"})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if got != SkipUpToDate {
		t.Errorf("Check() = %q, want %q", got, SkipUpToDate)
	}
}

func TestCheckProcessesWhenSomethingChanged(t *testing.T) {
	tests := []struct {
		name    string
		report  *Report
		version string
		mutate  func(*Report, os.FileInfo)
	}{
		{
			name:    "no report at all",
			report:  nil,
			version: "1.0.0",
		},
		{
			name:    "written by another version",
			report:  &Report{Tool: Tool{Version: "0.9.0"}},
			version: "1.0.0",
		},
		{
			name:    "the file has grown",
			report:  &Report{Tool: Tool{Version: "1.0.0"}},
			version: "1.0.0",
			mutate:  func(r *Report, _ os.FileInfo) { r.File.Size = 999999 },
		},
		{
			name:    "the file has been touched",
			report:  &Report{Tool: Tool{Version: "1.0.0"}},
			version: "1.0.0",
			mutate: func(r *Report, info os.FileInfo) {
				r.File.Modified = info.ModTime().Add(-time.Hour)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var audioPath string
			var info os.FileInfo
			if tt.mutate == nil {
				audioPath, info = writeTrackAndReport(t, tt.report)
			} else {
				// The report has to be written after mutation, so lay the
				// track down first and then overwrite the report.
				audioPath, info = writeTrackAndReport(t, tt.report)
				tt.mutate(tt.report, info)
				data, err := Marshal(tt.report)
				if err != nil {
					t.Fatalf("Marshal() error = %v", err)
				}
				if err := Write(SidecarPath(audioPath, DefaultReportSuffix), data); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
			}

			got, err := Check(audioPath, info, CheckOptions{ToolVersion: tt.version})
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if got != SkipNone {
				t.Errorf("Check() = %q, want %q", got, SkipNone)
			}
		})
	}
}

func TestCheckForceIgnoresEverything(t *testing.T) {
	audioPath, info := writeTrackAndReport(t, &Report{Tool: Tool{Version: "1.0.0"}})

	got, err := Check(audioPath, info, CheckOptions{ToolVersion: "1.0.0", Force: true})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if got != SkipNone {
		t.Errorf("Check() = %q, want %q", got, SkipNone)
	}
}

// TestCheckTreatsAnUnreadableReportAsAbsent covers a report truncated by an
// earlier crash: the remedy is to write a good one, not to fail the run.
func TestCheckTreatsAnUnreadableReportAsAbsent(t *testing.T) {
	audioPath, info := writeTrackAndReport(t, nil)
	if err := os.WriteFile(SidecarPath(audioPath, DefaultReportSuffix), []byte("{ truncated"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Check(audioPath, info, CheckOptions{ToolVersion: "1.0.0"})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if got != SkipNone {
		t.Errorf("Check() = %q, want %q", got, SkipNone)
	}
}

func TestCheckWithoutFileInfo(t *testing.T) {
	audioPath, _ := writeTrackAndReport(t, &Report{Tool: Tool{Version: "1.0.0"}})

	got, err := Check(audioPath, nil, CheckOptions{ToolVersion: "1.0.0"})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if got != SkipNone {
		t.Errorf("Check() = %q, want %q", got, SkipNone)
	}
}

func TestCheckUsesTheConfiguredSuffix(t *testing.T) {
	audioPath, info := writeTrackAndReport(t, &Report{Tool: Tool{Version: "1.0.0"}})

	// The report was written under the default suffix, so looking for another
	// one must find nothing.
	got, err := Check(audioPath, info, CheckOptions{
		ToolVersion:  "1.0.0",
		ReportSuffix: ".measurements.json",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if got != SkipNone {
		t.Errorf("Check() = %q, want %q", got, SkipNone)
	}
}
