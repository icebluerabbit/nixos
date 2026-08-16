package measure

import "math"

// Levels measures amplitude properties of every channel: how loud the loudest
// moment is, how loud the track is on average, how many samples sit at the
// limit of the format, and whether the waveform is centred.
type Levels struct {
	format Format
	peak   []int32
	sumSq  []float64
	sum    []float64
	clip   []uint64
	count  uint64
}

// ChannelLevels holds the amplitude measurements for one channel.
type ChannelLevels struct {
	// PeakSample is the largest absolute sample value seen.
	PeakSample int32
	// PeakDBFS is PeakSample expressed relative to full scale. Digital
	// silence yields negative infinity, which marshals as null.
	PeakDBFS float64
	// RMSDBFS is the root-mean-square level relative to full scale.
	RMSDBFS float64
	// DCOffset is the mean sample value as a fraction of full scale. A
	// centred waveform sits at zero.
	DCOffset float64
	// ClippedSamples counts samples resting exactly on the largest or
	// smallest representable value. Such samples may be deliberate, but a
	// large count indicates a signal that ran out of headroom.
	ClippedSamples uint64
}

// LevelsResult is the final value of a Levels accumulator.
type LevelsResult struct {
	Channels []ChannelLevels
	// SampleCount is the number of inter-channel samples measured.
	SampleCount uint64
}

// NewLevels returns a Levels accumulator for the given format.
func NewLevels(f Format) *Levels {
	return &Levels{
		format: f,
		peak:   make([]int32, f.Channels),
		sumSq:  make([]float64, f.Channels),
		sum:    make([]float64, f.Channels),
		clip:   make([]uint64, f.Channels),
	}
}

// Name implements Accumulator.
func (a *Levels) Name() string { return "levels" }

// Write implements Accumulator.
func (a *Levels) Write(b Block) {
	n := b.Len()
	if n == 0 {
		return
	}

	maxSample := int32(math.Ldexp(1, a.format.BitsPerSample-1)) - 1
	minSample := -maxSample - 1

	for c := range min(b.Channels(), len(a.peak)) {
		samples := b.Samples[c]
		peak, sumSq, sum, clip := a.peak[c], a.sumSq[c], a.sum[c], a.clip[c]
		for _, s := range samples {
			abs := s
			if abs < 0 {
				// Negating the most negative value overflows, so clamp it to
				// the largest positive magnitude instead.
				if abs == minSample {
					abs = maxSample
				} else {
					abs = -abs
				}
			}
			if abs > peak {
				peak = abs
			}
			v := float64(s)
			sumSq += v * v
			sum += v
			if s == maxSample || s == minSample {
				clip++
			}
		}
		a.peak[c], a.sumSq[c], a.sum[c], a.clip[c] = peak, sumSq, sum, clip
	}
	a.count += uint64(n)
}

// Result implements the finaliser for the accumulator.
func (a *Levels) Result() LevelsResult {
	full := a.format.FullScale()
	out := LevelsResult{
		Channels:    make([]ChannelLevels, len(a.peak)),
		SampleCount: a.count,
	}
	for c := range a.peak {
		ch := ChannelLevels{
			PeakSample:     a.peak[c],
			PeakDBFS:       amplitudeDB(float64(a.peak[c]) / full),
			RMSDBFS:        math.Inf(-1),
			ClippedSamples: a.clip[c],
		}
		if a.count > 0 {
			mean := a.sum[c] / float64(a.count)
			ch.DCOffset = mean / full
			ch.RMSDBFS = amplitudeDB(math.Sqrt(a.sumSq[c]/float64(a.count)) / full)
		}
		out.Channels[c] = ch
	}
	return out
}

// amplitudeDB converts a linear amplitude ratio to decibels, mapping zero to
// negative infinity rather than producing a NaN.
func amplitudeDB(ratio float64) float64 {
	if ratio <= 0 {
		return math.Inf(-1)
	}
	return 20 * math.Log10(ratio)
}
