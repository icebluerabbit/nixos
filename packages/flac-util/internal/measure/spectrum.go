package measure

import (
	"fmt"
	"math"
)

// Default spectrum settings. FFTSize is quoted at DefaultReferenceRate and is
// scaled with the stream's own rate so that a frequency bin covers the same
// span in Hz whatever the file's sample rate.
const (
	DefaultFFTSize       = 2048
	DefaultReferenceRate = 44100
	DefaultOverlap       = 0.5
	DefaultColumns       = 1920
)

// SpectrumOptions configures the Spectrum accumulator.
type SpectrumOptions struct {
	// FFTSize is the transform length at ReferenceRate. It must be a power of
	// two.
	FFTSize int
	// ReferenceRate is the sample rate FFTSize is quoted at.
	ReferenceRate int
	// Window names the window function, as accepted by NewWindow.
	Window string
	// Overlap is the fraction of a frame shared with the next, in [0,1).
	Overlap float64
	// Columns is how many time slices the grid retains. Frames are averaged
	// into columns, and the number of frames per column doubles whenever the
	// stream outruns the allocation, so memory stays bounded regardless of
	// track length.
	Columns int
}

// DefaultSpectrumOptions returns the settings used unless overridden.
func DefaultSpectrumOptions() SpectrumOptions {
	return SpectrumOptions{
		FFTSize:       DefaultFFTSize,
		ReferenceRate: DefaultReferenceRate,
		Window:        DefaultWindow,
		Overlap:       DefaultOverlap,
		Columns:       DefaultColumns,
	}
}

// Validate reports whether the options are usable.
func (o SpectrumOptions) Validate() error {
	switch {
	case o.FFTSize < 2 || o.FFTSize&(o.FFTSize-1) != 0:
		return fmt.Errorf("measure: fft size must be a power of two >= 2, got %d", o.FFTSize)
	case o.ReferenceRate <= 0:
		return fmt.Errorf("measure: reference rate must be positive, got %d", o.ReferenceRate)
	case o.Overlap < 0 || o.Overlap >= 1:
		return fmt.Errorf("measure: overlap must be within [0,1), got %v", o.Overlap)
	case o.Columns < 1:
		return fmt.Errorf("measure: columns must be positive, got %d", o.Columns)
	}
	if _, err := NewWindow(o.Window, 2); err != nil {
		return err
	}
	return nil
}

// SizeFor returns the transform length to use for a stream at the given sample
// rate, being FFTSize scaled by the ratio of rates and rounded to a power of
// two.
func (o SpectrumOptions) SizeFor(sampleRate int) int {
	const minSize, maxSize = 64, 1 << 16

	scaled := float64(o.FFTSize) * float64(sampleRate) / float64(o.ReferenceRate)
	size := 1
	for size < maxSize && float64(size)*math.Sqrt2 < scaled {
		size <<= 1
	}
	return min(max(size, minSize), maxSize)
}

// Grid is a time-by-frequency table of mean power, the raw material a
// spectrogram image is drawn from.
//
// Values are linear power relative to digital full scale, not decibels: the
// conversion belongs to whatever renders or reports them.
type Grid struct {
	// Power is indexed by column then frequency bin.
	Power [][]float32
	// Bins is the length of each column, being FFTSize/2+1.
	Bins int
	// SampleRate of the analysed stream, in Hz.
	SampleRate int
	// FFTSize actually used, after scaling for the sample rate.
	FFTSize int
	// Window names the window function used.
	Window string
	// Hop is the advance between consecutive frames, in samples.
	Hop int
	// Frames is the total number of frames analysed.
	Frames int64
	// FramesPerColumn is how many frames each column averages.
	FramesPerColumn int64
}

// Columns returns the number of time slices in the grid.
func (g Grid) Columns() int { return len(g.Power) }

// BinHz returns the width of one frequency bin, in Hz.
func (g Grid) BinHz() float64 {
	if g.FFTSize == 0 {
		return 0
	}
	return float64(g.SampleRate) / float64(g.FFTSize)
}

// BinFrequency returns the centre frequency of a bin, in Hz.
func (g Grid) BinFrequency(bin int) float64 { return float64(bin) * g.BinHz() }

// PowerDB converts linear power to decibels relative to full scale, mapping
// zero to negative infinity rather than producing a NaN.
func PowerDB(power float64) float64 {
	if power <= 0 {
		return math.Inf(-1)
	}
	return 10 * math.Log10(power)
}

// Spectrum accumulates a Grid from decoded audio.
//
// Channels are mixed to mono before analysis, matching how spectrograms are
// conventionally presented; per-channel differences are the business of the
// Stereo accumulator.
type Spectrum struct {
	format Format
	opts   SpectrumOptions

	plan    *fftPlan
	window  []float64
	gain    float64
	fftSize int
	hop     int
	bins    int

	pending []float64
	re      []float64
	im      []float64
	power   []float64

	sums   [][]float32
	counts []int32

	columns      int
	curColumn    int
	framesPerCol int64
	framesInCol  int64
	frames       int64
	finished     bool
}

// NewSpectrum returns a Spectrum accumulator for the given format.
func NewSpectrum(f Format, opts SpectrumOptions) (*Spectrum, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	size := opts.SizeFor(f.SampleRate)
	plan, err := newFFTPlan(size)
	if err != nil {
		return nil, err
	}
	window, err := NewWindow(opts.Window, size)
	if err != nil {
		return nil, err
	}
	gain := CoherentGain(window)
	if gain <= 0 {
		return nil, fmt.Errorf("measure: window %q has non-positive coherent gain", opts.Window)
	}

	hop := max(int(float64(size)*(1-opts.Overlap)), 1)
	bins := size/2 + 1

	a := &Spectrum{
		format:       f,
		opts:         opts,
		plan:         plan,
		window:       window,
		gain:         gain,
		fftSize:      size,
		hop:          hop,
		bins:         bins,
		pending:      make([]float64, 0, size+4096),
		re:           make([]float64, size),
		im:           make([]float64, size),
		power:        make([]float64, bins),
		sums:         make([][]float32, opts.Columns),
		counts:       make([]int32, opts.Columns),
		columns:      opts.Columns,
		framesPerCol: 1,
	}
	for i := range a.sums {
		a.sums[i] = make([]float32, bins)
	}
	return a, nil
}

// Name implements Accumulator.
func (a *Spectrum) Name() string { return "spectrum" }

// FFTSize returns the transform length in use for this stream.
func (a *Spectrum) FFTSize() int { return a.fftSize }

// Hop returns the advance between consecutive frames, in samples.
func (a *Spectrum) Hop() int { return a.hop }

// Write implements Accumulator.
func (a *Spectrum) Write(b Block) {
	n, channels := b.Len(), b.Channels()
	if n == 0 || channels == 0 || a.finished {
		return
	}

	scale := 1 / (a.format.FullScale() * float64(channels))
	for i := range n {
		var acc float64
		for c := range channels {
			acc += float64(b.Samples[c][i])
		}
		a.pending = append(a.pending, acc*scale)
	}
	a.drain()
}

// drain consumes every whole frame available in the pending buffer and shifts
// the remainder to the front.
func (a *Spectrum) drain() {
	consumed := 0
	for len(a.pending)-consumed >= a.fftSize {
		a.frame(a.pending[consumed : consumed+a.fftSize])
		consumed += a.hop
	}
	if consumed > 0 {
		a.pending = append(a.pending[:0], a.pending[consumed:]...)
	}
}

// frame analyses one windowed frame and folds it into the current column.
func (a *Spectrum) frame(samples []float64) {
	for i, s := range samples {
		a.re[i] = s * a.window[i] / a.gain
		a.im[i] = 0
	}
	a.plan.powerSpectrum(a.re, a.im, a.power)

	column := a.sums[a.curColumn]
	for k, p := range a.power {
		column[k] += float32(p)
	}
	a.counts[a.curColumn]++
	a.frames++

	a.framesInCol++
	if a.framesInCol >= a.framesPerCol {
		a.curColumn++
		a.framesInCol = 0
		if a.curColumn == a.columns {
			a.halve()
		}
	}
}

// halve merges adjacent column pairs, doubling the time each column spans. It
// runs whenever a stream produces more frames than the allocation holds, which
// is what keeps memory independent of track length.
func (a *Spectrum) halve() {
	half := a.columns / 2
	for i := range half {
		lo, hi := a.sums[2*i], a.sums[2*i+1]
		dst := a.sums[i]
		if i != 2*i {
			copy(dst, lo)
		}
		for k, v := range hi {
			dst[k] += v
		}
		a.counts[i] = a.counts[2*i] + a.counts[2*i+1]
	}
	for i := half; i < a.columns; i++ {
		clear(a.sums[i])
		a.counts[i] = 0
	}
	a.curColumn = half
	a.framesPerCol *= 2
	a.framesInCol = 0
}

// finish analyses any tail too short to fill a frame, zero-padded, so that
// streams shorter than one transform still produce a grid.
func (a *Spectrum) finish() {
	if a.finished {
		return
	}
	a.finished = true
	if a.frames > 0 || len(a.pending) == 0 {
		return
	}

	padded := make([]float64, a.fftSize)
	copy(padded, a.pending)
	a.frame(padded)
}

// Result implements the finaliser for the accumulator. Calling it ends the
// stream; later writes are ignored.
func (a *Spectrum) Result() Grid {
	a.finish()

	used := a.curColumn
	if a.framesInCol > 0 {
		used++
	}

	g := Grid{
		Power:           make([][]float32, used),
		Bins:            a.bins,
		SampleRate:      a.format.SampleRate,
		FFTSize:         a.fftSize,
		Window:          a.opts.Window,
		Hop:             a.hop,
		Frames:          a.frames,
		FramesPerColumn: a.framesPerCol,
	}
	for i := range used {
		column := make([]float32, a.bins)
		if a.counts[i] > 0 {
			inv := 1 / float32(a.counts[i])
			for k, v := range a.sums[i] {
				column[k] = v * inv
			}
		}
		g.Power[i] = column
	}
	return g
}
