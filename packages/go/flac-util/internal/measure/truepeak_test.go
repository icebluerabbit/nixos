package measure

import (
	"math"
	"testing"
)

// measureTruePeak runs a signal through a TruePeak accumulator.
func measureTruePeak(f Format, samples []int32) TruePeakResult {
	a := NewTruePeak(f)
	a.Write(Block{Samples: [][]int32{samples}})
	return a.Result()
}

// TestTruePeakExceedsTheSamplePeak is the whole reason this measurement
// exists.
//
// A sine placed so that its crest falls between two samples never reaches full
// scale in the file, but the waveform does. A converter reconstructing it will
// clip where the sample values say there is headroom.
func TestTruePeakExceedsTheSamplePeak(t *testing.T) {
	f := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}
	full := f.FullScale()

	// A quarter-rate sine offset by half a sample: every sample lands either
	// side of the crest, never on it.
	samples := make([]int32, 4096)
	for i := range samples {
		phase := 2*math.Pi*0.25*float64(i) + math.Pi/4
		samples[i] = int32(math.Round((full - 1) * math.Sin(phase)))
	}

	var samplePeak int32
	for _, s := range samples {
		if abs := max(s, -s); abs > samplePeak {
			samplePeak = abs
		}
	}
	samplePeakDB := 20 * math.Log10(float64(samplePeak)/full)

	got := measureTruePeak(f, samples).ChannelsDBTP[0]
	if got <= samplePeakDB {
		t.Errorf("true peak %.3f dBTP against sample peak %.3f dBFS: "+
			"the waveform between samples was not seen", got, samplePeakDB)
	}
	// The crest of a quarter-rate sine sampled off-peak sits about 3 dB above
	// what the samples show.
	if got-samplePeakDB > 4 {
		t.Errorf("true peak %.3f dBTP is %.3f dB above the samples, which is too much",
			got, got-samplePeakDB)
	}
}

// TestTruePeakOfASteadySignalIsItsLevel checks the interpolation does not
// invent level where there is none to find.
func TestTruePeakOfASteadySignalIsItsLevel(t *testing.T) {
	f := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}

	// A constant at half scale. Reconstruction of a constant is the constant.
	samples := make([]int32, 4096)
	for i := range samples {
		samples[i] = int32(f.FullScale() / 2)
	}

	got := measureTruePeak(f, samples).ChannelsDBTP[0]
	if math.Abs(got-(-6.02)) > 0.2 {
		t.Errorf("true peak of a half-scale constant = %.3f dBTP, want about -6.02", got)
	}
}

func TestTruePeakOfSilence(t *testing.T) {
	f := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}

	got := measureTruePeak(f, make([]int32, 4096)).ChannelsDBTP[0]
	if !math.IsInf(got, -1) {
		t.Errorf("true peak of silence = %v, want negative infinity", got)
	}
}

func TestTruePeakMeasuresEveryChannel(t *testing.T) {
	f := Format{SampleRate: 44100, Channels: 2, BitsPerSample: 16}
	a := NewTruePeak(f)

	loud := make([]int32, 2048)
	for i := range loud {
		loud[i] = int32(math.Round((f.FullScale() - 1) * math.Sin(float64(i))))
	}
	a.Write(Block{Samples: [][]int32{loud, make([]int32, len(loud))}})

	got := a.Result()
	if len(got.ChannelsDBTP) != 2 {
		t.Fatalf("%d channels, want 2", len(got.ChannelsDBTP))
	}
	if got.ChannelsDBTP[0] <= -6 {
		t.Errorf("the loud channel measured %v dBTP", got.ChannelsDBTP[0])
	}
	if !math.IsInf(got.ChannelsDBTP[1], -1) {
		t.Errorf("the silent channel measured %v dBTP", got.ChannelsDBTP[1])
	}
	if got.Oversample != truePeakOversample {
		t.Errorf("Oversample = %d, want %d", got.Oversample, truePeakOversample)
	}
}

// TestSincPhasesSumToUnity checks each branch of the interpolation filter
// passes a constant unchanged. A branch that does not would shift the level of
// the reconstructed waveform rather than only its shape.
func TestSincPhasesSumToUnity(t *testing.T) {
	for p, phase := range sincPhases(truePeakOversample, truePeakTaps) {
		var sum float64
		for _, c := range phase {
			sum += c
		}
		if math.Abs(sum-1) > 1e-12 {
			t.Errorf("phase %d sums to %v, want 1", p, sum)
		}
	}
}

func TestSinc(t *testing.T) {
	if got := sinc(0); got != 1 {
		t.Errorf("sinc(0) = %v, want 1", got)
	}
	// Zero at every non-zero integer, which is what makes it reconstruct
	// samples exactly.
	for _, x := range []float64{1, 2, 3, -1, -2} {
		if got := sinc(x); math.Abs(got) > 1e-12 {
			t.Errorf("sinc(%v) = %v, want 0", x, got)
		}
	}
}

func TestTruePeakName(t *testing.T) {
	if got := NewTruePeak(Format{Channels: 1}).Name(); got == "" {
		t.Error("Name() is empty")
	}
}
