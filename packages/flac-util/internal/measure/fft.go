package measure

import (
	"fmt"
	"math"
	"math/bits"
)

// fftPlan holds the precomputed tables for a radix-2 transform of a fixed
// size. Building the tables once and reusing the plan for every frame of a
// track keeps the per-frame cost to the butterflies alone.
//
// The transform is a textbook iterative Cooley-Tukey. A real-input variant
// would halve the work, and is the obvious optimisation if profiling ever says
// the spectrum accumulator matters; it is not worth the extra complexity until
// then.
type fftPlan struct {
	n   int
	rev []int
	cos []float64
	sin []float64
}

// newFFTPlan builds a plan for transforms of length n, which must be a power of
// two of at least 2.
func newFFTPlan(n int) (*fftPlan, error) {
	if n < 2 || n&(n-1) != 0 {
		return nil, fmt.Errorf("measure: fft size must be a power of two >= 2, got %d", n)
	}
	p := &fftPlan{
		n:   n,
		rev: make([]int, n),
		cos: make([]float64, n/2),
		sin: make([]float64, n/2),
	}

	shift := bits.UintSize - bits.TrailingZeros(uint(n))
	for i := range p.rev {
		p.rev[i] = int(bits.Reverse(uint(i)) >> shift)
	}
	for k := range p.cos {
		theta := -2 * math.Pi * float64(k) / float64(n)
		p.cos[k] = math.Cos(theta)
		p.sin[k] = math.Sin(theta)
	}
	return p, nil
}

// Size returns the transform length.
func (p *fftPlan) Size() int { return p.n }

// transform replaces re and im with their discrete Fourier transform, in
// place. Both slices must have length p.Size().
func (p *fftPlan) transform(re, im []float64) {
	if len(re) != p.n || len(im) != p.n {
		panic(fmt.Sprintf("measure: fft input length %d/%d, want %d", len(re), len(im), p.n))
	}

	for i, j := range p.rev {
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}

	for size := 2; size <= p.n; size <<= 1 {
		half := size / 2
		step := p.n / size
		for start := 0; start < p.n; start += size {
			for j, k := start, 0; j < start+half; j, k = j+1, k+step {
				c, s := p.cos[k], p.sin[k]
				hr, hi := re[j+half], im[j+half]
				tr := hr*c - hi*s
				ti := hr*s + hi*c
				re[j+half] = re[j] - tr
				im[j+half] = im[j] - ti
				re[j] += tr
				im[j] += ti
			}
		}
	}
}

// powerSpectrum transforms re/im in place and writes the mean power of each of
// the n/2+1 non-negative frequency bins into out.
//
// Power is normalised so that a full-scale sinusoid at a bin centre, analysed
// through a window of unit coherent gain, yields 1.0 - which makes 0 dB equal
// to digital full scale.
func (p *fftPlan) powerSpectrum(re, im, out []float64) {
	bins := p.n/2 + 1
	if len(out) != bins {
		panic(fmt.Sprintf("measure: power spectrum output length %d, want %d", len(out), bins))
	}
	p.transform(re, im)

	// A real signal splits its energy between the positive and negative
	// halves of the spectrum, so every bin except DC and Nyquist is scaled by
	// two to recover the amplitude of the original sinusoid.
	scale := 2 / float64(p.n)
	for k := range bins {
		s := scale
		if k == 0 || k == p.n/2 {
			s = 1 / float64(p.n)
		}
		a := re[k] * s
		b := im[k] * s
		out[k] = a*a + b*b
	}
}
