package measure

import "math"

// Loudness measures programme loudness to ITU-R BS.1770-5, in LUFS, together
// with the loudness range defined by EBU Tech 3342.
//
// The algorithm is not this package's invention and is not open to
// improvement: a number described as LUFS is only worth recording if it is
// comparable with every other tool that reports one. It runs in four steps.
//
//   - Every channel is filtered by the two-stage K-weighting filter, which
//     approximates how the ear weights frequency.
//   - The filtered signal is squared and averaged over overlapping windows.
//   - Windows quieter than an absolute threshold are discarded, then windows
//     quieter than a threshold derived from the rest are discarded too. This
//     two-pass gate is what stops silence between tracks from dragging the
//     figure down.
//   - What survives is averaged and expressed in LUFS.
//
// Loudness range uses the same gated blocks over a longer window, reported as
// the spread between the 10th and 95th percentiles.
type Loudness struct {
	format  Format
	filters []*biquadPair

	weights []float64

	// momentaryWindow and shortWindow accumulate their own overlapping
	// windows; blocks hold the mean square of every completed one. Gating
	// needs them all, because the threshold it applies is derived from them.
	momentaryWindow *slidingWindow
	shortWindow     *slidingWindow
	momentary       []float64
	shortTerm       []float64
}

// slidingWindow accumulates the mean square of a window that advances by a
// fraction of its own length.
//
// It keeps one running sum per step rather than one for the whole window, so
// that a window's value is the exact sum of the steps it covers. Scaling a
// single running sum down as the window advances would be an exponential
// decay, which is a different measurement wearing the same name: the samples
// that left the window would go on contributing to it forever, quietly, and a
// track's loudness would depend on what came before the part being measured.
type slidingWindow struct {
	channels int
	// steps is how many steps make up one window.
	steps int
	// samplesPerStep is the length of one step.
	samplesPerStep int

	// current is the running sum of the step being filled, per channel.
	current []float64
	filled  int
	// history holds the completed step sums, oldest first, at most steps of
	// them.
	history [][]float64
}

// newSlidingWindow returns a window of the given length advancing by the given
// fraction of it.
func newSlidingWindow(channels, window int, overlap float64) *slidingWindow {
	steps := max(int(math.Round(1/(1-overlap))), 1)
	return &slidingWindow{
		channels:       channels,
		steps:          steps,
		samplesPerStep: max(window/steps, 1),
		current:        make([]float64, channels),
	}
}

// length is the number of samples one whole window covers.
func (w *slidingWindow) length() int { return w.samplesPerStep * w.steps }

// add folds one sample of one channel into the step being filled.
func (w *slidingWindow) add(channel int, squared float64) {
	w.current[channel] += squared
}

// advance records that one sample of every channel has been added, and returns
// the completed step when one has just finished.
func (w *slidingWindow) advance() ([]float64, bool) {
	w.filled++
	if w.filled < w.samplesPerStep {
		return nil, false
	}

	step := w.current
	w.current = make([]float64, w.channels)
	w.filled = 0

	w.history = append(w.history, step)
	if len(w.history) > w.steps {
		w.history = w.history[len(w.history)-w.steps:]
	}
	if len(w.history) < w.steps {
		return nil, false
	}

	// The window is the sum of the steps it covers, exactly.
	total := make([]float64, w.channels)
	for _, past := range w.history {
		for c, v := range past {
			total[c] += v
		}
	}
	return total, true
}

// Loudness constants from ITU-R BS.1770-5 and EBU Tech 3342.
const (
	// momentaryWindowMS is the window integrated loudness is built from.
	momentaryWindowMS = 400
	// momentaryOverlap is the fraction of a window shared with the next.
	momentaryOverlap = 0.75
	// shortTermWindowMS is the window loudness range is built from.
	shortTermWindowMS = 3000

	// absoluteGateLUFS discards windows quieter than this outright.
	absoluteGateLUFS = -70.0
	// relativeGateLU is how far below the ungated loudness the second gate
	// sits.
	relativeGateLU = -10.0
	// rangeGateLU is the corresponding gate for loudness range.
	rangeGateLU = -20.0

	// loudnessOffset converts a mean square into LUFS.
	loudnessOffset = -0.691
)

// LoudnessResult is the final value of a Loudness accumulator.
type LoudnessResult struct {
	// IntegratedLUFS is the gated loudness of the whole programme. Silence
	// yields negative infinity, which marshals as null.
	IntegratedLUFS float64
	// RangeLU is the spread between the quiet and loud parts, in loudness
	// units, from EBU Tech 3342.
	RangeLU float64
	// Blocks is how many momentary windows were measured.
	Blocks int
	// GatedBlocks is how many survived both gates. A programme where these
	// differ widely is one with a lot of near-silence in it.
	GatedBlocks int
}

// NewLoudness returns a Loudness accumulator for the given format.
func NewLoudness(f Format) *Loudness {
	window := f.SampleRate * momentaryWindowMS / 1000
	short := f.SampleRate * shortTermWindowMS / 1000

	a := &Loudness{
		format:          f,
		filters:         make([]*biquadPair, f.Channels),
		weights:         channelWeights(f.Channels),
		momentaryWindow: newSlidingWindow(f.Channels, window, momentaryOverlap),
		shortWindow:     newSlidingWindow(f.Channels, short, momentaryOverlap),
	}
	for c := range a.filters {
		a.filters[c] = newKWeighting(f.SampleRate)
	}
	return a
}

// channelWeights returns the per-channel weighting from BS.1770.
//
// Left, right and centre count fully; the surround channels count for more,
// because sound arriving from behind is perceived as louder than the same
// sound in front. Anything beyond five channels is weighted as unity, which
// is what the standard says for channels it does not name.
func channelWeights(channels int) []float64 {
	weights := make([]float64, channels)
	for c := range weights {
		switch c {
		case 3, 4:
			weights[c] = 1.41
		default:
			weights[c] = 1.0
		}
	}
	return weights
}

// Name implements Accumulator.
func (a *Loudness) Name() string { return "loudness" }

// Write implements Accumulator.
func (a *Loudness) Write(b Block) {
	n := b.Len()
	if n == 0 {
		return
	}
	scale := 1 / a.format.FullScale()

	for i := range n {
		for c := range min(b.Channels(), len(a.filters)) {
			v := a.filters[c].step(float64(b.Samples[c][i]) * scale)
			a.momentaryWindow.add(c, v*v)
			a.shortWindow.add(c, v*v)
		}

		if total, done := a.momentaryWindow.advance(); done {
			a.momentary = append(a.momentary, a.meanSquare(total, a.momentaryWindow.length()))
		}
		if total, done := a.shortWindow.advance(); done {
			a.shortTerm = append(a.shortTerm, a.meanSquare(total, a.shortWindow.length()))
		}
	}
}

// meanSquare returns the channel-weighted mean square of a filled window.
func (a *Loudness) meanSquare(sumSq []float64, window int) float64 {
	var total float64
	for c, s := range sumSq {
		total += a.weights[c] * s / float64(window)
	}
	return total
}

// Result finalises the measurement.
func (a *Loudness) Result() LoudnessResult {
	return LoudnessResult{
		IntegratedLUFS: gatedLoudness(a.momentary),
		RangeLU:        loudnessRange(a.shortTerm),
		Blocks:         len(a.momentary),
		GatedBlocks:    len(gatedBlocks(a.momentary, relativeGateLU)),
	}
}

// lufs converts a mean square to LUFS.
func lufs(meanSquare float64) float64 {
	if meanSquare <= 0 {
		return math.Inf(-1)
	}
	return loudnessOffset + 10*math.Log10(meanSquare)
}

// gatedBlocks returns the windows surviving both gates.
//
// The absolute gate drops digital silence. The relative gate then drops
// whatever sits more than the given margin below the loudness of what is
// left, which is what stops quiet passages from pulling a programme's figure
// down towards them.
func gatedBlocks(blocks []float64, marginLU float64) []float64 {
	loud := make([]float64, 0, len(blocks))
	for _, ms := range blocks {
		if lufs(ms) > absoluteGateLUFS {
			loud = append(loud, ms)
		}
	}
	if len(loud) == 0 {
		return nil
	}

	threshold := lufs(mean(loud)) + marginLU
	gated := make([]float64, 0, len(loud))
	for _, ms := range loud {
		if lufs(ms) > threshold {
			gated = append(gated, ms)
		}
	}
	return gated
}

// gatedLoudness returns the integrated loudness of a set of windows.
func gatedLoudness(blocks []float64) float64 {
	gated := gatedBlocks(blocks, relativeGateLU)
	if len(gated) == 0 {
		return math.Inf(-1)
	}
	return lufs(mean(gated))
}

// loudnessRange returns the spread between the quiet and loud parts of a
// programme, from EBU Tech 3342.
//
// It is the distance between the 10th and 95th percentiles of the gated
// short-term windows: the loudest moments and the quietest that still count as
// programme rather than as background.
func loudnessRange(blocks []float64) float64 {
	gated := gatedBlocks(blocks, rangeGateLU)
	if len(gated) < 2 {
		return 0
	}

	levels := make([]float64, 0, len(gated))
	for _, ms := range gated {
		levels = append(levels, lufs(ms))
	}
	sortFloats(levels)

	return percentile(levels, 0.95) - percentile(levels, 0.10)
}

// mean returns the arithmetic mean of a non-empty slice.
func mean(values []float64) float64 {
	var total float64
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}

// percentile returns the value at a fraction through a sorted slice.
func percentile(sorted []float64, fraction float64) float64 {
	if len(sorted) == 0 {
		return math.Inf(-1)
	}
	i := int(fraction * float64(len(sorted)-1))
	return sorted[min(max(i, 0), len(sorted)-1)]
}

// sortFloats sorts in place. Insertion sort is enough: the slice holds one
// entry per three seconds of audio, so a long album is a few hundred values.
func sortFloats(values []float64) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
