package spectrogram

import (
	"fmt"
	"image"
	"image/png"
	"io"
)

// Default image settings.
//
// The frequency ceiling is fixed rather than taken from the file's own sample
// rate so that a given frequency always lands on the same row, in every image,
// whatever the file. That is what lets a reader learn the picture once instead
// of recalculating the scale for every track.
const (
	DefaultWidth   = 1920
	DefaultHeight  = 1080
	DefaultMaxHz   = 24000
	DefaultRangeDB = 120
)

// Options configures rendering.
type Options struct {
	// Width and Height are the pixel dimensions of the image.
	Width  int
	Height int
	// MaxHz is the frequency at the top edge.
	MaxHz float64
	// RangeDB is how far below full scale still shows, as a positive number of
	// decibels. The default matches the reference command line tool, so that
	// images produced here look like the ones the community trades in.
	RangeDB float64
	// Palette names the colour ramp.
	Palette string
	// Gridlines are the frequencies to mark. Nil draws none.
	Gridlines []Gridline
}

// DefaultOptions returns the settings used unless overridden.
func DefaultOptions() Options {
	return Options{
		Width:     DefaultWidth,
		Height:    DefaultHeight,
		MaxHz:     DefaultMaxHz,
		RangeDB:   DefaultRangeDB,
		Palette:   DefaultPalette,
		Gridlines: DefaultGridlines(),
	}
}

// Validate reports whether the options are usable.
func (o Options) Validate() error {
	switch {
	case o.Width < 1 || o.Height < 1:
		return fmt.Errorf("spectrogram: size must be positive, got %dx%d", o.Width, o.Height)
	case o.MaxHz <= 0:
		return fmt.Errorf("spectrogram: maximum frequency must be positive, got %v", o.MaxHz)
	case o.RangeDB <= 0:
		return fmt.Errorf("spectrogram: range must be positive, got %v", o.RangeDB)
	}
	if _, err := NewPalette(o.Palette); err != nil {
		return err
	}
	return validateGridlines(o.Gridlines, o.MaxHz)
}

// FloorDB returns the quietest level that still shows, as a negative number.
func (o Options) FloorDB() float64 { return -o.RangeDB }

// Build colours a level table into an indexed image.
func Build(levels *Levels, opts Options) (*image.Paletted, error) {
	if levels == nil {
		return nil, fmt.Errorf("spectrogram: levels are nil")
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	if levels.Width != opts.Width || levels.Height != opts.Height {
		return nil, fmt.Errorf("spectrogram: levels are %dx%d, options ask for %dx%d",
			levels.Width, levels.Height, opts.Width, opts.Height)
	}

	palette, err := NewPalette(opts.Palette)
	if err != nil {
		return nil, err
	}

	img := image.NewPaletted(image.Rect(0, 0, opts.Width, opts.Height), palette.Colors())
	floor := opts.FloorDB()
	for y := range opts.Height {
		row := img.Pix[y*img.Stride : y*img.Stride+opts.Width]
		for x := range opts.Width {
			row[x] = palette.Index(levels.At(x, y), floor)
		}
	}
	return img, nil
}

// Encode writes the image as a PNG.
func Encode(w io.Writer, img image.Image) error {
	if img == nil {
		return fmt.Errorf("spectrogram: image is nil")
	}
	encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
	return encoder.Encode(w, img)
}
