package spectrogram

import (
	"image/color"
	"math"
	"testing"
)

func TestPaletteNames(t *testing.T) {
	want := []string{"spectrum", "gray"}
	got := PaletteNames()
	if len(got) != len(want) {
		t.Fatalf("PaletteNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("PaletteNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNewPaletteRejectsUnknownName(t *testing.T) {
	if _, err := NewPalette("viridis"); err == nil {
		t.Fatal("NewPalette() with an unknown name succeeded, want error")
	}
}

func TestNewPaletteShape(t *testing.T) {
	for _, name := range PaletteNames() {
		t.Run(name, func(t *testing.T) {
			p, err := NewPalette(name)
			if err != nil {
				t.Fatalf("NewPalette() error = %v", err)
			}
			if p.Name() != name {
				t.Errorf("Name() = %q, want %q", p.Name(), name)
			}
			if got := len(p.Colors()); got != 256 {
				t.Fatalf("len(Colors()) = %d, want 256", got)
			}
			for i, c := range p.Colors() {
				if _, _, _, a := c.RGBA(); a != 0xffff {
					t.Fatalf("entry %d is not opaque, alpha = %d", i, a)
				}
			}
		})
	}
}

func TestPaletteDarkestEntryIsBlack(t *testing.T) {
	for _, name := range PaletteNames() {
		t.Run(name, func(t *testing.T) {
			p, err := NewPalette(name)
			if err != nil {
				t.Fatalf("NewPalette() error = %v", err)
			}
			r, g, b, _ := p.Colors()[0].RGBA()
			if r != 0 || g != 0 || b != 0 {
				t.Errorf("entry 0 = (%d,%d,%d), want black", r, g, b)
			}
		})
	}
}

func TestGrayPaletteIsGray(t *testing.T) {
	p, err := NewPalette(PaletteGray)
	if err != nil {
		t.Fatalf("NewPalette() error = %v", err)
	}
	for i, c := range p.Colors() {
		rgba, ok := c.(color.RGBA)
		if !ok {
			t.Fatalf("entry %d is %T, want color.RGBA", i, c)
		}
		if rgba.R != rgba.G || rgba.G != rgba.B {
			t.Fatalf("entry %d = %+v, want equal channels", i, rgba)
		}
	}
}

func TestPaletteIndexEndpoints(t *testing.T) {
	p, err := NewPalette(DefaultPalette)
	if err != nil {
		t.Fatalf("NewPalette() error = %v", err)
	}

	tests := []struct {
		name  string
		db    float64
		floor float64
		want  uint8
	}{
		{"silence", math.Inf(-1), -120, 0},
		{"at the floor", -120, -120, 0},
		{"below the floor", -200, -120, 0},
		{"full scale", 0, -120, 255},
		{"above full scale", 6, -120, 255},
		{"halfway", -60, -120, 128},
		{"a quarter up", -90, -120, 64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.Index(tt.db, tt.floor); got != tt.want {
				t.Errorf("Index(%v, %v) = %d, want %d", tt.db, tt.floor, got, tt.want)
			}
		})
	}
}

func TestPaletteIndexRejectsNonNegativeFloor(t *testing.T) {
	p, err := NewPalette(DefaultPalette)
	if err != nil {
		t.Fatalf("NewPalette() error = %v", err)
	}
	for _, floor := range []float64{0, 10} {
		if got := p.Index(-6, floor); got != 0 {
			t.Errorf("Index(-6, %v) = %d, want 0", floor, got)
		}
	}
}

func TestPaletteIndexIsMonotonic(t *testing.T) {
	p, err := NewPalette(DefaultPalette)
	if err != nil {
		t.Fatalf("NewPalette() error = %v", err)
	}

	previous := p.Index(-120, -120)
	for db := -119.0; db <= 0; db++ {
		got := p.Index(db, -120)
		if got < previous {
			t.Fatalf("Index(%v) = %d, which is below the previous %d", db, got, previous)
		}
		previous = got
	}
}

func TestDefaultPaletteIsSpectrum(t *testing.T) {
	if DefaultPalette != PaletteSpectrum {
		t.Errorf("DefaultPalette = %q, want %q", DefaultPalette, PaletteSpectrum)
	}
}

func TestSpectrumColorIsClamped(t *testing.T) {
	for _, tt := range []struct {
		t    float64
		want color.RGBA
	}{
		{-1, color.RGBA{A: 0xff}},
		{0, color.RGBA{A: 0xff}},
		{1, color.RGBA{R: 255, G: 255, B: 255, A: 0xff}},
		{2, color.RGBA{R: 255, G: 255, B: 255, A: 0xff}},
	} {
		if got := spectrumColor(tt.t); got != tt.want {
			t.Errorf("spectrumColor(%v) = %+v, want %+v", tt.t, got, tt.want)
		}
	}
}

func TestLerp(t *testing.T) {
	tests := []struct {
		name    string
		a, b, f float64
		want    uint8
	}{
		{"start", 0, 100, 0, 0},
		{"end", 0, 100, 1, 100},
		{"middle", 0, 100, 0.5, 50},
		{"clamped low", -50, -10, 0, 0},
		{"clamped high", 200, 400, 1, 255},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lerp(tt.a, tt.b, tt.f); got != tt.want {
				t.Errorf("lerp(%v,%v,%v) = %d, want %d", tt.a, tt.b, tt.f, got, tt.want)
			}
		})
	}
}
