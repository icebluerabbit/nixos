package inventory

import (
	"fmt"
	"strconv"

	"homelab/flac-util/internal/report"
)

// Absent is shown in place of a value that could not be read.
const Absent = "—"

// Cell is one value in the table.
type Cell struct {
	// Text is what is shown.
	Text string
	// Value is the number Text renders, for columns that sort by size.
	Value float64
	// HasValue says whether Value means anything.
	HasValue bool
	// Band is which bucket the value fell in, or NoBand when the column has
	// no buckets or the value reached none of them. Palette says which set of
	// buckets it is an index into.
	Band    int
	Palette Palette
	// Attention marks a value that means something on its own: a checksum
	// that does not match, or a report that no longer describes its track.
	Attention bool
	// Muted marks a value worth stating but not worth looking at, so that a
	// column of them recedes and the one entry that is not muted stands out.
	Muted bool
}

// Options configures how entries are turned into rows.
type Options struct {
	// CutoffFloorDB is how far below the loudest bin still counts as content.
	// Zero selects DefaultCutoffFloorDB.
	CutoffFloorDB float64
}

// Column is one column of the table.
type Column struct {
	// Title is the header.
	Title string
	// Numeric says the column sorts by size rather than by text.
	Numeric bool
	// Primary says this column names the row rather than measuring it, so a
	// display can set it apart from the values beside it.
	Primary bool
	// Width is the visible width the values need, ignoring any styling.
	Width int
	// Cell extracts the value for one entry.
	Cell func(Entry, Options) Cell
}

// Columns returns the table's columns, in order.
//
// This is a list for deciding which tracks are worth opening, not a place to
// read a track from. Everything it holds either says what a file claims to be
// or says that the claim is worth checking; the rest of what was measured
// belongs where there is room for it.
func Columns() []Column {
	return []Column{
		{Title: "track", Width: 34, Primary: true, Cell: trackCell},
		{Title: "rate", Width: 5, Numeric: true, Cell: rateCell},
		{Title: "cutoff", Width: 7, Numeric: true, Cell: cutoffCell},
		{Title: "depth", Width: 6, Numeric: true, Cell: depthCell},
		{Title: "effective", Width: 9, Numeric: true, Cell: effectiveDepthCell},
		{Title: "status", Width: 10, Cell: statusCell},
	}
}

// plain returns a cell with no breakpoint applied to it.
func plain(text string) Cell { return Cell{Text: text, Band: NoBand} }

// number returns a cell holding a value that sorts by size.
func number(text string, value float64) Cell {
	return Cell{Text: text, Value: value, HasValue: true, Band: NoBand}
}

// absentCell is the value shown when there is nothing to show.
func absentCell() Cell { return plain(Absent) }

func trackCell(e Entry, _ Options) Cell {
	return plain(e.Track())
}

// rateCell is the sample rate in kilohertz.
//
// It has a column of its own because the tracker's encoding throws it away:
// 44.1 kHz and 192 kHz are both filed as 24bit Lossless at 24 bits, and the
// difference between them is not visible anywhere else.
func rateCell(e Entry, _ Options) Cell {
	if e.Report == nil {
		return absentCell()
	}
	rate := e.Report.Stream.SampleRate
	return number(
		strconv.FormatFloat(float64(rate)/1000, 'f', -1, 64),
		float64(rate),
	)
}

// depthCell is the bit depth the header declares.
func depthCell(e Entry, _ Options) Cell {
	if e.Report == nil {
		return absentCell()
	}
	return depthOf(e.Report.Stream.BitsPerSample)
}

// effectiveDepthCell is how many bits the audio turns out to use.
//
// Against the declared depth this is the other half of the pair the cutoff
// makes with the sample rate: a 16-bit file padded out to 24 declares 24 and
// measures 16, the same shape of lie as a CD resampled to 96 kHz.
func effectiveDepthCell(e Entry, _ Options) Cell {
	if e.Report == nil {
		return absentCell()
	}
	return depthOf(e.Report.Resolution.EffectiveBits)
}

// depthOf renders a bit depth, coloured by which of the usual depths it is.
func depthOf(bits int) Cell {
	if bits <= 0 {
		return absentCell()
	}

	cell := number(strconv.Itoa(bits)+"bit", float64(bits))
	cell.Band, cell.Palette = DepthOf(bits), PaletteDepth
	return cell
}

// cutoffCell is where the content stops.
//
// Only a cutoff below the ceiling is banded. At the ceiling there is no
// cutoff to name - the content runs to the highest frequency the format can
// hold - and banding it would file a 96 kHz track under a row of RED's table
// that describes what a CD holds.
func cutoffCell(e Entry, opts Options) Cell {
	hz, ratio, ok := Reach(e.Report, opts.CutoffFloorDB)
	if !ok {
		return absentCell()
	}

	// The rate beside it is the ceiling, so the two together already say how
	// far up the spectrum the content reaches; a percentage would be those
	// two divided.
	cell := number(fmt.Sprintf("%.1fk", hz/1000), ratio)
	if ratio < 1 {
		cell.Band, cell.Palette = band(hz, CutoffBreakpoints()), PaletteCutoff
	}
	return cell
}

// statusCell is every reason not to trust the row, in one column.
//
// They are one column because they amount to the same warning and because
// they are all rare: a row with nothing wrong says so quietly, so anything
// else written here is by itself a reason to look.
//
// The order is the order the doubts override each other. A report that does
// not describe the file makes what it says about the audio beside the point,
// and audio that stopped short makes its checksum a foregone conclusion.
func statusCell(e Entry, _ Options) Cell {
	switch {
	case e.Err != nil:
		return flag("unreadable")
	case e.State != report.StateCurrent:
		return flag(string(e.State))
	case e.Report == nil:
		return flag(string(report.StateMissing))
	case !e.Report.Decode.Complete:
		return flag("partial")
	case !e.Report.Integrity.StoredMD5Present:
		return flag("no md5")
	case !e.Report.Integrity.Match:
		return flag("MISMATCH")
	}

	// Stated rather than left blank, so the column is visibly answering rather
	// than empty - but muted, so that a library of them stays quiet and the
	// one row that is not muted still catches the eye.
	cell := plain("ok")
	cell.Muted = true
	return cell
}

// flag returns a cell that demands attention.
func flag(text string) Cell {
	return Cell{Text: text, Band: NoBand, Attention: true}
}

// Table is one group laid out for reading.
type Table struct {
	// Group is the release the rows describe.
	Group Group
	// Columns are the table's columns.
	Columns []Column
	// Rows holds one row per entry, in the same order as Group.Entries.
	Rows [][]Cell
}

// NewTable turns a group's entries into rows.
func NewTable(g Group, opts Options) *Table {
	columns := Columns()

	rows := make([][]Cell, len(g.Entries))
	for i, entry := range g.Entries {
		row := make([]Cell, len(columns))
		for j, column := range columns {
			row[j] = column.Cell(entry, opts)
		}
		rows[i] = row
	}
	return &Table{Group: g, Columns: columns, Rows: rows}
}
