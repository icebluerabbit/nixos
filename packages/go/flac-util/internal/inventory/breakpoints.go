package inventory

// NoBand is the band of a cell no breakpoint applies to.
const NoBand = -1

// Breakpoint is one frequency a cutoff is banded at.
type Breakpoint struct {
	// Hz is the frequency, in hertz.
	Hz float64
	// Format is what a cutoff at this frequency is characteristic of.
	Format string
}

// CutoffBreakpoints are the frequencies a measured cutoff is banded at, and
// what RED's interview guide associates each one with.
//
// The table is theirs, not this tool's: https://interviewfor.red/en/spectrals.html
// Banding a value only says which row of that table it landed on. It does not
// say the track is a transcode, because a cutoff is one observation and the
// guide's own point is that the picture has to be read as a whole.
func CutoffBreakpoints() []Breakpoint {
	return []Breakpoint{
		{Hz: 16000, Format: "MP3 128 CBR"},
		{Hz: 18500, Format: "MP3 V2"},
		{Hz: 19000, Format: "MP3 192 CBR"},
		{Hz: 19500, Format: "MP3 V0"},
		{Hz: 20000, Format: "MP3 256 CBR"},
		{Hz: 20500, Format: "MP3 320 CBR"},
		{Hz: 22000, Format: "CD / lossless"},
	}
}

// band returns the index of the highest breakpoint a value reaches, or NoBand
// when it reaches none of them.
func band(value float64, breakpoints []Breakpoint) int {
	found := NoBand
	for i, bp := range breakpoints {
		if value >= bp.Hz {
			found = i
		}
	}
	return found
}

// BandCount is how many bands a value can fall in, which is the number of
// colours a display needs.
func BandCount() int { return len(CutoffBreakpoints()) }

// Depths are the bit depths a FLAC is found at, lowest first.
//
// The header allows 1 to 32 - five bits holding the value less one - but only
// these occur in practice. A depth outside the list is shown without a colour
// rather than forced into a neighbouring one.
//
// RED and OPS, which both run Gazelle, are coarser than this. They have two
// encodings for FLAC and file everything under 24 bits as plain Lossless:
//
//	if ($Properties['Bitrate'] == 'Lossless')       { ... torrents_lossless   }
//	if ($Properties['Bitrate'] == '24bit Lossless') { ... torrents_lossless24 }
//
// Splitting them out shows a distinction the tracker throws away.
func Depths() []int { return []int{8, 16, 20, 24, 32} }

// DepthOf returns which of Depths a bit depth is, or NoBand for a depth that
// is not one of them.
func DepthOf(bits int) int {
	for i, depth := range Depths() {
		if bits == depth {
			return i
		}
	}
	return NoBand
}

// Palette names which set of buckets a banded cell is an index into.
//
// Two kinds of column are bucketed and they are bucketed against different
// things: a depth is a value from a fixed list, a cutoff is a place in a range.
// Sharing one ramp between them would give the two meanings the same colours.
type Palette int

// The bucket sets.
const (
	// PaletteNone is a value in no bucket.
	PaletteNone Palette = iota
	// PaletteDepth indexes Depths.
	PaletteDepth
	// PaletteCutoff indexes CutoffBreakpoints.
	PaletteCutoff
)
