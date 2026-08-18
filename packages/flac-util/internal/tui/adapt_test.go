package tui

import (
	"testing"

	"flac-util/internal/inventory"
)

func TestInventoryColumnsCarryEveryFlag(t *testing.T) {
	// The measured table decides what each column is; losing a flag here would
	// silently unalign a column of numbers or unbold the one that names a row.
	for i, got := range inventoryColumns(inventory.Columns()) {
		want := inventory.Columns()[i]
		if got.Title != want.Title || got.Width != want.Width ||
			got.Numeric != want.Numeric || got.Primary != want.Primary {
			t.Errorf("column %d = %+v, want it to match %+v", i, got, want)
		}
	}
}

func TestBandStyle(t *testing.T) {
	theme := NewTheme()

	tests := []struct {
		name string
		cell inventory.Cell
		want string
	}{
		{"unbanded", inventory.Cell{Band: inventory.NoBand}, theme.Label.Render("x")},
		{"lowest band", inventory.Cell{Band: 0, Palette: inventory.PaletteCutoff}, theme.Bands[0].Render("x")},
		{
			name: "highest band",
			cell: inventory.Cell{Band: len(theme.Bands) - 1, Palette: inventory.PaletteCutoff},
			want: theme.Bands[len(theme.Bands)-1].Render("x"),
		},
		{"attention wins", inventory.Cell{Band: 0, Attention: true}, theme.Bad.Render("x")},
		{"band past the ramp", inventory.Cell{Band: len(theme.Bands), Palette: inventory.PaletteCutoff}, theme.Label.Render("x")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bandStyle(theme, tt.cell).Render("x"); got != tt.want {
				t.Errorf("bandStyle() rendered %q, want %q", got, tt.want)
			}
		})
	}
}

func TestThemeHasAColourForEveryBand(t *testing.T) {
	// A band with no colour falls back to the unbanded style, which silently
	// merges two rows of RED's table into one on screen.
	if got, want := len(NewTheme().Bands), inventory.BandCount(); got < want {
		t.Errorf("the theme has %d band colours for %d bands", got, want)
	}
}

func TestInventoryRowsCarryTextAndColour(t *testing.T) {
	theme := NewTheme()
	rows := [][]inventory.Cell{{
		{Text: "01 - a", Band: inventory.NoBand},
		{Text: "16.0k", Band: 0, Palette: inventory.PaletteCutoff},
		{Text: "MISMATCH", Band: inventory.NoBand, Attention: true},
	}}

	got := inventoryRows(theme, rows)
	if len(got) != 1 || len(got[0]) != 3 {
		t.Fatalf("converted to %d rows", len(got))
	}
	for j, want := range []string{"01 - a", "16.0k", "MISMATCH"} {
		if got[0][j].Text != want {
			t.Errorf("cell %d = %q, want %q", j, got[0][j].Text, want)
		}
	}
	if got[0][1].Style.Render("x") != theme.Bands[0].Render("x") {
		t.Error("the banded cell lost its colour")
	}
	if got[0][2].Style.Render("x") != theme.Bad.Render("x") {
		t.Error("the cell demanding attention lost its colour")
	}
}

// TestBandStyleMutesAQuietValue checks a value worth stating but not worth
// looking at recedes. A column of "ok" in the ordinary colour would compete
// with the one row that says something else.
func TestBandStyleMutesAQuietValue(t *testing.T) {
	theme := NewTheme()

	muted := inventory.Cell{Band: inventory.NoBand, Muted: true}
	if got, want := bandStyle(theme, muted).Render("ok"), theme.Subtle.Render("ok"); got != want {
		t.Errorf("a muted cell rendered %q, want %q", got, want)
	}

	// Attention still wins: a problem is never quiet.
	loud := inventory.Cell{Band: inventory.NoBand, Muted: true, Attention: true}
	if got, want := bandStyle(theme, loud).Render("x"), theme.Bad.Render("x"); got != want {
		t.Errorf("a muted problem rendered %q, want %q", got, want)
	}
}
