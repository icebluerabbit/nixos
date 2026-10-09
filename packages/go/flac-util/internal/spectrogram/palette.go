package spectrogram

import (
	"fmt"
	"image/color"
	"math"
	"slices"
)

// Palette names accepted by NewPalette.
const (
	PaletteSpectrum = "spectrum"
	PaletteGray     = "gray"
)

// DefaultPalette is the palette used unless one is chosen explicitly.
//
// Colour rather than grey because the judgement being made is whether faint
// content is present at all, and a hue ramp separates levels near the floor
// far better than shades of grey do.
const DefaultPalette = PaletteSpectrum

// paletteSize is the number of entries, which is what lets the result be
// stored as an eight bit indexed image: the same file size as greyscale.
const paletteSize = 256

// PaletteNames returns the accepted palette names in a stable order.
func PaletteNames() []string {
	return []string{PaletteSpectrum, PaletteGray}
}

// Palette maps a level in decibels to one of a fixed set of colours.
type Palette struct {
	name   string
	colors color.Palette
}

// NewPalette returns the named palette.
func NewPalette(name string) (*Palette, error) {
	if !slices.Contains(PaletteNames(), name) {
		return nil, fmt.Errorf("spectrogram: unknown palette %q, want one of %v", name, PaletteNames())
	}

	colors := make(color.Palette, paletteSize)
	for i := range paletteSize {
		t := float64(i) / float64(paletteSize-1)
		if name == PaletteGray {
			level := uint8(math.Round(t * 255))
			colors[i] = color.RGBA{R: level, G: level, B: level, A: 0xff}
			continue
		}
		colors[i] = spectrumColor(t)
	}
	return &Palette{name: name, colors: colors}, nil
}

// Name returns the palette's name.
func (p *Palette) Name() string { return p.name }

// Colors returns the palette entries, for use when allocating an image.
func (p *Palette) Colors() color.Palette { return p.colors }

// Index maps a level in decibels to a palette entry.
//
// The floor is the quietest level that still shows: anything at or below it
// takes the darkest entry, and full scale takes the brightest. Choosing the
// floor is the single setting most able to change how a picture reads, which
// is why it is a parameter rather than a constant.
func (p *Palette) Index(db, floorDB float64) uint8 {
	if floorDB >= 0 {
		return 0
	}
	if math.IsInf(db, -1) || db <= floorDB {
		return 0
	}
	if db >= 0 {
		return paletteSize - 1
	}

	t := (db - floorDB) / -floorDB
	return uint8(math.Round(t * float64(paletteSize-1)))
}

// spectrumColor returns the colour at position t along a dark blue, green,
// yellow, white ramp, which is the convention spectrogram viewers use.
func spectrumColor(t float64) color.RGBA {
	stops := []struct {
		at      float64
		r, g, b float64
	}{
		{0.00, 0, 0, 0},
		{0.25, 20, 20, 130},
		{0.50, 20, 160, 90},
		{0.75, 250, 220, 60},
		{1.00, 255, 255, 255},
	}

	t = min(max(t, 0), 1)
	for i := 1; i < len(stops); i++ {
		if t > stops[i].at {
			continue
		}
		lo, hi := stops[i-1], stops[i]
		span := hi.at - lo.at
		f := 0.0
		if span > 0 {
			f = (t - lo.at) / span
		}
		return color.RGBA{
			R: lerp(lo.r, hi.r, f),
			G: lerp(lo.g, hi.g, f),
			B: lerp(lo.b, hi.b, f),
			A: 0xff,
		}
	}
	return color.RGBA{R: 255, G: 255, B: 255, A: 0xff}
}

// lerp interpolates between two channel values.
func lerp(a, b, f float64) uint8 {
	return uint8(math.Round(min(max(a+(b-a)*f, 0), 255)))
}
