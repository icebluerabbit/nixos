package spectrogram

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"testing"
)

func TestDefaultOptionsAreValid(t *testing.T) {
	opts := DefaultOptions()
	if err := opts.Validate(); err != nil {
		t.Fatalf("DefaultOptions().Validate() error = %v", err)
	}

	if opts.Width != 1920 || opts.Height != 1080 {
		t.Errorf("default size = %dx%d, want 1920x1080", opts.Width, opts.Height)
	}
	if opts.MaxHz != 24000 {
		t.Errorf("default MaxHz = %v, want 24000", opts.MaxHz)
	}
	if opts.RangeDB != 120 {
		t.Errorf("default RangeDB = %v, want 120", opts.RangeDB)
	}
	if got := opts.FloorDB(); got != -120 {
		t.Errorf("FloorDB() = %v, want -120", got)
	}
}

func TestOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Options)
		wantErr bool
	}{
		{"defaults", func(*Options) {}, false},
		{"zero width", func(o *Options) { o.Width = 0 }, true},
		{"negative width", func(o *Options) { o.Width = -1 }, true},
		{"zero height", func(o *Options) { o.Height = 0 }, true},
		{"zero ceiling", func(o *Options) { o.MaxHz = 0 }, true},
		{"zero range", func(o *Options) { o.RangeDB = 0 }, true},
		{"negative range", func(o *Options) { o.RangeDB = -60 }, true},
		{"unknown palette", func(o *Options) { o.Palette = "viridis" }, true},
		{"gridline above the ceiling", func(o *Options) { o.MaxHz = 20000 }, true},
		{"no gridlines", func(o *Options) { o.Gridlines = nil }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultOptions()
			tt.mutate(&opts)
			if err := opts.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildRejectsBadInput(t *testing.T) {
	opts := DefaultOptions()
	opts.Width, opts.Height = 4, 4
	opts.Gridlines = nil

	if _, err := Build(nil, opts); err == nil {
		t.Error("Build() with nil levels succeeded, want error")
	}

	levels := &Levels{Width: 4, Height: 4, DB: make([]float32, 16)}
	bad := opts
	bad.RangeDB = 0
	if _, err := Build(levels, bad); err == nil {
		t.Error("Build() with an invalid range succeeded, want error")
	}

	mismatched := &Levels{Width: 8, Height: 8, DB: make([]float32, 64)}
	if _, err := Build(mismatched, opts); err == nil {
		t.Error("Build() with mismatched dimensions succeeded, want error")
	}
}

func TestBuildMapsLevelsToPaletteEntries(t *testing.T) {
	opts := DefaultOptions()
	opts.Width, opts.Height, opts.RangeDB = 3, 1, 120
	opts.Gridlines = nil

	levels := &Levels{
		Width:  3,
		Height: 1,
		DB: []float32{
			float32(math.Inf(-1)), // silence
			-60,                   // halfway up the range
			0,                     // full scale
		},
	}

	img, err := Build(levels, opts)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := img.Bounds(); got.Dx() != 3 || got.Dy() != 1 {
		t.Fatalf("bounds = %v, want 3x1", got)
	}

	want := []uint8{0, 128, 255}
	for x, w := range want {
		if got := img.ColorIndexAt(x, 0); got != w {
			t.Errorf("pixel %d = %d, want %d", x, got, w)
		}
	}
}

func TestBuildUsesTheChosenPalette(t *testing.T) {
	opts := DefaultOptions()
	opts.Width, opts.Height = 1, 1
	opts.Palette = PaletteGray
	opts.Gridlines = nil

	img, err := Build(&Levels{Width: 1, Height: 1, DB: []float32{0}}, opts)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	gray, err := NewPalette(PaletteGray)
	if err != nil {
		t.Fatalf("NewPalette() error = %v", err)
	}
	if img.Palette[255] != gray.Colors()[255] {
		t.Errorf("palette entry = %+v, want %+v", img.Palette[255], gray.Colors()[255])
	}
}

func TestBuildRespectsTheFloor(t *testing.T) {
	// A narrower range lifts quiet content into view, which is exactly why the
	// setting is exposed rather than fixed.
	levels := &Levels{Width: 1, Height: 1, DB: []float32{-90}}

	opts := DefaultOptions()
	opts.Width, opts.Height, opts.Gridlines = 1, 1, nil

	opts.RangeDB = 120
	wide, err := Build(levels, opts)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	opts.RangeDB = 100
	narrow, err := Build(levels, opts)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if narrow.ColorIndexAt(0, 0) >= wide.ColorIndexAt(0, 0) {
		t.Errorf("narrowing the range gave %d, wider gave %d, want the narrower to be darker",
			narrow.ColorIndexAt(0, 0), wide.ColorIndexAt(0, 0))
	}
}

func TestEncodeProducesAReadablePNG(t *testing.T) {
	opts := DefaultOptions()
	opts.Width, opts.Height = 16, 8
	opts.Gridlines = nil

	img, err := Build(&Levels{Width: 16, Height: 8, DB: make([]float32, 128)}, opts)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	var buf bytes.Buffer
	if err := Encode(&buf, img); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	if got := decoded.Bounds(); got.Dx() != 16 || got.Dy() != 8 {
		t.Errorf("decoded bounds = %v, want 16x8", got)
	}
	if _, ok := decoded.(*image.Paletted); !ok {
		t.Errorf("decoded image is %T, want an indexed image", decoded)
	}
}

func TestEncodeRejectsNilImage(t *testing.T) {
	if err := Encode(&bytes.Buffer{}, nil); err == nil {
		t.Fatal("Encode() with a nil image succeeded, want error")
	}
}

// TestRenderPipeline runs the four stages in the order the pipeline uses them,
// confirming they compose.
func TestRenderPipeline(t *testing.T) {
	grid := testGrid([][]float32{{0, 0, 1}, {0, 0, 0.5}})

	opts := DefaultOptions()
	opts.Width, opts.Height, opts.MaxHz = 64, 32, 1000
	opts.Gridlines = []Gridline{{Hz: 500}}

	levels, err := Scale(grid, opts.Width, opts.Height, opts.MaxHz)
	if err != nil {
		t.Fatalf("Scale() error = %v", err)
	}
	img, err := Build(levels, opts)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if err := Annotate(img, opts); err != nil {
		t.Fatalf("Annotate() error = %v", err)
	}

	var buf bytes.Buffer
	if err := Encode(&buf, img); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("Encode() produced no output")
	}
}
