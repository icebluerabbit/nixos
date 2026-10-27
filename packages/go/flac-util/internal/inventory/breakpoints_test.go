package inventory

import (
	"fmt"
	"slices"
	"testing"
)

func TestCutoffBreakpointsAreRedsTable(t *testing.T) {
	// Verbatim from https://interviewfor.red/en/spectrals.html. The point of
	// banding is to name the row of that table a value landed on, so a wrong
	// entry here mislabels a track.
	want := []Breakpoint{
		{Hz: 16000, Format: "MP3 128 CBR"},
		{Hz: 18500, Format: "MP3 V2"},
		{Hz: 19000, Format: "MP3 192 CBR"},
		{Hz: 19500, Format: "MP3 V0"},
		{Hz: 20000, Format: "MP3 256 CBR"},
		{Hz: 20500, Format: "MP3 320 CBR"},
		{Hz: 22000, Format: "CD / lossless"},
	}

	got := CutoffBreakpoints()
	if len(got) != len(want) {
		t.Fatalf("%d breakpoints, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("breakpoint %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if !slices.IsSortedFunc(got, func(a, b Breakpoint) int {
		switch {
		case a.Hz < b.Hz:
			return -1
		case a.Hz > b.Hz:
			return 1
		}
		return 0
	}) {
		t.Errorf("breakpoints are not in ascending order: %+v", got)
	}
}

func TestBandCountMatchesTheBreakpoints(t *testing.T) {
	if got, want := BandCount(), len(CutoffBreakpoints()); got != want {
		t.Errorf("BandCount() = %d, want %d", got, want)
	}
}

func TestBand(t *testing.T) {
	breakpoints := []Breakpoint{{Hz: 16000}, {Hz: 18500}, {Hz: 19000}, {Hz: 22000}}

	tests := []struct {
		name  string
		value float64
		want  int
	}{
		{"below every breakpoint", 15000, NoBand},
		{"exactly on the lowest", 16000, 0},
		{"between the first two", 17000, 0},
		{"exactly on the second", 18500, 1},
		{"just under the third", 18999, 1},
		{"exactly on the highest", 22000, 3},
		{"above the highest", 22050, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := band(tt.value, breakpoints); got != tt.want {
				t.Errorf("band(%g) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestBandWithoutBreakpoints(t *testing.T) {
	if got := band(20000, nil); got != NoBand {
		t.Errorf("band() with no breakpoints = %d, want %d", got, NoBand)
	}
}

// TestCutoffCellLeavesAFullSpectrumUnbanded is the mislabel this banding was
// rebuilt for. RED's table describes where a lossy encoder stops, so it has
// nothing to say about a track that stops nowhere - and its top row, read as
// a band, filed a 96 kHz track under what a CD holds.
func TestCutoffCellLeavesAFullSpectrumUnbanded(t *testing.T) {
	for _, rate := range []int{44100, 96000} {
		r := trackReport()
		r.Stream.SampleRate = rate
		r.Spectrum = fullSpectrum(rate, -10, -20, -40, -90, -100)

		rows := NewTable(Group{Entries: []Entry{currentEntry("/a/t.flac", r)}}, Options{}).Rows
		cell := rows[0][column(t, "cutoff")]

		if cell.Band != NoBand {
			t.Errorf("%d Hz: Band = %d, want %d for content reaching the ceiling",
				rate, cell.Band, NoBand)
		}
		if cell.Value != 1 {
			t.Errorf("%d Hz: ratio = %v, want 1", rate, cell.Value)
		}
		if want := fmt.Sprintf("%.1fk", Nyquist(rate)/1000); cell.Text != want {
			t.Errorf("%d Hz: text = %q, want %q", rate, cell.Text, want)
		}
	}
}

func TestCutoffCellBandsAShelfWhereItFalls(t *testing.T) {
	// Content stops dead at 19.5 kHz and everything above is empty, which is
	// what an encoder's low-pass leaves behind.
	r := trackReport()
	r.Spectrum = spectrum(19500.0/2, -10, -20, -30)

	rows := NewTable(Group{Entries: []Entry{currentEntry("/a/t.flac", r)}}, Options{}).Rows
	cell := rows[0][column(t, "cutoff")]

	if want := 3; cell.Band != want {
		t.Errorf("Band = %d, want %d (%s)", cell.Band, want, CutoffBreakpoints()[want].Format)
	}
}

func TestUnbandedColumnsCarryNoBand(t *testing.T) {
	rows := NewTable(Group{Entries: []Entry{currentEntry("/a/t.flac", trackReport())}}, Options{}).Rows

	for _, title := range []string{"track", "status"} {
		if got := rows[0][column(t, title)].Band; got != NoBand {
			t.Errorf("%s has band %d, want %d: only cutoff has breakpoints", title, got, NoBand)
		}
	}
}
