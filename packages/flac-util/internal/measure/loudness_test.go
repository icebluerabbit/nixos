package measure

import (
	"math"
	"testing"
)

// toneBlock returns one block of a sine at the given frequency and peak level,
// for every channel, starting at the given sample offset.
func toneBlock(f Format, hz, peakDBFS float64, offset, samples int) Block {
	amplitude := math.Pow(10, peakDBFS/20) * f.FullScale()

	out := make([][]int32, f.Channels)
	for c := range out {
		out[c] = make([]int32, samples)
		for i := range out[c] {
			t := float64(offset+i) / float64(f.SampleRate)
			out[c][i] = int32(math.Round(amplitude * math.Sin(2*math.Pi*hz*t)))
		}
	}
	return Block{Samples: out}
}

// measureSine runs a sine of the given length through a Loudness accumulator.
func measureSine(f Format, hz, peakDBFS float64, seconds float64) LoudnessResult {
	a := NewLoudness(f)

	const blockSize = 4096
	total := int(seconds * float64(f.SampleRate))

	for offset := 0; offset < total; offset += blockSize {
		n := min(blockSize, total-offset)
		a.Write(toneBlock(f, hz, peakDBFS, offset, n))
	}
	return a.Result()
}

// TestLoudnessMatchesTheComplianceSignals is the check that says whether any
// of this is worth recording.
//
// EBU Tech 3341 defines test signals and the readings a conforming meter must
// give. A 1 kHz sine at -18 dBFS peak reads -18.0 LUFS; the same signal 5 dB
// quieter reads 5 LU quieter. They are run at every rate the library holds,
// because the filter coefficients are derived per rate and a mistake there
// would show up here and nowhere else.
func TestLoudnessMatchesTheComplianceSignals(t *testing.T) {
	// Tech 3341 allows ±0.1 LU. The margin here is a little wider because a
	// sine quantised to 16 bits is not exactly the signal the standard
	// describes.
	const tolerance = 0.15

	for _, rate := range []int{44100, 48000, 96000} {
		for _, peak := range []float64{-18, -23, -30} {
			f := Format{SampleRate: rate, Channels: 2, BitsPerSample: 16}
			got := measureSine(f, 1000, peak, 10).IntegratedLUFS

			if math.Abs(got-peak) > tolerance {
				t.Errorf("%d Hz, 1 kHz at %.0f dBFS: %.2f LUFS, want %.2f",
					rate, peak, got, peak)
			}
		}
	}
}

// TestLoudnessIsRelativelyCorrect checks the scale, independently of where
// zero sits: halving the amplitude has to cost 6 LU.
func TestLoudnessIsRelativelyCorrect(t *testing.T) {
	f := Format{SampleRate: 48000, Channels: 2, BitsPerSample: 16}

	loud := measureSine(f, 1000, -18, 10).IntegratedLUFS
	quiet := measureSine(f, 1000, -24, 10).IntegratedLUFS

	if got := loud - quiet; math.Abs(got-6) > 0.1 {
		t.Errorf("6 dB quieter measured %.2f LU quieter, want 6", got)
	}
}

// TestLoudnessWeightsFrequency checks the K-weighting reaches the result: the
// same amplitude at 10 kHz has to read louder than at 1 kHz, by the height of
// the shelf.
func TestLoudnessWeightsFrequency(t *testing.T) {
	f := Format{SampleRate: 48000, Channels: 2, BitsPerSample: 16}

	mid := measureSine(f, 1000, -20, 10).IntegratedLUFS
	high := measureSine(f, 10000, -20, 10).IntegratedLUFS

	if got := high - mid; got < 3 || got > 4 {
		t.Errorf("10 kHz read %.2f LU above 1 kHz, want the shelf, roughly 3.3", got)
	}
}

// TestLoudnessGatesSilence is what the two-pass gate is for: a track with a
// long silent tail has to read as loud as the same track without one.
func TestLoudnessGatesSilence(t *testing.T) {
	f := Format{SampleRate: 48000, Channels: 2, BitsPerSample: 16}

	short := measureSine(f, 1000, -18, 10)

	withSilence := NewLoudness(f)
	const blockSize = 4096
	for offset := 0; offset < 10*f.SampleRate; offset += blockSize {
		withSilence.Write(toneBlock(f, 1000, -18, offset, blockSize))
	}
	// Thirty seconds of digital silence after it.
	silent := Block{Samples: [][]int32{make([]int32, blockSize), make([]int32, blockSize)}}
	for range 30 * f.SampleRate / blockSize {
		withSilence.Write(silent)
	}

	got := withSilence.Result()
	if math.Abs(got.IntegratedLUFS-short.IntegratedLUFS) > 0.2 {
		t.Errorf("with silence %.2f LUFS, without %.2f: the gate is not discarding it",
			got.IntegratedLUFS, short.IntegratedLUFS)
	}
	if got.GatedBlocks >= got.Blocks {
		t.Errorf("%d of %d blocks survived the gate; the silence was counted",
			got.GatedBlocks, got.Blocks)
	}
}

func TestLoudnessOfSilence(t *testing.T) {
	f := Format{SampleRate: 48000, Channels: 2, BitsPerSample: 16}
	a := NewLoudness(f)

	silent := Block{Samples: [][]int32{make([]int32, 4096), make([]int32, 4096)}}
	for range 100 {
		a.Write(silent)
	}

	got := a.Result()
	if !math.IsInf(got.IntegratedLUFS, -1) {
		t.Errorf("IntegratedLUFS = %v, want negative infinity", got.IntegratedLUFS)
	}
	if got.GatedBlocks != 0 {
		t.Errorf("GatedBlocks = %d, want none", got.GatedBlocks)
	}
}

func TestLoudnessOfNothing(t *testing.T) {
	f := Format{SampleRate: 48000, Channels: 2, BitsPerSample: 16}
	got := NewLoudness(f).Result()

	if !math.IsInf(got.IntegratedLUFS, -1) {
		t.Errorf("IntegratedLUFS = %v, want negative infinity", got.IntegratedLUFS)
	}
	if got.Blocks != 0 {
		t.Errorf("Blocks = %d, want none", got.Blocks)
	}
	if got.RangeLU != 0 {
		t.Errorf("RangeLU = %v, want zero", got.RangeLU)
	}
}

// TestLoudnessRangeOfASteadySignal checks a signal that never changes level
// has no loudness range to speak of.
func TestLoudnessRangeOfASteadySignal(t *testing.T) {
	f := Format{SampleRate: 48000, Channels: 2, BitsPerSample: 16}

	if got := measureSine(f, 1000, -18, 30).RangeLU; got > 1 {
		t.Errorf("RangeLU = %.2f for a steady tone, want it near zero", got)
	}
}

// TestLoudnessRangeSpansQuietAndLoud checks the range measure notices a
// programme that changes level.
func TestLoudnessRangeSpansQuietAndLoud(t *testing.T) {
	f := Format{SampleRate: 48000, Channels: 2, BitsPerSample: 16}
	a := NewLoudness(f)

	const blockSize = 4096
	write := func(peakDBFS float64, seconds int) {
		total := seconds * f.SampleRate
		for offset := 0; offset < total; offset += blockSize {
			a.Write(toneBlock(f, 1000, peakDBFS, offset, min(blockSize, total-offset)))
		}
	}

	// Twenty seconds loud, twenty quiet: a 10 LU spread.
	write(-18, 20)
	write(-28, 20)

	if got := a.Result().RangeLU; got < 5 {
		t.Errorf("RangeLU = %.2f across a 10 LU change, want it to notice", got)
	}
}

func TestChannelWeights(t *testing.T) {
	// Left, right and centre count fully; the surround pair counts for more.
	got := channelWeights(6)
	want := []float64{1.0, 1.0, 1.0, 1.41, 1.41, 1.0}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("channel %d weighted %v, want %v", i, got[i], want[i])
		}
	}
	if len(channelWeights(2)) != 2 {
		t.Error("a stereo stream got the wrong number of weights")
	}
}

func TestLufsOfZero(t *testing.T) {
	if got := lufs(0); !math.IsInf(got, -1) {
		t.Errorf("lufs(0) = %v, want negative infinity", got)
	}
}

func TestPercentile(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		fraction float64
		want     float64
	}{
		{0, 1},
		{0.5, 5},
		{1, 10},
	}
	for _, tt := range tests {
		if got := percentile(sorted, tt.fraction); got != tt.want {
			t.Errorf("percentile(%v) = %v, want %v", tt.fraction, got, tt.want)
		}
	}
	if got := percentile(nil, 0.5); !math.IsInf(got, -1) {
		t.Errorf("percentile of nothing = %v, want negative infinity", got)
	}
}

func TestSortFloats(t *testing.T) {
	values := []float64{3, 1, 4, 1, 5, 9, 2, 6}
	sortFloats(values)

	for i := 1; i < len(values); i++ {
		if values[i] < values[i-1] {
			t.Fatalf("not sorted: %v", values)
		}
	}
}

// TestSlidingWindowCoversExactlyItsOwnSamples is the property the previous
// implementation did not have.
//
// A window has to be the sum of the samples inside it and nothing else. Scaling
// one running sum down as the window advances leaves every sample contributing
// forever, so a loud passage would go on inflating the windows after it.
func TestSlidingWindowCoversExactlyItsOwnSamples(t *testing.T) {
	// Four steps of two samples each: an eight-sample window advancing by two.
	w := newSlidingWindow(1, 8, 0.75)

	if w.steps != 4 {
		t.Fatalf("steps = %d, want 4", w.steps)
	}
	if w.length() != 8 {
		t.Fatalf("length() = %d, want 8", w.length())
	}

	// Feed 1, 2, 3, ... and check each completed window sums its own eight.
	var got []float64
	for i := 1; i <= 16; i++ {
		w.add(0, float64(i))
		if total, done := w.advance(); done {
			got = append(got, total[0])
		}
	}

	// First window covers 1..8, then 3..10, 5..12, and so on.
	want := []float64{36, 52, 68, 84, 100}
	if len(got) != len(want) {
		t.Fatalf("%d windows, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("window %d summed %v, want %v", i, got[i], want[i])
		}
	}
}

// TestSlidingWindowForgetsWhatLeavesIt is the same property stated the other
// way: a loud burst must stop counting once it has passed out of the window.
func TestSlidingWindowForgetsWhatLeavesIt(t *testing.T) {
	w := newSlidingWindow(1, 8, 0.75)

	// One very loud step, then silence.
	for range 2 {
		w.add(0, 1000)
		w.advance()
	}
	var last float64
	for range 20 {
		w.add(0, 0)
		if total, done := w.advance(); done {
			last = total[0]
		}
	}

	if last != 0 {
		t.Errorf("the window still holds %v after the burst left it", last)
	}
}

func TestSlidingWindowNeedsAWholeWindowFirst(t *testing.T) {
	w := newSlidingWindow(2, 8, 0.75)

	// Three steps is not yet a window.
	for range 6 {
		w.add(0, 1)
		w.add(1, 1)
		if _, done := w.advance(); done {
			t.Fatal("a window completed before enough samples arrived")
		}
	}
}
