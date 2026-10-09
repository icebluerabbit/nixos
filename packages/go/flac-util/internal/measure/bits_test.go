package measure

import "testing"

func TestBitsName(t *testing.T) {
	if got := NewBits(testFormat()).Name(); got != "bits" {
		t.Errorf("Name() = %q, want %q", got, "bits")
	}
}

func TestBitsEmptyStreamReportsAllZero(t *testing.T) {
	got := NewBits(testFormat()).Result()

	if got.DeclaredBits != 16 {
		t.Errorf("DeclaredBits = %d, want 16", got.DeclaredBits)
	}
	if len(got.Channels) != 2 {
		t.Fatalf("len(Channels) = %d, want 2", len(got.Channels))
	}
	for i, ch := range got.Channels {
		if !ch.AllZero {
			t.Errorf("channel %d AllZero = false, want true", i)
		}
	}
	if got.EffectiveBits != 0 {
		t.Errorf("EffectiveBits = %d, want 0", got.EffectiveBits)
	}
}

func TestBitsEffectiveResolution(t *testing.T) {
	tests := []struct {
		name          string
		declared      int
		samples       []int32
		wantUnused    int
		wantEffective int
	}{
		{
			name:          "genuine sixteen bit uses every bit",
			declared:      16,
			samples:       []int32{1, 2, 3},
			wantUnused:    0,
			wantEffective: 16,
		},
		{
			name:          "sixteen bit audio padded into a twenty four bit container",
			declared:      24,
			samples:       []int32{1 << 8, 3 << 8, -5 << 8},
			wantUnused:    8,
			wantEffective: 16,
		},
		{
			name:          "genuine twenty four bit",
			declared:      24,
			samples:       []int32{1 << 8, 3 << 8, 1},
			wantUnused:    0,
			wantEffective: 24,
		},
		{
			name:          "eight bit audio padded into a twenty four bit container",
			declared:      24,
			samples:       []int32{1 << 16, 7 << 16},
			wantUnused:    16,
			wantEffective: 8,
		},
		{
			name:          "negative samples carry the same low bits",
			declared:      24,
			samples:       []int32{-256, -512},
			wantUnused:    8,
			wantEffective: 16,
		},
		{
			name:          "a single odd sample proves full resolution",
			declared:      24,
			samples:       []int32{1 << 8, 1 << 8, 1 << 8, 1},
			wantUnused:    0,
			wantEffective: 24,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewBits(Format{SampleRate: 44100, Channels: 1, BitsPerSample: tt.declared})
			a.Write(Block{Samples: [][]int32{tt.samples}})
			got := a.Result()

			ch := got.Channels[0]
			if ch.AllZero {
				t.Fatal("AllZero = true, want false")
			}
			if ch.UnusedLowBits != tt.wantUnused {
				t.Errorf("UnusedLowBits = %d, want %d", ch.UnusedLowBits, tt.wantUnused)
			}
			if ch.EffectiveBits != tt.wantEffective {
				t.Errorf("EffectiveBits = %d, want %d", ch.EffectiveBits, tt.wantEffective)
			}
			if got.EffectiveBits != tt.wantEffective {
				t.Errorf("overall EffectiveBits = %d, want %d", got.EffectiveBits, tt.wantEffective)
			}
		})
	}
}

func TestBitsOverallTakesTheRichestChannel(t *testing.T) {
	a := NewBits(Format{SampleRate: 96000, Channels: 2, BitsPerSample: 24})
	a.Write(Block{Samples: [][]int32{
		{1 << 8, 2 << 8}, // sixteen bits of detail
		{1, 2},           // twenty four bits of detail
	}})
	got := a.Result()

	if got.Channels[0].EffectiveBits != 16 {
		t.Errorf("left EffectiveBits = %d, want 16", got.Channels[0].EffectiveBits)
	}
	if got.Channels[1].EffectiveBits != 24 {
		t.Errorf("right EffectiveBits = %d, want 24", got.Channels[1].EffectiveBits)
	}
	if got.EffectiveBits != 24 {
		t.Errorf("overall EffectiveBits = %d, want 24", got.EffectiveBits)
	}
}

func TestBitsSilentChannelAlongsideActiveOne(t *testing.T) {
	a := NewBits(testFormat())
	a.Write(Block{Samples: [][]int32{
		{0, 0},
		{1, 2},
	}})
	got := a.Result()

	if !got.Channels[0].AllZero {
		t.Error("left AllZero = false, want true")
	}
	if got.Channels[1].AllZero {
		t.Error("right AllZero = true, want false")
	}
	if got.EffectiveBits != 16 {
		t.Errorf("EffectiveBits = %d, want 16", got.EffectiveBits)
	}
}

func TestBitsNeverExceedsDeclaredResolution(t *testing.T) {
	// Sample values wider than the declared resolution are malformed input;
	// the reported resolution must still be bounded by what was declared.
	a := NewBits(Format{SampleRate: 44100, Channels: 1, BitsPerSample: 8})
	a.Write(Block{Samples: [][]int32{{1 << 30}}})

	if got := a.Result().Channels[0].EffectiveBits; got > 8 {
		t.Errorf("EffectiveBits = %d, want at most 8", got)
	}
}

func TestBitsIsIndependentOfBlocking(t *testing.T) {
	format := Format{SampleRate: 96000, Channels: 1, BitsPerSample: 24}

	whole := NewBits(format)
	whole.Write(Block{Samples: [][]int32{{1 << 8, 1 << 8, 1}}})

	split := NewBits(format)
	split.Write(Block{Samples: [][]int32{{1 << 8}}})
	split.Write(Block{Samples: [][]int32{{1 << 8, 1}}})

	if whole.Result().EffectiveBits != split.Result().EffectiveBits {
		t.Errorf("blocking changed EffectiveBits: %d then %d",
			whole.Result().EffectiveBits, split.Result().EffectiveBits)
	}
}
