package measure

import "math"

// Stereo measures how alike the first two channels are.
//
// A stereo stream produced by duplicating a single channel, or by widening a
// mono source, leaves the two channels perfectly or almost perfectly
// correlated. Genuine stereo material does not.
type Stereo struct {
	format     Format
	applicable bool
	sumX       float64
	sumY       float64
	sumXX      float64
	sumYY      float64
	sumXY      float64
	identical  uint64
	count      uint64
}

// StereoResult is the final value of a Stereo accumulator.
type StereoResult struct {
	// Applicable reports whether the stream had at least two channels. When
	// false the remaining fields carry no information.
	Applicable bool
	// Correlation is the Pearson correlation of the first two channels, in
	// [-1,1]. It is NaN when either channel is constant, which includes
	// digital silence.
	Correlation float64
	// IdenticalSamples counts positions where both channels hold exactly the
	// same value.
	IdenticalSamples uint64
	// TotalSamples is the number of positions compared.
	TotalSamples uint64
}

// NewStereo returns a Stereo accumulator for the given format.
func NewStereo(f Format) *Stereo {
	return &Stereo{format: f, applicable: f.Channels >= 2}
}

// Name implements Accumulator.
func (a *Stereo) Name() string { return "stereo" }

// Write implements Accumulator.
func (a *Stereo) Write(b Block) {
	if !a.applicable || b.Channels() < 2 {
		return
	}
	left, right := b.Samples[0], b.Samples[1]
	n := min(len(left), len(right))

	for i := range n {
		x, y := float64(left[i]), float64(right[i])
		a.sumX += x
		a.sumY += y
		a.sumXX += x * x
		a.sumYY += y * y
		a.sumXY += x * y
		if left[i] == right[i] {
			a.identical++
		}
	}
	a.count += uint64(n)
}

// Result implements the finaliser for the accumulator.
func (a *Stereo) Result() StereoResult {
	out := StereoResult{
		Applicable:       a.applicable,
		Correlation:      math.NaN(),
		IdenticalSamples: a.identical,
		TotalSamples:     a.count,
	}
	if !a.applicable || a.count == 0 {
		return out
	}

	n := float64(a.count)
	covariance := a.sumXY/n - (a.sumX/n)*(a.sumY/n)
	varX := a.sumXX/n - (a.sumX/n)*(a.sumX/n)
	varY := a.sumYY/n - (a.sumY/n)*(a.sumY/n)
	if varX <= 0 || varY <= 0 {
		return out
	}

	// Accumulated rounding can push a perfectly correlated pair a hair outside
	// the mathematically valid range.
	out.Correlation = min(max(covariance/math.Sqrt(varX*varY), -1), 1)
	return out
}
