package tui

import (
	"charm.land/lipgloss/v2"

	"flac-util/internal/inventory"
)

// inventoryColumns converts the measured table's columns into the ones this
// package draws.
func inventoryColumns(columns []inventory.Column) []column {
	out := make([]column, len(columns))
	for i, c := range columns {
		out[i] = column{
			Title:   c.Title,
			Width:   c.Width,
			Numeric: c.Numeric,
			Primary: c.Primary,
		}
	}
	return out
}

// inventoryRows converts measured cells into drawable ones, deciding here what
// colour each value is shown in.
//
// This is the only place that turns a measurement into a colour. A banded value
// is coloured by which breakpoint range it fell in and nothing else: the colour
// says where the value sits, not whether it is acceptable.
func inventoryRows(theme Theme, rows [][]inventory.Cell) [][]cell {
	out := make([][]cell, len(rows))
	for i, row := range rows {
		converted := make([]cell, len(row))
		for j, c := range row {
			converted[j] = styled(c.Text, bandStyle(theme, c))
		}
		out[i] = converted
	}
	return out
}

// bandStyle returns the colour a measured cell is shown in.
//
// Which set of colours it comes from is the cell's own business: a depth and a
// cutoff are both bucketed, but into different things, and a shared ramp would
// make them read as the same kind of statement.
func bandStyle(theme Theme, c inventory.Cell) lipgloss.Style {
	if c.Attention {
		return theme.Bad
	}
	if c.Muted {
		return theme.Subtle
	}

	var palette []lipgloss.Style
	switch c.Palette {
	case inventory.PaletteCutoff:
		palette = theme.Bands
	case inventory.PaletteDepth:
		palette = theme.Depths
	case inventory.PaletteNone:
		return theme.Label
	}

	if c.Band >= 0 && c.Band < len(palette) {
		return palette[c.Band]
	}
	return theme.Label
}
