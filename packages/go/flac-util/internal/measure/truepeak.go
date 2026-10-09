package measure

import "math"

// truePeakOversample is how many times the signal is oversampled before its
// peak is taken. ITU-R BS.1770 asks for at least four times at 48 kHz and
// below.
const truePeakOversample = 4

// truePeakTaps is the length of the interpolation filter, per phase.
const truePeakTaps = 12

// TruePeak measures the highest level the waveform reaches between samples.
//
// Levels reports the largest sample value, which is the highest point that was
// stored. It is not the highest point the waveform passes through: a signal
// can sit below full scale at every sample and still rise above it between
// two of them, and a converter reconstructing that signal will clip where the
// file does not.
//
// The signal is reconstructed at four times its rate and the peak taken from
// that, which is what ITU-R BS.1770 specifies. The figure can exceed 0 dBFS,
// and on a loud master usually does.
type TruePeak struct {
	format Format

	// phases holds one polyphase branch of the interpolation filter per
	// output sample position.
	phases [][]float64
	// history is the last few input samples of each channel, oldest first.
	history [][]float64
	peak    []float64
	// primed counts how many samples have entered the history. Until it is
	// full the history holds zeros the audio never contained, and a filter
	// fed a step out of silence overshoots — so those outputs are computed
	// and discarded rather than recorded as a peak the file does not have.
	primed int
}

// TruePeakResult is the final value of a TruePeak accumulator.
type TruePeakResult struct {
	// ChannelsDBTP is the true peak of each channel in dBTP, decibels
	// relative to full scale measured on the reconstructed waveform.
	ChannelsDBTP []float64
	// Oversample is the factor the signal was reconstructed at.
	Oversample int
}

// NewTruePeak returns a TruePeak accumulator for the given format.
func NewTruePeak(f Format) *TruePeak {
	a := &TruePeak{
		format:  f,
		phases:  sincPhases(truePeakOversample, truePeakTaps),
		history: make([][]float64, f.Channels),
		peak:    make([]float64, f.Channels),
	}
	for c := range a.history {
		a.history[c] = make([]float64, truePeakTaps)
	}
	return a
}

// sincPhases builds the polyphase interpolation filter.
//
// Each phase reconstructs the waveform at one position between two samples,
// from a windowed sinc: the function that ideal reconstruction uses. The
// window keeps the finite length from ringing.
func sincPhases(oversample, taps int) [][]float64 {
	phases := make([][]float64, oversample)

	for p := range phases {
		coefficients := make([]float64, taps)
		offset := float64(p) / float64(oversample)

		var sum float64
		for i := range taps {
			// Distance from this tap to the point being reconstructed.
			x := float64(i-taps/2+1) - offset

			// Windowed sinc. The Blackman window is the same family the
			// spectrum uses, chosen for how little it leaks.
			w := 0.42 -
				0.5*math.Cos(2*math.Pi*float64(i)/float64(taps-1)) +
				0.08*math.Cos(4*math.Pi*float64(i)/float64(taps-1))
			coefficients[i] = sinc(x) * w
			sum += coefficients[i]
		}

		// Normalised so a constant signal reconstructs as itself rather than
		// as a slightly different constant.
		if sum != 0 {
			for i := range coefficients {
				coefficients[i] /= sum
			}
		}
		phases[p] = coefficients
	}
	return phases
}

// sinc returns the normalised cardinal sine.
func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	return math.Sin(math.Pi*x) / (math.Pi * x)
}

// Name implements Accumulator.
func (a *TruePeak) Name() string { return "truepeak" }

// Write implements Accumulator.
func (a *TruePeak) Write(b Block) {
	scale := 1 / a.format.FullScale()

	channels := min(b.Channels(), len(a.history))
	primed := a.primed

	for c := range channels {
		history, peak := a.history[c], a.peak[c]
		primed = a.primed

		for _, s := range b.Samples[c] {
			copy(history, history[1:])
			history[len(history)-1] = float64(s) * scale

			if primed < len(history) {
				primed++
				continue
			}

			for _, phase := range a.phases {
				var sum float64
				for i, coefficient := range phase {
					sum += history[i] * coefficient
				}
				if abs := math.Abs(sum); abs > peak {
					peak = abs
				}
			}
		}
		a.peak[c] = peak
	}
	a.primed = primed
}

// Result finalises the measurement.
func (a *TruePeak) Result() TruePeakResult {
	out := TruePeakResult{
		ChannelsDBTP: make([]float64, len(a.peak)),
		Oversample:   truePeakOversample,
	}
	for c, p := range a.peak {
		if p <= 0 {
			out.ChannelsDBTP[c] = math.Inf(-1)
			continue
		}
		out.ChannelsDBTP[c] = 20 * math.Log10(p)
	}
	return out
}
