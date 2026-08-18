package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"flac-util/internal/inventory"
)

// PrintTables writes every release out one after another.
//
// This is what a pipe or a log gets. Nothing is paged and nothing is dropped,
// since whatever is reading has its own way of looking back.
//
// Colour is stripped unless the caller says the destination can show it.
// Lipgloss renders every style in full and leaves it to the program to match
// the terminal, so there is nothing between this and a file full of escape
// sequences except this call.
func PrintTables(w io.Writer, tables []*inventory.Table, colour bool) error {
	theme := NewTheme()

	if len(tables) == 0 {
		_, err := fmt.Fprintln(w, "no tracks found")
		return err
	}

	var b strings.Builder
	for i, t := range tables {
		if i > 0 {
			b.WriteString("\n")
		}

		rows := inventoryRows(theme, t.Rows)
		styler := newTableStyler(theme, inventoryColumns(t.Columns), rows)
		b.WriteString(theme.Title.Render(t.Group.Label()))
		b.WriteString("\n")
		b.WriteString(theme.Subtle.Render(t.Group.Dir))
		b.WriteString("\n")
		b.WriteString(theme.Label.Bold(true).Render(styler.plainHeader()))
		b.WriteString("\n")

		for _, cells := range rows {
			b.WriteString(styler.plainRow(cells))
			b.WriteString("\n")
		}
	}

	out := b.String()
	if !colour {
		out = ansi.Strip(out)
	}

	_, err := io.WriteString(w, out)
	return err
}
