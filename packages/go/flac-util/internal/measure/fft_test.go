package measure

import (
	"math"
	"testing"
)

func TestNewFFTPlanRejectsBadSizes(t *testing.T) {
	for _, n := range []int{-1, 0, 1, 3, 6, 100, 1000} {
		if _, err := newFFTPlan(n); err == nil {
			t.Errorf("newFFTPlan(%d) succeeded, want error", n)
		}
	}
	for _, n := range []int{2, 4, 8, 1024, 2048} {
		p, err := newFFTPlan(n)
		if err != nil {
			t.Errorf("newFFTPlan(%d) error = %v", n, err)
			continue
		}
		if p.Size() != n {
			t.Errorf("Size() = %d, want %d", p.Size(), n)
		}
	}
}

func TestTransformPanicsOnWrongLength(t *testing.T) {
	p, err := newFFTPlan(8)
	if err != nil {
		t.Fatalf("newFFTPlan() error = %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Error("transform() with mismatched lengths did not panic")
		}
	}()
	p.transform(make([]float64, 4), make([]float64, 8))
}

// TestTransformAgainstNaiveDFT checks the fast transform against a direct
// evaluation of the definition, which is the only independent reference that
// does not reuse the implementation being tested.
func TestTransformAgainstNaiveDFT(t *testing.T) {
	const n = 32
	p, err := newFFTPlan(n)
	if err != nil {
		t.Fatalf("newFFTPlan() error = %v", err)
	}

	re := make([]float64, n)
	im := make([]float64, n)
	for i := range n {
		// An arbitrary but deterministic signal with both parts populated.
		re[i] = math.Sin(float64(i)) + 0.25*float64(i%5)
		im[i] = math.Cos(float64(i) * 0.7)
	}
	wantRe, wantIm := naiveDFT(re, im)

	p.transform(re, im)

	for k := range n {
		if !closeTo(re[k], wantRe[k], 1e-9) || !closeTo(im[k], wantIm[k], 1e-9) {
			t.Fatalf("bin %d = (%v,%v), want (%v,%v)", k, re[k], im[k], wantRe[k], wantIm[k])
		}
	}
}

func TestTransformOfConstantSignal(t *testing.T) {
	const n = 16
	p, err := newFFTPlan(n)
	if err != nil {
		t.Fatalf("newFFTPlan() error = %v", err)
	}

	re := make([]float64, n)
	im := make([]float64, n)
	for i := range re {
		re[i] = 1
	}
	p.transform(re, im)

	if !closeTo(re[0], float64(n), 1e-9) {
		t.Errorf("DC bin = %v, want %v", re[0], n)
	}
	for k := 1; k < n; k++ {
		if !closeTo(re[k], 0, 1e-9) || !closeTo(im[k], 0, 1e-9) {
			t.Errorf("bin %d = (%v,%v), want (0,0)", k, re[k], im[k])
		}
	}
}

// TestPowerSpectrumFullScaleSine pins the amplitude convention: a full-scale
// sinusoid sitting exactly on a bin centre must read as unity power, which is
// what makes zero decibels mean digital full scale everywhere else.
func TestPowerSpectrumFullScaleSine(t *testing.T) {
	const (
		n    = 64
		bin  = 8
		tol  = 1e-9
		want = 1.0
	)
	p, err := newFFTPlan(n)
	if err != nil {
		t.Fatalf("newFFTPlan() error = %v", err)
	}

	re := make([]float64, n)
	im := make([]float64, n)
	for i := range n {
		re[i] = math.Sin(2 * math.Pi * float64(bin) * float64(i) / float64(n))
	}
	out := make([]float64, n/2+1)
	p.powerSpectrum(re, im, out)

	if !closeTo(out[bin], want, tol) {
		t.Errorf("power at bin %d = %v, want %v", bin, out[bin], want)
	}
	for k := range out {
		if k == bin {
			continue
		}
		if out[k] > 1e-18 {
			t.Errorf("power at bin %d = %v, want approximately zero", k, out[k])
		}
	}
}

func TestPowerSpectrumDCAndNyquist(t *testing.T) {
	const n = 16
	p, err := newFFTPlan(n)
	if err != nil {
		t.Fatalf("newFFTPlan() error = %v", err)
	}

	// A constant signal at full scale is pure DC, and must not gain the factor
	// of two applied to the bins that have a negative-frequency twin.
	re := make([]float64, n)
	im := make([]float64, n)
	for i := range re {
		re[i] = 1
	}
	out := make([]float64, n/2+1)
	p.powerSpectrum(re, im, out)

	if !closeTo(out[0], 1, 1e-12) {
		t.Errorf("DC power = %v, want 1", out[0])
	}

	// Alternating full-scale samples are the Nyquist frequency, likewise
	// without a twin.
	for i := range re {
		re[i] = float64(1 - 2*(i%2))
		im[i] = 0
	}
	p.powerSpectrum(re, im, out)

	if !closeTo(out[n/2], 1, 1e-12) {
		t.Errorf("Nyquist power = %v, want 1", out[n/2])
	}
}

func TestPowerSpectrumPanicsOnWrongOutputLength(t *testing.T) {
	p, err := newFFTPlan(8)
	if err != nil {
		t.Fatalf("newFFTPlan() error = %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Error("powerSpectrum() with wrong output length did not panic")
		}
	}()
	p.powerSpectrum(make([]float64, 8), make([]float64, 8), make([]float64, 3))
}

// naiveDFT evaluates the discrete Fourier transform directly from its
// definition, for use as a reference in tests.
func naiveDFT(re, im []float64) (outRe, outIm []float64) {
	n := len(re)
	outRe = make([]float64, n)
	outIm = make([]float64, n)
	for k := range n {
		var sumRe, sumIm float64
		for t := range n {
			theta := -2 * math.Pi * float64(k) * float64(t) / float64(n)
			c, s := math.Cos(theta), math.Sin(theta)
			sumRe += re[t]*c - im[t]*s
			sumIm += re[t]*s + im[t]*c
		}
		outRe[k], outIm[k] = sumRe, sumIm
	}
	return outRe, outIm
}
