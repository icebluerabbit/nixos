package inventory

import (
	"testing"

	"homelab/flac-util/internal/report"
)

// column returns the index of the column with the given title.
func column(t *testing.T, title string) int {
	t.Helper()
	for i, c := range Columns() {
		if c.Title == title {
			return i
		}
	}
	t.Fatalf("there is no %q column", title)
	return -1
}

// trackReport returns a report for an ordinary CD-sourced track.
func trackReport() *report.Report {
	return &report.Report{
		Stream:     report.Stream{SampleRate: 44100, BitsPerSample: 16, Channels: 2},
		Resolution: report.Resolution{DeclaredBits: 16, EffectiveBits: 16},
		Integrity:  report.Integrity{StoredMD5Present: true, Match: true},
		Decode:     report.Decode{Complete: true},
		Levels: []report.ChannelLevels{
			{ClippedSamples: 100},
			{ClippedSamples: 140},
		},
		Stereo:   &report.Stereo{Correlation: 0.5486},
		Spectrum: fullSpectrum(44100, -10, -20, -30, -40),
	}
}

// currentEntry returns an entry whose report describes its track.
func currentEntry(path string, r *report.Report) Entry {
	return Entry{AudioPath: path, State: report.StateCurrent, Report: r}
}

func TestColumnsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Columns() {
		if seen[c.Title] {
			t.Errorf("the %q column appears twice", c.Title)
		}
		seen[c.Title] = true

		if c.Cell == nil {
			t.Errorf("the %q column has no way to produce a value", c.Title)
		}
		if c.Width < len(c.Title) {
			t.Errorf("the %q column is %d wide, too narrow for its own title", c.Title, c.Width)
		}
	}
}

func TestCellValues(t *testing.T) {
	entry := currentEntry("/music/A/01 - Track.flac", trackReport())
	rows := NewTable(Group{Entries: []Entry{entry}}, Options{}).Rows

	tests := []struct {
		title string
		want  string
	}{
		{"track", "01 - Track"},
		{"rate", "44.1"},
		{"depth", "16bit"},
		{"effective", "16bit"},
		{"cutoff", "22.1k"},
		{"status", "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			if got := rows[0][column(t, tt.title)].Text; got != tt.want {
				t.Errorf("%s = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestCellsWithoutAReport(t *testing.T) {
	entry := Entry{AudioPath: "/music/A/01 - Track.flac", State: report.StateMissing}
	rows := NewTable(Group{Entries: []Entry{entry}}, Options{}).Rows

	for _, title := range []string{"rate", "cutoff", "depth", "effective"} {
		if got := rows[0][column(t, title)].Text; got != Absent {
			t.Errorf("%s = %q, want %q", title, got, Absent)
		}
	}
	if got := rows[0][column(t, "track")].Text; got != "01 - Track" {
		t.Errorf("track = %q, want the name to survive a missing report", got)
	}
	if got := rows[0][column(t, "status")]; got.Text != string(report.StateMissing) || !got.Attention {
		t.Errorf("status = %+v, want %q marked for attention", got, report.StateMissing)
	}
}

func TestStatusCellReportsAChecksumProblem(t *testing.T) {
	tests := []struct {
		name          string
		mutate        func(*report.Report)
		wantText      string
		wantAttention bool
	}{
		{"matching", func(*report.Report) {}, "ok", false},
		{
			name:          "mismatched",
			mutate:        func(r *report.Report) { r.Integrity.Match = false },
			wantText:      "MISMATCH",
			wantAttention: true,
		},
		{
			name:          "none stored",
			mutate:        func(r *report.Report) { r.Integrity.StoredMD5Present = false },
			wantText:      "no md5",
			wantAttention: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := trackReport()
			tt.mutate(r)
			rows := NewTable(Group{Entries: []Entry{currentEntry("/a/t.flac", r)}}, Options{}).Rows

			got := rows[0][column(t, "status")]
			if got.Text != tt.wantText || got.Attention != tt.wantAttention {
				t.Errorf("status = %+v, want %q attention %v", got, tt.wantText, tt.wantAttention)
			}
		})
	}
}

func TestUnreadableEntryIsShownAsSuch(t *testing.T) {
	entry := Entry{
		AudioPath: "/a/t.flac",
		State:     report.StateUnreadable,
		Err:       errNotAReport,
	}
	rows := NewTable(Group{Entries: []Entry{entry}}, Options{}).Rows

	if got := rows[0][column(t, "status")]; got.Text != "unreadable" || !got.Attention {
		t.Errorf("status = %+v, want unreadable marked for attention", got)
	}
}

// errNotAReport stands in for whatever went wrong reading a sidecar.
var errNotAReport = errString("not a report")

type errString string

func (e errString) Error() string { return string(e) }

// TestStateCellFlagsAPartialDecode covers the case a whole library scan is for:
// a report whose numbers cover only part of a file has to say so, or it reads
// as a description of the whole thing.
func TestStatusCellFlagsAPartialDecode(t *testing.T) {
	r := trackReport()
	r.Decode = report.Decode{Complete: false, Error: "frame 1696: invalid sync-code"}

	rows := NewTable(Group{Entries: []Entry{currentEntry("/a/t.flac", r)}}, Options{}).Rows
	got := rows[0][column(t, "status")]

	if got.Text != "partial" || !got.Attention {
		t.Errorf("status = %+v, want %q marked for attention", got, "partial")
	}
}

func TestStatusCellIsBlankForACompleteCurrentReport(t *testing.T) {
	rows := NewTable(Group{Entries: []Entry{currentEntry("/a/t.flac", trackReport())}}, Options{}).Rows
	if got := rows[0][column(t, "status")].Text; got != "ok" {
		t.Errorf("status = %q, want %q", got, "ok")
	}
}

// TestStateCellPrefersTheReportsOwnDoubt checks a stale report is reported as
// stale rather than as partial: if the report does not describe the file, what
// it says about the decode is beside the point.
func TestStatusCellPrefersTheReportsOwnDoubt(t *testing.T) {
	r := trackReport()
	r.Decode = report.Decode{Complete: false}

	entry := Entry{AudioPath: "/a/t.flac", State: report.StateStale, Report: r}
	rows := NewTable(Group{Entries: []Entry{entry}}, Options{}).Rows

	if got := rows[0][column(t, "status")].Text; got != string(report.StateStale) {
		t.Errorf("status = %q, want %q", got, report.StateStale)
	}
}
