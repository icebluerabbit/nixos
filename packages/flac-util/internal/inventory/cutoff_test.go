package inventory

import (
	"math"
	"testing"

	"flac-util/internal/report"
)

// spectrum returns a spectrum whose bins hold the given levels.
func spectrum(binHz float64, levels ...float64) *report.Spectrum {
	bins := make([]report.Number, len(levels))
	for i, v := range levels {
		bins[i] = report.Number(v)
	}
	return &report.Spectrum{BinHz: report.Number(binHz), Bins: len(bins), AverageDBFS: bins}
}

func TestCutoff(t *testing.T) {
	tests := []struct {
		name    string
		s       *report.Spectrum
		floorDB float64
		wantHz  float64
		wantOK  bool
	}{
		{
			name:    "content to the top bin",
			s:       spectrum(100, -20, -30, -40, -50),
			floorDB: 60,
			wantHz:  300,
			wantOK:  true,
		},
		{
			name: "a shelf leaves the top bins below the floor",
			// The last two bins sit 70 dB down, further than the floor, so the
			// content ends at bin 2.
			s:       spectrum(100, -10, -20, -30, -90, -95),
			floorDB: 60,
			wantHz:  200,
			wantOK:  true,
		},
		{
			// The same bins, read against a floor 20 dB tighter, end sooner:
			// bin 2 sits 45 dB down and no longer counts.
			name:    "a tighter floor finds less content",
			s:       spectrum(100, -10, -20, -55, -75),
			floorDB: 40,
			wantHz:  100,
			wantOK:  true,
		},
		{
			name:    "the same bins against the default floor",
			s:       spectrum(100, -10, -20, -55, -75),
			floorDB: 60,
			wantHz:  200,
			wantOK:  true,
		},
		{
			name:    "zero floor takes the default",
			s:       spectrum(100, -10, -20, -55, -65),
			floorDB: 0,
			wantHz:  300,
			wantOK:  true,
		},
		{
			name:    "silence has no highest frequency",
			s:       spectrum(100, math.Inf(-1), math.Inf(-1)),
			floorDB: 60,
			wantOK:  false,
		},
		{
			name:    "no spectrum",
			s:       nil,
			floorDB: 60,
			wantOK:  false,
		},
		{
			name:    "no bins",
			s:       spectrum(100),
			floorDB: 60,
			wantOK:  false,
		},
		{
			name:    "no bin width",
			s:       spectrum(0, -10, -20),
			floorDB: 60,
			wantOK:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hz, ok := Cutoff(tt.s, tt.floorDB)
			if ok != tt.wantOK {
				t.Fatalf("Cutoff() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && hz != tt.wantHz {
				t.Errorf("Cutoff() = %v Hz, want %v Hz", hz, tt.wantHz)
			}
		})
	}
}

func TestCutoffIgnoresNonFiniteBinsWhenFindingThePeak(t *testing.T) {
	// A silent bin marshals as null and reads back as negative infinity. It
	// must not be mistaken for the loudest bin, or the floor lands nowhere.
	s := spectrum(100, math.Inf(-1), -20, -30, -100)
	hz, ok := Cutoff(s, 60)
	if !ok {
		t.Fatal("Cutoff() found nothing in a spectrum with content")
	}
	if hz != 200 {
		t.Errorf("Cutoff() = %v Hz, want 200 Hz", hz)
	}
}

// fullSpectrum returns a spectrum for the given sample rate whose highest bin
// sits exactly at Nyquist, which is where a lossless track's content reaches.
func fullSpectrum(rate int, levels ...float64) *report.Spectrum {
	return spectrum(Nyquist(rate)/float64(len(levels)-1), levels...)
}

func TestNyquist(t *testing.T) {
	tests := []struct {
		rate int
		want float64
	}{
		{44100, 22050},
		{48000, 24000},
		{96000, 48000},
		{0, 0},
	}
	for _, tt := range tests {
		if got := Nyquist(tt.rate); got != tt.want {
			t.Errorf("Nyquist(%d) = %v, want %v", tt.rate, got, tt.want)
		}
	}
}

// TestReachIsOneAtTheCeiling checks the arithmetic needs no tolerance: the
// highest bin of a spectrum is exactly Nyquist, because bin width is the rate
// divided by the transform length.
func TestReachIsOneAtTheCeiling(t *testing.T) {
	for _, rate := range []int{44100, 48000, 96000} {
		r := &report.Report{
			Stream:   report.Stream{SampleRate: rate},
			Spectrum: fullSpectrum(rate, -10, -20, -30, -40),
		}

		hz, ratio, ok := Reach(r, 0)
		if !ok {
			t.Fatalf("%d Hz: Reach() found nothing", rate)
		}
		if hz != Nyquist(rate) {
			t.Errorf("%d Hz: cutoff = %v, want %v", rate, hz, Nyquist(rate))
		}
		if ratio != 1 {
			t.Errorf("%d Hz: ratio = %v, want exactly 1", rate, ratio)
		}
	}
}

// TestReachBelowTheCeiling covers the case the ratio exists for: content that
// stops well short of what the format could hold.
func TestReachBelowTheCeiling(t *testing.T) {
	// Four bins spanning 0 to 48 kHz, with the top two empty: content stops
	// at 24 kHz, half of what a 96 kHz stream can carry.
	r := &report.Report{
		Stream:   report.Stream{SampleRate: 96000},
		Spectrum: fullSpectrum(96000, -10, -20, -30, -200),
	}

	hz, ratio, ok := Reach(r, 0)
	if !ok {
		t.Fatal("Reach() found nothing")
	}
	if hz != 32000 {
		t.Errorf("cutoff = %v Hz, want 32000", hz)
	}
	if want := 32000.0 / 48000.0; ratio != want {
		t.Errorf("ratio = %v, want %v", ratio, want)
	}
}

func TestReachWithoutAReport(t *testing.T) {
	if _, _, ok := Reach(nil, 0); ok {
		t.Error("Reach(nil) reported a cutoff")
	}
	if _, _, ok := Reach(&report.Report{}, 0); ok {
		t.Error("Reach() of a report with no sample rate reported a cutoff")
	}
}
