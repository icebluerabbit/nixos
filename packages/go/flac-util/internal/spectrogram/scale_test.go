package spectrogram

import (
	"math"
	"testing"

	"homelab/flac-util/internal/measure"
)

// testGrid builds a grid with a deliberately coarse frequency scale, so that
// the mapping from bins to rows can be reasoned about by hand. With a sample
// rate of 2000 and a transform of four, bins sit at 0, 500 and 1000 Hz.
func testGrid(columns [][]float32) *measure.Grid {
	return &measure.Grid{
		Power:      columns,
		Bins:       3,
		SampleRate: 2000,
		FFTSize:    4,
		Hop:        2,
	}
}

func TestScaleRejectsBadInput(t *testing.T) {
	tests := []struct {
		name   string
		grid   *measure.Grid
		width  int
		height int
		maxHz  float64
	}{
		{"nil grid", nil, 4, 4, 1000},
		{"zero width", testGrid([][]float32{{0, 0, 0}}), 0, 4, 1000},
		{"zero height", testGrid([][]float32{{0, 0, 0}}), 4, 0, 1000},
		{"zero ceiling", testGrid([][]float32{{0, 0, 0}}), 4, 4, 0},
		{"grid without columns", testGrid(nil), 4, 4, 1000},
		{"grid without a frequency scale", &measure.Grid{Power: [][]float32{{1}}, Bins: 1}, 4, 4, 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Scale(tt.grid, tt.width, tt.height, tt.maxHz); err == nil {
				t.Fatal("Scale() succeeded, want error")
			}
		})
	}
}

// TestScalePutsHighFrequenciesAtTheTop pins the orientation of the picture,
// which every later stage and every reader depends on.
func TestScalePutsHighFrequenciesAtTheTop(t *testing.T) {
	// Power only in the highest bin, at full scale.
	grid := testGrid([][]float32{{0, 0, 1}})

	levels, err := Scale(grid, 1, 2, 1000)
	if err != nil {
		t.Fatalf("Scale() error = %v", err)
	}

	if got := levels.At(0, 0); !closeTo(got, 0, 1e-6) {
		t.Errorf("top row = %v dB, want 0", got)
	}
	if got := levels.At(0, 1); !math.IsInf(got, -1) {
		t.Errorf("bottom row = %v, want -Inf", got)
	}
}

func TestScalePutsLowFrequenciesAtTheBottom(t *testing.T) {
	grid := testGrid([][]float32{{1, 0, 0}})

	levels, err := Scale(grid, 1, 2, 1000)
	if err != nil {
		t.Fatalf("Scale() error = %v", err)
	}

	if got := levels.At(0, 1); !closeTo(got, 0, 1e-6) {
		t.Errorf("bottom row = %v dB, want 0", got)
	}
}

// TestScaleTakesThePeakNotTheMean is the reason the resampling exists in this
// form: a narrow band of quiet content must survive being squeezed into a
// pixel, because that is exactly what a reader is looking for.
func TestScaleTakesThePeakNotTheMean(t *testing.T) {
	// Sixteen bins, only one of them carrying anything, all squeezed into a
	// single row.
	power := make([]float32, 16)
	power[9] = 1
	grid := &measure.Grid{
		Power:      [][]float32{power},
		Bins:       16,
		SampleRate: 32,
		FFTSize:    32,
	}

	levels, err := Scale(grid, 1, 1, 16)
	if err != nil {
		t.Fatalf("Scale() error = %v", err)
	}
	if got := levels.At(0, 0); !closeTo(got, 0, 1e-6) {
		t.Errorf("level = %v dB, want 0 - the loud bin was averaged away", got)
	}
}

func TestScaleResamplesColumns(t *testing.T) {
	// Four columns of increasing level squeezed into two pixels: each output
	// pixel takes the louder of the pair it covers.
	grid := testGrid([][]float32{
		{0, 0, 0.001},
		{0, 0, 0.01},
		{0, 0, 0.1},
		{0, 0, 1},
	})

	levels, err := Scale(grid, 2, 1, 1000)
	if err != nil {
		t.Fatalf("Scale() error = %v", err)
	}
	if got := levels.At(0, 0); !closeTo(got, -20, 1e-6) {
		t.Errorf("left pixel = %v dB, want -20", got)
	}
	if got := levels.At(1, 0); !closeTo(got, 0, 1e-6) {
		t.Errorf("right pixel = %v dB, want 0", got)
	}
}

func TestScaleStretchesShortGrids(t *testing.T) {
	// A grid with fewer columns than the picture is wide must still fill it.
	grid := testGrid([][]float32{{0, 0, 1}})

	levels, err := Scale(grid, 8, 1, 1000)
	if err != nil {
		t.Fatalf("Scale() error = %v", err)
	}
	for x := range 8 {
		if got := levels.At(x, 0); !closeTo(got, 0, 1e-6) {
			t.Errorf("pixel %d = %v dB, want 0", x, got)
		}
	}
}

func TestScaleAboveTheGridsRangeIsSilent(t *testing.T) {
	// A ceiling well above what the grid can represent leaves the upper part
	// of the picture empty rather than repeating the topmost bin.
	grid := testGrid([][]float32{{1, 1, 1}})

	levels, err := Scale(grid, 1, 4, 4000)
	if err != nil {
		t.Fatalf("Scale() error = %v", err)
	}
	if got := levels.At(0, 0); !math.IsInf(got, -1) {
		t.Errorf("top row = %v, want -Inf above the grid's range", got)
	}
}

func TestScaleSilenceIsNegativeInfinity(t *testing.T) {
	grid := testGrid([][]float32{{0, 0, 0}})

	levels, err := Scale(grid, 2, 2, 1000)
	if err != nil {
		t.Fatalf("Scale() error = %v", err)
	}
	for y := range 2 {
		for x := range 2 {
			if got := levels.At(x, y); !math.IsInf(got, -1) {
				t.Errorf("pixel (%d,%d) = %v, want -Inf", x, y, got)
			}
		}
	}
}

func TestLevelsAtOutOfBounds(t *testing.T) {
	levels := &Levels{Width: 2, Height: 2, DB: make([]float32, 4)}

	for _, p := range [][2]int{{-1, 0}, {0, -1}, {2, 0}, {0, 2}, {5, 5}} {
		if got := levels.At(p[0], p[1]); !math.IsInf(got, -1) {
			t.Errorf("At(%d,%d) = %v, want -Inf", p[0], p[1], got)
		}
	}
}

func TestScaleDimensions(t *testing.T) {
	grid := testGrid([][]float32{{1, 1, 1}})

	levels, err := Scale(grid, 7, 5, 1000)
	if err != nil {
		t.Fatalf("Scale() error = %v", err)
	}
	if levels.Width != 7 || levels.Height != 5 {
		t.Errorf("dimensions = %dx%d, want 7x5", levels.Width, levels.Height)
	}
	if len(levels.DB) != 35 {
		t.Errorf("len(DB) = %d, want 35", len(levels.DB))
	}
	if levels.MaxHz != 1000 {
		t.Errorf("MaxHz = %v, want 1000", levels.MaxHz)
	}
}

// closeTo reports whether got is within tolerance of want.
func closeTo(got, want, tolerance float64) bool {
	return math.Abs(got-want) <= tolerance
}
