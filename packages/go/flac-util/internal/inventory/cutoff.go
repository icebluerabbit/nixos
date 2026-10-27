package inventory

import (
	"math"

	"homelab/flac-util/internal/report"
	"homelab/flac-util/internal/spectrogram"
)

// DefaultCutoffFloorDB is how far below the loudest frequency bin a bin still
// counts as carrying content.
//
// It is the dynamic range the spectrogram is drawn over, so a cutoff read from
// the table is the same edge the eye finds in the image beside it. Anything
// tighter reads the faint top end of a lossless track as an ending, which is
// exactly the mistake the picture exists to prevent.
//
// The value is a reading aid, not a threshold anything is judged against, and
// it is adjustable because where a spectrum "ends" depends entirely on how
// faint a signal you are willing to call signal.
var DefaultCutoffFloorDB = spectrogram.DefaultOptions().RangeDB

// Cutoff returns the highest frequency, in hertz, still carrying content, and
// whether it could be worked out at all.
//
// It is derived at read time rather than recorded by collect, which keeps the
// report to what was measured and lets the floor be moved without measuring
// anything again.
func Cutoff(s *report.Spectrum, floorDB float64) (float64, bool) {
	if s == nil || len(s.AverageDBFS) == 0 || s.BinHz <= 0 {
		return 0, false
	}
	if floorDB <= 0 {
		floorDB = DefaultCutoffFloorDB
	}

	peak := math.Inf(-1)
	for _, v := range s.AverageDBFS {
		if f := float64(v); !math.IsInf(f, 0) && !math.IsNaN(f) && f > peak {
			peak = f
		}
	}
	if math.IsInf(peak, 0) {
		// Every bin is silent, so there is no highest frequency to name.
		return 0, false
	}

	threshold := peak - floorDB
	for i := len(s.AverageDBFS) - 1; i >= 0; i-- {
		if float64(s.AverageDBFS[i]) > threshold {
			return float64(i) * float64(s.BinHz), true
		}
	}
	return 0, false
}

// Nyquist is the highest frequency a stream of the given sample rate can
// represent: half of it. Nothing above this can exist in the file, so it is
// the ceiling any cutoff is measured against.
func Nyquist(sampleRate int) float64 { return float64(sampleRate) / 2 }

// Reach returns where a track's content stops, in hertz, and how far up its
// own ceiling that is.
//
// The ratio is what makes a cutoff comparable across sample rates. 22 kHz is
// the ceiling for CD audio and less than half the ceiling at 96 kHz, so the
// same figure means "nothing was taken off the top" in one file and "the top
// was taken off" in the other. A ratio of 1 is the highest bin there is, and
// no encoder's low-pass can sit there.
func Reach(r *report.Report, floorDB float64) (hz, ratio float64, ok bool) {
	if r == nil {
		return 0, 0, false
	}

	nyquist := Nyquist(r.Stream.SampleRate)
	if nyquist <= 0 {
		return 0, 0, false
	}

	hz, ok = Cutoff(r.Spectrum, floorDB)
	if !ok {
		return 0, 0, false
	}
	return hz, hz / nyquist, true
}
