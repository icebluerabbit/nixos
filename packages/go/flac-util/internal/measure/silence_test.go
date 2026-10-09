package measure

import "testing"

func TestSilenceName(t *testing.T) {
	if got := NewSilence(testFormat()).Name(); got != "silence" {
		t.Errorf("Name() = %q, want %q", got, "silence")
	}
}

func TestSilenceEmptyStream(t *testing.T) {
	got := NewSilence(testFormat()).Result()

	if got.TotalSamples != 0 {
		t.Errorf("TotalSamples = %d, want 0", got.TotalSamples)
	}
	if got.AllSilent {
		t.Error("AllSilent = true for an empty stream, want false")
	}
}

func TestSilenceRuns(t *testing.T) {
	tests := []struct {
		name         string
		left         []int32
		right        []int32
		wantLeading  uint64
		wantTrailing uint64
		wantAll      bool
	}{
		{
			name:         "no silence at all",
			left:         []int32{1, 2, 3},
			right:        []int32{1, 2, 3},
			wantLeading:  0,
			wantTrailing: 0,
		},
		{
			name:         "leading silence only",
			left:         []int32{0, 0, 5},
			right:        []int32{0, 0, 5},
			wantLeading:  2,
			wantTrailing: 0,
		},
		{
			name:         "trailing silence only",
			left:         []int32{5, 0, 0},
			right:        []int32{5, 0, 0},
			wantLeading:  0,
			wantTrailing: 2,
		},
		{
			name:         "silence at both ends",
			left:         []int32{0, 5, 0, 0},
			right:        []int32{0, 5, 0, 0},
			wantLeading:  1,
			wantTrailing: 2,
		},
		{
			name:         "silence in the middle is not counted",
			left:         []int32{5, 0, 0, 5},
			right:        []int32{5, 0, 0, 5},
			wantLeading:  0,
			wantTrailing: 0,
		},
		{
			name:         "entirely silent",
			left:         []int32{0, 0, 0},
			right:        []int32{0, 0, 0},
			wantLeading:  3,
			wantTrailing: 3,
			wantAll:      true,
		},
		{
			name:         "one channel non-zero breaks the silence",
			left:         []int32{0, 0},
			right:        []int32{0, 1},
			wantLeading:  1,
			wantTrailing: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewSilence(testFormat())
			a.Write(Block{Samples: [][]int32{tt.left, tt.right}})
			got := a.Result()

			if got.LeadingSamples != tt.wantLeading {
				t.Errorf("LeadingSamples = %d, want %d", got.LeadingSamples, tt.wantLeading)
			}
			if got.TrailingSamples != tt.wantTrailing {
				t.Errorf("TrailingSamples = %d, want %d", got.TrailingSamples, tt.wantTrailing)
			}
			if got.AllSilent != tt.wantAll {
				t.Errorf("AllSilent = %v, want %v", got.AllSilent, tt.wantAll)
			}
			if got.TotalSamples != uint64(len(tt.left)) {
				t.Errorf("TotalSamples = %d, want %d", got.TotalSamples, len(tt.left))
			}
		})
	}
}

func TestSilenceSpansBlockBoundaries(t *testing.T) {
	// A run of silence split across blocks must be counted as one run, which
	// is the case the streaming implementation is most likely to get wrong.
	a := NewSilence(testFormat())
	a.Write(Block{Samples: [][]int32{{0, 0}, {0, 0}}})
	a.Write(Block{Samples: [][]int32{{0, 7}, {0, 7}}})
	a.Write(Block{Samples: [][]int32{{0, 0}, {0, 0}}})
	a.Write(Block{Samples: [][]int32{{0}, {0}}})

	got := a.Result()
	if got.LeadingSamples != 3 {
		t.Errorf("LeadingSamples = %d, want 3", got.LeadingSamples)
	}
	if got.TrailingSamples != 3 {
		t.Errorf("TrailingSamples = %d, want 3", got.TrailingSamples)
	}
	if got.TotalSamples != 7 {
		t.Errorf("TotalSamples = %d, want 7", got.TotalSamples)
	}
	if got.AllSilent {
		t.Error("AllSilent = true, want false")
	}
}

func TestSilenceIsIndependentOfBlocking(t *testing.T) {
	whole := NewSilence(testFormat())
	whole.Write(Block{Samples: [][]int32{{0, 0, 4, 0}, {0, 0, 4, 0}}})

	split := NewSilence(testFormat())
	split.Write(Block{Samples: [][]int32{{0}, {0}}})
	split.Write(Block{Samples: [][]int32{{0, 4}, {0, 4}}})
	split.Write(Block{Samples: [][]int32{{0}, {0}}})

	if whole.Result() != split.Result() {
		t.Errorf("blocking changed the result: %+v then %+v", whole.Result(), split.Result())
	}
}

func TestSilenceIgnoresEmptyBlocks(t *testing.T) {
	a := NewSilence(testFormat())
	a.Write(Block{})
	a.Write(Block{Samples: [][]int32{{0}, {0}}})
	a.Write(Block{Samples: [][]int32{{}, {}}})

	if got := a.Result().TotalSamples; got != 1 {
		t.Errorf("TotalSamples = %d, want 1", got)
	}
}
