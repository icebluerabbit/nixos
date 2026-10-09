package measure

import "math"

// The K-weighting filter of ITU-R BS.1770-5.
//
// The standard prints its coefficients for 48 kHz only, and says an
// implementation at another rate must use coefficients giving the same
// frequency response. They are therefore derived here from the analog
// prototype at whatever rate the stream runs at, rather than tabulated: a
// library of 44.1 and 96 kHz files would otherwise be measured by a filter
// built for neither.
//
// Getting this wrong is quiet. A filter with the wrong corner frequency still
// produces a plausible number, so the compliance signals of EBU Tech 3341 are
// what say whether it is right, and they are run at every rate the library
// holds rather than only at the one the standard tabulates.
const (
	// Stage one is a high shelf standing in for the acoustic effect of a
	// head: frequencies above it arrive louder than the same energy below.
	shelfHz   = 1681.974450955533
	shelfQ    = 0.7071752369554196
	shelfGain = 3.999843853973347
	// shelfVbExponent relates the shelf's mid-band gain to its high-band
	// gain. It is not 1.5 or any other round number; it is the value the
	// prototype was fitted with.
	shelfVbExponent = 0.4996667741545416

	// Stage two is a high pass, the RLB curve, discarding the low end the ear
	// barely weights.
	highPassHz = 38.13547087602444
	highPassQ  = 0.5003270373238773
)

// biquad is one second-order section in direct form I, with its coefficients
// already divided through by a0.
type biquad struct {
	b0, b1, b2 float64
	a1, a2     float64

	x1, x2 float64
	y1, y2 float64
}

// step feeds one sample through the section and returns the output.
func (f *biquad) step(x float64) float64 {
	y := f.b0*x + f.b1*f.x1 + f.b2*f.x2 - f.a1*f.y1 - f.a2*f.y2

	f.x2, f.x1 = f.x1, x
	f.y2, f.y1 = f.y1, y
	return y
}

// biquadPair is the two sections of the K-weighting filter, in series.
//
// They are kept as two sections rather than convolved into one fourth-order
// filter. The result is the same and the arithmetic is better conditioned.
type biquadPair struct {
	shelf    biquad
	highPass biquad
}

// step feeds one sample through both sections.
func (p *biquadPair) step(x float64) float64 {
	return p.highPass.step(p.shelf.step(x))
}

// reset clears the filter's memory, so a fresh signal does not inherit the
// tail of the last one.
func (p *biquadPair) reset() {
	p.shelf.x1, p.shelf.x2, p.shelf.y1, p.shelf.y2 = 0, 0, 0, 0
	p.highPass.x1, p.highPass.x2, p.highPass.y1, p.highPass.y2 = 0, 0, 0, 0
}

// newKWeighting returns the K-weighting filter for a sample rate.
func newKWeighting(sampleRate int) *biquadPair {
	return &biquadPair{
		shelf:    kShelf(sampleRate),
		highPass: kHighPass(sampleRate),
	}
}

// kShelf returns the shelving section for a sample rate.
func kShelf(sampleRate int) biquad {
	k := math.Tan(math.Pi * shelfHz / float64(sampleRate))
	vh := math.Pow(10, shelfGain/20)
	vb := math.Pow(vh, shelfVbExponent)

	a0 := 1 + k/shelfQ + k*k
	return biquad{
		b0: (vh + vb*k/shelfQ + k*k) / a0,
		b1: 2 * (k*k - vh) / a0,
		b2: (vh - vb*k/shelfQ + k*k) / a0,
		a1: 2 * (k*k - 1) / a0,
		a2: (1 - k/shelfQ + k*k) / a0,
	}
}

// kHighPass returns the high-pass section for a sample rate.
//
// Its numerator is 1, -2, 1 with no division by a0. That is what the standard
// tabulates at 48 kHz, so it is what the derivation has to reproduce at every
// other rate.
func kHighPass(sampleRate int) biquad {
	k := math.Tan(math.Pi * highPassHz / float64(sampleRate))

	a0 := 1 + k/highPassQ + k*k
	return biquad{
		b0: 1,
		b1: -2,
		b2: 1,
		a1: 2 * (k*k - 1) / a0,
		a2: (1 - k/highPassQ + k*k) / a0,
	}
}
