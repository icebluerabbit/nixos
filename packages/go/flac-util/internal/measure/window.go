package measure

import (
	"fmt"
	"math"
	"slices"
)

// Window names accepted by NewWindow.
const (
	WindowRectangular    = "rectangular"
	WindowHann           = "hann"
	WindowHamming        = "hamming"
	WindowBlackmanHarris = "blackman-harris"
)

// DefaultWindow is the window used unless one is chosen explicitly.
//
// Blackman-Harris is the default rather than the more common Hann because the
// interesting part of a music spectrum here is its very quiet high end, and
// Hann leaks roughly 60 dB more energy out of loud low-frequency bins than
// Blackman-Harris does. With Hann that leakage can be mistaken for content.
const DefaultWindow = WindowBlackmanHarris

// WindowNames returns the accepted window names in a stable order.
func WindowNames() []string {
	return []string{WindowRectangular, WindowHann, WindowHamming, WindowBlackmanHarris}
}

// NewWindow returns the coefficients of the named window of length n.
func NewWindow(name string, n int) ([]float64, error) {
	if n < 1 {
		return nil, fmt.Errorf("measure: window length must be positive, got %d", n)
	}
	if !slices.Contains(WindowNames(), name) {
		return nil, fmt.Errorf("measure: unknown window %q, want one of %v", name, WindowNames())
	}

	w := make([]float64, n)
	if name == WindowRectangular || n == 1 {
		for i := range w {
			w[i] = 1
		}
		return w, nil
	}

	// Symmetric definition: the denominator is n-1, so the first and last
	// coefficients are equal.
	denom := float64(n - 1)
	for i := range w {
		x := 2 * math.Pi * float64(i) / denom
		switch name {
		case WindowHann:
			w[i] = 0.5 - 0.5*math.Cos(x)
		case WindowHamming:
			w[i] = 0.54 - 0.46*math.Cos(x)
		case WindowBlackmanHarris:
			w[i] = 0.35875 -
				0.48829*math.Cos(x) +
				0.14128*math.Cos(2*x) -
				0.01168*math.Cos(3*x)
		}
	}
	return w, nil
}

// CoherentGain returns the mean of the window coefficients.
//
// Dividing a windowed frame by this value restores the amplitude a sinusoid
// would have had without the window, which is what makes bin magnitudes
// comparable to digital full scale.
func CoherentGain(w []float64) float64 {
	if len(w) == 0 {
		return 0
	}
	var sum float64
	for _, v := range w {
		sum += v
	}
	return sum / float64(len(w))
}
