package spectrogram

import (
	"image"
	"slices"
	"testing"
)

func TestGridlineText(t *testing.T) {
	tests := []struct {
		name string
		line Gridline
		want string
	}{
		{"explicit label", Gridline{Hz: 16000, Label: "128 kbps"}, "128 kbps"},
		{"whole kilohertz", Gridline{Hz: 16000}, "16k"},
		{"fractional kilohertz", Gridline{Hz: 18500}, "18.5k"},
		{"compact disc ceiling", Gridline{Hz: 22050}, "22.05k"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.line.Text(); got != tt.want {
				t.Errorf("Text() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDefaultGridlinesCoverTheReferenceFrequencies checks the ruler carries the
// frequencies the published guides list, since that is the whole reason it is
// drawn.
func TestDefaultGridlinesCoverTheReferenceFrequencies(t *testing.T) {
	want := []float64{16000, 18500, 19000, 19500, 20000, 20500, 22050}

	got := DefaultGridlines()
	if len(got) != len(want) {
		t.Fatalf("DefaultGridlines() has %d entries, want %d", len(got), len(want))
	}
	for i, line := range got {
		if line.Hz != want[i] {
			t.Errorf("gridline %d = %v Hz, want %v Hz", i, line.Hz, want[i])
		}
	}
	if !slices.IsSorted(want) {
		t.Error("the reference frequencies are not in ascending order")
	}
}

func TestParseGridlines(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    []Gridline
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"whitespace only", "   ", nil, false},
		{"single", "16000", []Gridline{{Hz: 16000}}, false},
		{"several", "16000,19000", []Gridline{{Hz: 16000}, {Hz: 19000}}, false},
		{"spaced", " 16000 , 19000 ", []Gridline{{Hz: 16000}, {Hz: 19000}}, false},
		{"labelled", "16000:low", []Gridline{{Hz: 16000, Label: "low"}}, false},
		{"trailing comma", "16000,", []Gridline{{Hz: 16000}}, false},
		{"fractional", "22050.5", []Gridline{{Hz: 22050.5}}, false},
		{"not a number", "sixteen", nil, true},
		{"empty frequency with label", ":label", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseGridlines(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseGridlines() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("ParseGridlines() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateGridlines(t *testing.T) {
	tests := []struct {
		name    string
		lines   []Gridline
		maxHz   float64
		wantErr bool
	}{
		{"none", nil, 24000, false},
		{"within range", []Gridline{{Hz: 16000}}, 24000, false},
		{"exactly at the ceiling", []Gridline{{Hz: 24000}}, 24000, false},
		{"above the ceiling", []Gridline{{Hz: 30000}}, 24000, true},
		{"zero", []Gridline{{Hz: 0}}, 24000, true},
		{"negative", []Gridline{{Hz: -1}}, 24000, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGridlines(tt.lines, tt.maxHz)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGridlines() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRowFor(t *testing.T) {
	tests := []struct {
		name   string
		hz     float64
		maxHz  float64
		height int
		want   int
	}{
		{"the ceiling is the top row", 1000, 1000, 100, 0},
		{"zero is the bottom row", 0, 1000, 100, 99},
		{"halfway", 500, 1000, 100, 50},
		{"above the ceiling clamps to the top", 2000, 1000, 100, 0},
		{"negative clamps to the bottom", -100, 1000, 100, 99},
		{"zero ceiling", 500, 0, 100, 0},
		{"zero height", 500, 1000, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RowFor(tt.hz, tt.maxHz, tt.height); got != tt.want {
				t.Errorf("RowFor(%v,%v,%d) = %d, want %d", tt.hz, tt.maxHz, tt.height, got, tt.want)
			}
		})
	}
}

func TestAnnotateRejectsBadInput(t *testing.T) {
	opts := DefaultOptions()
	opts.Width, opts.Height = 32, 32

	if err := Annotate(nil, opts); err == nil {
		t.Error("Annotate() with a nil image succeeded, want error")
	}

	img := newTestImage(t, 32, 32)
	bad := opts
	bad.Palette = "viridis"
	if err := Annotate(img, bad); err == nil {
		t.Error("Annotate() with an unknown palette succeeded, want error")
	}
}

func TestAnnotateWithoutGridlinesLeavesTheImageAlone(t *testing.T) {
	opts := DefaultOptions()
	opts.Width, opts.Height = 16, 16
	opts.Gridlines = nil

	img := newTestImage(t, 16, 16)
	before := slices.Clone(img.Pix)

	if err := Annotate(img, opts); err != nil {
		t.Fatalf("Annotate() error = %v", err)
	}
	if !slices.Equal(img.Pix, before) {
		t.Error("Annotate() changed the image despite having no gridlines")
	}
}

// TestAnnotateDrawsAtTheRightRow checks the ruler lands where the geometry
// says it should, which is the only property that makes it useful.
func TestAnnotateDrawsAtTheRightRow(t *testing.T) {
	const (
		width  = 64
		height = 100
		maxHz  = 1000
	)
	opts := DefaultOptions()
	opts.Width, opts.Height, opts.MaxHz = width, height, maxHz
	opts.Gridlines = []Gridline{{Hz: 500, Label: ""}}

	img := newTestImage(t, width, height)
	if err := Annotate(img, opts); err != nil {
		t.Fatalf("Annotate() error = %v", err)
	}

	want := RowFor(500, maxHz, height)
	if got := img.ColorIndexAt(0, want); got == 0 {
		t.Errorf("row %d was not drawn on", want)
	}

	// The dashes leave gaps, so a fully drawn row would be a bug of its own.
	var lit int
	for x := range width {
		if img.ColorIndexAt(x, want) != 0 {
			lit++
		}
	}
	if lit == width {
		t.Error("the rule is solid, want it dashed so content stays legible")
	}
	if lit == 0 {
		t.Error("the rule was not drawn at all")
	}
}

func TestAnnotateDrawsALabel(t *testing.T) {
	const width, height = 128, 100
	opts := DefaultOptions()
	opts.Width, opts.Height, opts.MaxHz = width, height, 1000
	opts.Gridlines = []Gridline{{Hz: 500}}

	img := newTestImage(t, width, height)
	if err := Annotate(img, opts); err != nil {
		t.Fatalf("Annotate() error = %v", err)
	}

	// Text sits above the rule, so some row between the top of the glyphs and
	// the rule itself must carry ink.
	row := RowFor(500, 1000, height)
	var drawn bool
	for y := max(row-16, 0); y < row; y++ {
		for x := range width {
			if img.ColorIndexAt(x, y) != 0 {
				drawn = true
			}
		}
	}
	if !drawn {
		t.Error("no label was drawn above the rule")
	}
}

// TestAnnotateLabelAtTheTopFallsBelowTheRule covers a line so close to the top
// edge that its label would be drawn off the picture.
func TestAnnotateLabelAtTheTopFallsBelowTheRule(t *testing.T) {
	const width, height = 128, 40
	opts := DefaultOptions()
	opts.Width, opts.Height, opts.MaxHz = width, height, 1000
	opts.Gridlines = []Gridline{{Hz: 1000}}

	img := newTestImage(t, width, height)
	if err := Annotate(img, opts); err != nil {
		t.Fatalf("Annotate() error = %v", err)
	}

	var drawn bool
	for y := 1; y < height; y++ {
		for x := range width {
			if img.ColorIndexAt(x, y) != 0 {
				drawn = true
			}
		}
	}
	if !drawn {
		t.Error("a label at the very top was dropped instead of moving below its rule")
	}
}

// newTestImage returns a blank indexed image using the default palette.
func newTestImage(t *testing.T, width, height int) *image.Paletted {
	t.Helper()
	p, err := NewPalette(DefaultPalette)
	if err != nil {
		t.Fatalf("NewPalette() error = %v", err)
	}
	return image.NewPaletted(image.Rect(0, 0, width, height), p.Colors())
}
