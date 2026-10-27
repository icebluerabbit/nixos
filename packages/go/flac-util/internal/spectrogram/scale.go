// Package spectrogram turns a measured time-by-frequency grid into a PNG
// image.
//
// Nothing here decides anything about the audio. It draws a picture and, at
// the reader's request, a ruler beside it; reading the picture is a job for a
// person.
package spectrogram

import (
	"fmt"
	"math"

	"homelab/flac-util/internal/measure"
)

// Levels is a width-by-height table of decibel values, laid out the way an
// image is: row zero is the top of the picture and holds the highest
// frequency, and column zero is the start of the track.
//
// It is the intermediate between a measured grid, whose shape is dictated by
// the transform, and an image, whose shape is dictated by the reader.
type Levels struct {
	Width  int
	Height int
	// DB is row-major, so the value at (x,y) lives at y*Width+x.
	DB []float32
	// MaxHz is the frequency at the top edge of the picture.
	MaxHz float64
}

// At returns the level at a pixel, in decibels relative to full scale.
func (l *Levels) At(x, y int) float64 {
	if x < 0 || y < 0 || x >= l.Width || y >= l.Height {
		return math.Inf(-1)
	}
	return float64(l.DB[y*l.Width+x])
}

// Scale resamples a measured grid onto a fixed pixel raster.
//
// Every output pixel takes the loudest bin it covers rather than their mean.
// A narrow band of quiet content is exactly what matters when reading the top
// of a spectrogram, and averaging is what would hide it.
func Scale(g *measure.Grid, width, height int, maxHz float64) (*Levels, error) {
	switch {
	case g == nil:
		return nil, fmt.Errorf("spectrogram: grid is nil")
	case width < 1 || height < 1:
		return nil, fmt.Errorf("spectrogram: size must be positive, got %dx%d", width, height)
	case maxHz <= 0:
		return nil, fmt.Errorf("spectrogram: maximum frequency must be positive, got %v", maxHz)
	case g.Columns() == 0:
		return nil, fmt.Errorf("spectrogram: grid has no columns")
	case g.BinHz() <= 0:
		return nil, fmt.Errorf("spectrogram: grid has no frequency scale")
	}

	out := &Levels{
		Width:  width,
		Height: height,
		DB:     make([]float32, width*height),
		MaxHz:  maxHz,
	}

	columns := g.Columns()
	binHz := g.BinHz()
	negInf := float32(math.Inf(-1))

	for y := range height {
		// Row zero is the top of the picture and therefore the highest
		// frequency, so the vertical axis runs backwards.
		lowHz := maxHz * float64(height-1-y) / float64(height)
		highHz := maxHz * float64(height-y) / float64(height)

		// A bin whose centre sits exactly on the boundary belongs to both
		// neighbouring rows, which costs a pixel of overlap and avoids losing
		// content at the very top of the picture.
		binLo := max(int(lowHz/binHz), 0)
		binHi := min(max(int(highHz/binHz)+1, binLo+1), g.Bins)

		for x := range width {
			colLo := x * columns / width
			colHi := max((x+1)*columns/width, colLo+1)
			colHi = min(colHi, columns)

			peak := float32(0)
			found := false
			for c := colLo; c < colHi; c++ {
				column := g.Power[c]
				for b := binLo; b < binHi && b < len(column); b++ {
					found = true
					peak = max(peak, column[b])
				}
			}

			value := negInf
			if found && peak > 0 {
				value = float32(measure.PowerDB(float64(peak)))
			}
			out.DB[y*width+x] = value
		}
	}
	return out, nil
}
