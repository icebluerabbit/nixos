package spectrogram

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Gridline is a labelled horizontal rule drawn across the picture at a fixed
// frequency.
type Gridline struct {
	// Hz is where the line sits.
	Hz float64
	// Label is the text drawn beside it. An empty label draws the frequency in
	// kilohertz.
	Label string
}

// Text returns the label to draw for the line.
func (g Gridline) Text() string {
	if g.Label != "" {
		return g.Label
	}
	return strconv.FormatFloat(g.Hz/1000, 'g', -1, 64) + "k"
}

// DefaultGridlines returns the frequencies at which common lossy encoders stop
// producing output.
//
// They are drawn as a ruler and nothing more. Where a file's content ends
// relative to these lines, and whether it ends abruptly or tapers, is for the
// reader to weigh: plenty of honest recordings are quiet at the top.
func DefaultGridlines() []Gridline {
	return []Gridline{
		{Hz: 16000},
		{Hz: 18500},
		{Hz: 19000},
		{Hz: 19500},
		{Hz: 20000},
		{Hz: 20500},
		{Hz: 22050},
	}
}

// ParseGridlines reads a comma separated list of frequencies in hertz, each
// optionally followed by a colon and a label.
func ParseGridlines(spec string) ([]Gridline, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}

	var out []Gridline
	for _, field := range strings.Split(spec, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}

		text := field
		var label string
		if name, rest, found := strings.Cut(field, ":"); found {
			text, label = strings.TrimSpace(name), strings.TrimSpace(rest)
		}

		hz, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, fmt.Errorf("spectrogram: gridline %q: %w", field, err)
		}
		out = append(out, Gridline{Hz: hz, Label: label})
	}
	return out, nil
}

// validateGridlines reports whether every line can be drawn.
func validateGridlines(lines []Gridline, maxHz float64) error {
	for _, line := range lines {
		if line.Hz <= 0 || math.IsNaN(line.Hz) {
			return fmt.Errorf("spectrogram: gridline frequency must be positive, got %v", line.Hz)
		}
		if line.Hz > maxHz {
			return fmt.Errorf("spectrogram: gridline at %v Hz is above the ceiling of %v Hz", line.Hz, maxHz)
		}
	}
	return nil
}

// RowFor returns the pixel row a frequency falls on, given the geometry.
func RowFor(hz, maxHz float64, height int) int {
	if maxHz <= 0 || height < 1 {
		return 0
	}
	row := int(math.Round(float64(height) * (1 - hz/maxHz)))
	return min(max(row, 0), height-1)
}

// Annotate draws the gridlines and their labels onto an already coloured
// image.
//
// Lines are drawn dashed and in the palette's brightest entry so that they
// read against the dark background of a spectrogram without hiding content
// underneath them.
func Annotate(img *image.Paletted, opts Options) error {
	if img == nil {
		return fmt.Errorf("spectrogram: image is nil")
	}
	if err := opts.Validate(); err != nil {
		return err
	}
	if len(opts.Gridlines) == 0 {
		return nil
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	ink := uint8(len(img.Palette) - 1)

	for _, line := range opts.Gridlines {
		y := RowFor(line.Hz, opts.MaxHz, height)
		// A dashed rule leaves the content beneath it legible.
		for x := 0; x < width; x += 8 {
			for dx := range 4 {
				if x+dx < width {
					img.SetColorIndex(x+dx, y, ink)
				}
			}
		}
		drawLabel(img, line.Text(), y, ink)
	}
	return nil
}

// drawLabel writes a line's text just above the rule, inset from the left.
func drawLabel(img *image.Paletted, text string, y int, ink uint8) {
	const (
		insetX     = 4
		aboveLine  = 3
		fontHeight = 13
	)

	baseline := y - aboveLine
	if baseline < fontHeight {
		baseline = y + fontHeight
	}
	if baseline >= img.Bounds().Dy() {
		return
	}

	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(img.Palette[ink].(color.RGBA)),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(insetX, baseline),
	}
	drawer.DrawString(text)
}
