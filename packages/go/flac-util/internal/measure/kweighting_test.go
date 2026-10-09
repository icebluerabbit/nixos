package measure

import (
	"math"
	"math/cmplx"
	"testing"
)

// TestKWeightingMatchesTheTabulatedCoefficients is the check that the
// derivation is right.
//
// ITU-R BS.1770-5 prints its coefficients for 48 kHz. Everything else here is
// derived from the analog prototype, so if the derivation reproduces the
// printed numbers at 48 kHz it is the same filter at 44.1 and 96 kHz too. If
// it does not, every loudness figure in the library is quietly wrong.
func TestKWeightingMatchesTheTabulatedCoefficients(t *testing.T) {
	const tolerance = 1e-11

	shelf := kShelf(48000)
	for _, tt := range []struct {
		name string
		got  float64
		want float64
	}{
		{"shelf b0", shelf.b0, 1.53512485958697},
		{"shelf b1", shelf.b1, -2.69169618940638},
		{"shelf b2", shelf.b2, 1.19839281085285},
		{"shelf a1", shelf.a1, -1.69065929318241},
		{"shelf a2", shelf.a2, 0.73248077421585},
	} {
		if math.Abs(tt.got-tt.want) > tolerance {
			t.Errorf("%s = %.14f, want %.14f", tt.name, tt.got, tt.want)
		}
	}

	highPass := kHighPass(48000)
	for _, tt := range []struct {
		name string
		got  float64
		want float64
	}{
		{"high pass b0", highPass.b0, 1.0},
		{"high pass b1", highPass.b1, -2.0},
		{"high pass b2", highPass.b2, 1.0},
		{"high pass a1", highPass.a1, -1.99004745483398},
		{"high pass a2", highPass.a2, 0.99007225036621},
	} {
		if math.Abs(tt.got-tt.want) > tolerance {
			t.Errorf("%s = %.14f, want %.14f", tt.name, tt.got, tt.want)
		}
	}
}

// response returns the filter's gain at a frequency, in decibels.
//
// It is evaluated from the coefficients rather than measured by running a sine
// through the filter. Peak-picking a sine lands on a different point of the
// waveform at each sample rate, an error of about a tenth of a decibel — the
// same size as the differences this is here to detect.
func response(sampleRate int, hz float64) float64 {
	filter := newKWeighting(sampleRate)
	w := 2 * math.Pi * hz / float64(sampleRate)

	return 20 * math.Log10(cmplx.Abs(gain(filter.shelf, w)*gain(filter.highPass, w)))
}

// gain evaluates one section on the unit circle.
func gain(f biquad, w float64) complex128 {
	z1 := cmplx.Exp(complex(0, -w))
	z2 := z1 * z1

	num := complex(f.b0, 0) + complex(f.b1, 0)*z1 + complex(f.b2, 0)*z2
	den := 1 + complex(f.a1, 0)*z1 + complex(f.a2, 0)*z2
	return num / den
}

// TestKWeightingHasTheSameShapeAtEveryRate is what the tabulated coefficients
// cannot check: that the derivation puts the curve in the same place at the
// rates this library actually holds.
//
// The response is compared against the 48 kHz filter, not against absolute
// figures, because the standard defines the other rates as "whatever gives the
// same response".
//
// The rates do not agree exactly and cannot. The bilinear transform matches
// the analog prototype at its corner frequency and warps the scale either side
// of it, by an amount that depends on the sample rate — so the digital filters
// converge on the same curve without landing on it. The error tracks distance
// from 48 kHz: under 0.01 dB at 44.1, about 0.02 at 96, about 0.04 at 192.
// A tolerance of 0.05 dB is therefore what the method delivers, and anything
// larger than that is a mistake rather than the transform.
func TestKWeightingHasTheSameShapeAtEveryRate(t *testing.T) {
	// Below the shelf, across it, and above it. Nothing near Nyquist, where
	// the rates genuinely differ.
	frequencies := []float64{40, 100, 500, 1000, 2000, 4000, 10000}

	for _, rate := range []int{44100, 96000, 192000} {
		for _, hz := range frequencies {
			want := response(48000, hz)
			got := response(rate, hz)

			if math.Abs(got-want) > 0.05 {
				t.Errorf("%d Hz at %d: %.2f dB, want %.2f dB (the 48 kHz filter)",
					int(hz), rate, got, want)
			}
		}
	}
}

// TestKWeightingShapeIsDirectionallyRight is a coarse check that this is the
// K-weighting curve and not, say, its inverse: bass cut, midrange close to
// unmodified, treble lifted onto the shelf.
//
// It states no precise figures. The tabulated coefficients already pin those,
// and inventing expected decibels here would record what this code happens to
// do rather than what the standard says.
func TestKWeightingShapeIsDirectionallyRight(t *testing.T) {
	deepBass := response(48000, 20)
	midrange := response(48000, 1000)
	treble := response(48000, 10000)

	if deepBass > -6 {
		t.Errorf("20 Hz = %.2f dB, want it well attenuated", deepBass)
	}
	if math.Abs(midrange) > 1.5 {
		t.Errorf("1 kHz = %.2f dB, want it close to unmodified", midrange)
	}
	if treble < 3 || treble > 5 {
		t.Errorf("10 kHz = %.2f dB, want it on the roughly 4 dB shelf", treble)
	}
	if treble <= midrange || midrange <= deepBass {
		t.Errorf("the curve does not rise with frequency: %.2f, %.2f, %.2f",
			deepBass, midrange, treble)
	}
}

func TestBiquadIsQuietWithoutInput(t *testing.T) {
	filter := newKWeighting(48000)
	for range 100 {
		if got := filter.step(0); got != 0 {
			t.Fatalf("silence produced %v", got)
		}
	}
}

// TestKWeightingResetClearsTheTail checks a reused filter does not carry the
// end of one signal into the start of the next.
func TestKWeightingResetClearsTheTail(t *testing.T) {
	filter := newKWeighting(48000)
	for range 1000 {
		filter.step(1)
	}

	filter.reset()
	if got := filter.step(0); got != 0 {
		t.Errorf("a reset filter produced %v from silence", got)
	}
}
