package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"flac-util/internal/inventory"
)

func TestPrintTables(t *testing.T) {
	var out strings.Builder
	tables := []*inventory.Table{
		viewTable("/music/A/One", map[string]float64{"01 a": 0.7, "02 b": 0.5}),
		viewTable("/music/A/Two", map[string]float64{"01 c": 0.6}),
	}

	if err := PrintTables(&out, tables, false); err != nil {
		t.Fatalf("PrintTables() error = %v", err)
	}
	got := out.String()

	// Everything is printed, since whatever is reading a pipe does its own
	// paging and a dropped release would go unnoticed.
	for _, want := range []string{"/music/A/One", "/music/A/Two", "01 a", "02 b", "01 c"} {
		if !strings.Contains(got, want) {
			t.Errorf("the output does not mention %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "track") {
		t.Errorf("no column headers:\n%s", got)
	}
}

func TestPrintTablesWithNothingToShow(t *testing.T) {
	var out strings.Builder
	if err := PrintTables(&out, nil, false); err != nil {
		t.Fatalf("PrintTables() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "no tracks found") {
		t.Errorf("PrintTables() = %q", got)
	}
}

func TestPrintTablesReportsAWriteFailure(t *testing.T) {
	tables := []*inventory.Table{viewTable("/music/A/One", map[string]float64{"01 a": 0.7})}

	err := PrintTables(failingWriter{}, tables, false)
	if !errors.Is(err, errWriteFailed) {
		t.Errorf("PrintTables() error = %v, want %v", err, errWriteFailed)
	}
}

// errWriteFailed stands in for a closed pipe.
var errWriteFailed = errors.New("write failed")

// failingWriter fails every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWriteFailed }

func TestPrintTablesStripsColourForAPipe(t *testing.T) {
	// Lipgloss v2 renders colour whatever the destination is, so a listing
	// redirected to a file would otherwise arrive full of escape sequences.
	tables := []*inventory.Table{viewTable("/music/A/One", map[string]float64{"01 a": 0.7})}

	var plain, coloured strings.Builder
	if err := PrintTables(&plain, tables, false); err != nil {
		t.Fatalf("PrintTables() error = %v", err)
	}
	if err := PrintTables(&coloured, tables, true); err != nil {
		t.Fatalf("PrintTables() error = %v", err)
	}

	if strings.Contains(plain.String(), "\x1b[") {
		t.Errorf("the piped output carries escape sequences:\n%q", plain.String())
	}
	if !strings.Contains(coloured.String(), "\x1b[") {
		t.Error("the terminal output carries no colour")
	}
	if got, want := ansi.Strip(coloured.String()), plain.String(); got != want {
		t.Errorf("the two differ by more than colour:\n%q\n%q", got, want)
	}
}
