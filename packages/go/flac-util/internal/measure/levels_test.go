package measure

import (
	"math"
	"testing"
)

func TestLevelsName(t *testing.T) {
	if got := NewLevels(testFormat()).Name(); got != "levels" {
		t.Errorf("Name() = %q, want %q", got, "levels")
	}
}

func TestLevelsEmptyStream(t *testing.T) {
	got := NewLevels(testFormat()).Result()

	if got.SampleCount != 0 {
		t.Errorf("SampleCount = %d, want 0", got.SampleCount)
	}
	if len(got.Channels) != 2 {
		t.Fatalf("len(Channels) = %d, want 2", len(got.Channels))
	}
	for i, ch := range got.Channels {
		if !math.IsInf(ch.PeakDBFS, -1) {
			t.Errorf("channel %d PeakDBFS = %v, want -Inf", i, ch.PeakDBFS)
		}
		if !math.IsInf(ch.RMSDBFS, -1) {
			t.Errorf("channel %d RMSDBFS = %v, want -Inf", i, ch.RMSDBFS)
		}
	}
}

func TestLevelsPeak(t *testing.T) {
	tests := []struct {
		name    string
		samples []int32
		want    int32
	}{
		{"positive peak", []int32{100, 3000, 20}, 3000},
		{"negative peak", []int32{-100, -3000, 20}, 3000},
		{"most negative sample clamps to the largest magnitude", []int32{-32768}, 32767},
		{"silence", []int32{0, 0}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewLevels(Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16})
			a.Write(Block{Samples: [][]int32{tt.samples}})
			if got := a.Result().Channels[0].PeakSample; got != tt.want {
				t.Errorf("PeakSample = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestLevelsPeakDBFS(t *testing.T) {
	a := NewLevels(Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16})
	// Half of full scale is six decibels down.
	a.Write(Block{Samples: [][]int32{{16384}}})

	if got := a.Result().Channels[0].PeakDBFS; !closeTo(got, -6.0206, 1e-3) {
		t.Errorf("PeakDBFS = %v, want approximately -6.02", got)
	}
}

func TestLevelsRMS(t *testing.T) {
	a := NewLevels(Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16})
	// A square wave alternating at half scale has an RMS equal to its
	// amplitude, so it lands six decibels down as well.
	a.Write(Block{Samples: [][]int32{{16384, -16384, 16384, -16384}}})

	if got := a.Result().Channels[0].RMSDBFS; !closeTo(got, -6.0206, 1e-3) {
		t.Errorf("RMSDBFS = %v, want approximately -6.02", got)
	}
}

func TestLevelsClipping(t *testing.T) {
	tests := []struct {
		name    string
		bits    int
		samples []int32
		want    uint64
	}{
		{"no clipping", 16, []int32{100, -100}, 0},
		{"positive limit", 16, []int32{32767}, 1},
		{"negative limit", 16, []int32{-32768}, 1},
		{"both limits repeatedly", 16, []int32{32767, -32768, 32767}, 3},
		{"one below the limit is clean", 16, []int32{32766, -32767}, 0},
		{"eight bit limits", 8, []int32{127, -128, 0}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewLevels(Format{SampleRate: 44100, Channels: 1, BitsPerSample: tt.bits})
			a.Write(Block{Samples: [][]int32{tt.samples}})
			if got := a.Result().Channels[0].ClippedSamples; got != tt.want {
				t.Errorf("ClippedSamples = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestLevelsDCOffset(t *testing.T) {
	tests := []struct {
		name    string
		samples []int32
		want    float64
	}{
		{"centred", []int32{1000, -1000}, 0},
		{"offset upwards", []int32{16384, 16384}, 0.5},
		{"offset downwards", []int32{-8192, -8192}, -0.25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewLevels(Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16})
			a.Write(Block{Samples: [][]int32{tt.samples}})
			if got := a.Result().Channels[0].DCOffset; !closeTo(got, tt.want, 1e-9) {
				t.Errorf("DCOffset = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLevelsTracksChannelsIndependently(t *testing.T) {
	a := NewLevels(testFormat())
	a.Write(Block{Samples: [][]int32{
		{100, 200},
		{32767, -32768},
	}})
	got := a.Result()

	if got.Channels[0].PeakSample != 200 {
		t.Errorf("left PeakSample = %d, want 200", got.Channels[0].PeakSample)
	}
	if got.Channels[0].ClippedSamples != 0 {
		t.Errorf("left ClippedSamples = %d, want 0", got.Channels[0].ClippedSamples)
	}
	if got.Channels[1].ClippedSamples != 2 {
		t.Errorf("right ClippedSamples = %d, want 2", got.Channels[1].ClippedSamples)
	}
}

func TestLevelsIsIndependentOfBlocking(t *testing.T) {
	whole := NewLevels(testFormat())
	whole.Write(Block{Samples: [][]int32{{1, 2, 3, 4}, {5, 6, 7, 8}}})

	split := NewLevels(testFormat())
	split.Write(Block{Samples: [][]int32{{1, 2}, {5, 6}}})
	split.Write(Block{Samples: [][]int32{{3, 4}, {7, 8}}})

	a, b := whole.Result(), split.Result()
	if a.SampleCount != b.SampleCount {
		t.Fatalf("SampleCount %d then %d", a.SampleCount, b.SampleCount)
	}
	for i := range a.Channels {
		if a.Channels[i] != b.Channels[i] {
			t.Errorf("channel %d differs: %+v then %+v", i, a.Channels[i], b.Channels[i])
		}
	}
}

func TestLevelsIgnoresExtraChannels(t *testing.T) {
	// A malformed block carrying more channels than the format declares must
	// not panic or corrupt the channels that do exist.
	a := NewLevels(Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16})
	a.Write(Block{Samples: [][]int32{{100}, {200}}})

	got := a.Result()
	if len(got.Channels) != 1 {
		t.Fatalf("len(Channels) = %d, want 1", len(got.Channels))
	}
	if got.Channels[0].PeakSample != 100 {
		t.Errorf("PeakSample = %d, want 100", got.Channels[0].PeakSample)
	}
}
