package measure

import (
	"math"
	"testing"
)

func TestStereoName(t *testing.T) {
	if got := NewStereo(testFormat()).Name(); got != "stereo" {
		t.Errorf("Name() = %q, want %q", got, "stereo")
	}
}

func TestStereoNotApplicableToMono(t *testing.T) {
	a := NewStereo(Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16})
	a.Write(Block{Samples: [][]int32{{1, 2, 3}}})
	got := a.Result()

	if got.Applicable {
		t.Error("Applicable = true for a mono stream, want false")
	}
	if !math.IsNaN(got.Correlation) {
		t.Errorf("Correlation = %v, want NaN", got.Correlation)
	}
	if got.TotalSamples != 0 {
		t.Errorf("TotalSamples = %d, want 0", got.TotalSamples)
	}
}

func TestStereoEmptyStream(t *testing.T) {
	got := NewStereo(testFormat()).Result()

	if !got.Applicable {
		t.Error("Applicable = false for a stereo format, want true")
	}
	if !math.IsNaN(got.Correlation) {
		t.Errorf("Correlation = %v, want NaN", got.Correlation)
	}
}

func TestStereoCorrelation(t *testing.T) {
	tests := []struct {
		name  string
		left  []int32
		right []int32
		want  float64
	}{
		{
			name:  "duplicated channel correlates perfectly",
			left:  []int32{100, -200, 300, -50},
			right: []int32{100, -200, 300, -50},
			want:  1,
		},
		{
			name:  "inverted channel correlates negatively",
			left:  []int32{100, -200, 300, -50},
			right: []int32{-100, 200, -300, 50},
			want:  -1,
		},
		{
			name:  "scaled copy still correlates perfectly",
			left:  []int32{100, -200, 300, -50},
			right: []int32{200, -400, 600, -100},
			want:  1,
		},
		{
			name:  "offset copy still correlates perfectly",
			left:  []int32{100, -200, 300, -50},
			right: []int32{110, -190, 310, -40},
			want:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewStereo(testFormat())
			a.Write(Block{Samples: [][]int32{tt.left, tt.right}})

			if got := a.Result().Correlation; !closeTo(got, tt.want, 1e-9) {
				t.Errorf("Correlation = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStereoCorrelationIsBounded(t *testing.T) {
	a := NewStereo(testFormat())
	for range 100 {
		a.Write(Block{Samples: [][]int32{
			{32767, -32768, 12345},
			{32767, -32768, 12345},
		}})
	}

	got := a.Result().Correlation
	if got < -1 || got > 1 {
		t.Errorf("Correlation = %v, want a value within [-1,1]", got)
	}
}

func TestStereoConstantChannelHasNoCorrelation(t *testing.T) {
	a := NewStereo(testFormat())
	a.Write(Block{Samples: [][]int32{
		{5, 5, 5, 5},
		{1, 2, 3, 4},
	}})

	if got := a.Result().Correlation; !math.IsNaN(got) {
		t.Errorf("Correlation = %v, want NaN for a constant channel", got)
	}
}

func TestStereoIdenticalSampleCount(t *testing.T) {
	tests := []struct {
		name  string
		left  []int32
		right []int32
		want  uint64
	}{
		{"none alike", []int32{1, 2, 3}, []int32{4, 5, 6}, 0},
		{"all alike", []int32{1, 2, 3}, []int32{1, 2, 3}, 3},
		{"some alike", []int32{1, 2, 3}, []int32{1, 9, 3}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewStereo(testFormat())
			a.Write(Block{Samples: [][]int32{tt.left, tt.right}})
			got := a.Result()

			if got.IdenticalSamples != tt.want {
				t.Errorf("IdenticalSamples = %d, want %d", got.IdenticalSamples, tt.want)
			}
			if got.TotalSamples != uint64(len(tt.left)) {
				t.Errorf("TotalSamples = %d, want %d", got.TotalSamples, len(tt.left))
			}
		})
	}
}

func TestStereoIsIndependentOfBlocking(t *testing.T) {
	whole := NewStereo(testFormat())
	whole.Write(Block{Samples: [][]int32{{1, 2, 3, 4}, {2, 5, 3, 9}}})

	split := NewStereo(testFormat())
	split.Write(Block{Samples: [][]int32{{1, 2}, {2, 5}}})
	split.Write(Block{Samples: [][]int32{{3, 4}, {3, 9}}})

	a, b := whole.Result(), split.Result()
	if !closeTo(a.Correlation, b.Correlation, 1e-12) {
		t.Errorf("Correlation %v then %v", a.Correlation, b.Correlation)
	}
	if a.IdenticalSamples != b.IdenticalSamples {
		t.Errorf("IdenticalSamples %d then %d", a.IdenticalSamples, b.IdenticalSamples)
	}
}

func TestStereoHandlesRaggedBlock(t *testing.T) {
	// A malformed block whose channels differ in length must not panic; the
	// shorter channel bounds the comparison.
	a := NewStereo(testFormat())
	a.Write(Block{Samples: [][]int32{{1, 2, 3}, {1, 2}}})

	if got := a.Result().TotalSamples; got != 2 {
		t.Errorf("TotalSamples = %d, want 2", got)
	}
}
