// Package tui draws a run while it happens.
//
// It is the only package that touches the terminal. Everything it displays
// arrives through the progress package, so the collector neither knows nor
// cares whether anybody is watching.
package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

// Theme holds every style the display uses, so that no other file invents one.
type Theme struct {
	Title    lipgloss.Style
	Subtle   lipgloss.Style
	Label    lipgloss.Style
	Value    lipgloss.Style
	Good     lipgloss.Style
	Bad      lipgloss.Style
	Warn     lipgloss.Style
	Header   lipgloss.Style
	Footer   lipgloss.Style
	Selected lipgloss.Style
	Border   lipgloss.Style

	// BannerFrom and BannerTo are the ends of the gradient the banner is
	// painted across; BannerText is what is legible against either of them.
	BannerFrom color.Color
	BannerTo   color.Color
	BannerText color.Color

	// SelectedBackground is laid behind the row the cursor is on. It is a
	// colour rather than a style because it is applied to each cell over
	// whatever that cell is already coloured, so the band colours survive
	// being selected.
	SelectedBackground color.Color

	// Bands colour a cutoff by which breakpoint range it fell in, lowest
	// first. They are a ramp so that a column can be read down at a glance;
	// none of them says a value is acceptable or not.
	Bands []lipgloss.Style

	// Depths colour a bit depth by which of the usual depths it is. A separate
	// set from Bands, because a depth is one of a fixed handful of values and a
	// cutoff is a position in a range - sharing a ramp would make the two read
	// as the same kind of statement.
	Depths []lipgloss.Style
}

// Colours are given as light and dark pairs, chosen once the terminal has said
// which it is. Deciding per style rather than per palette is what keeps a
// single call site for the answer.
type pair struct{ light, dark string }

var (
	colorTitle  = pair{"#5A3E9B", "#C4B5FD"}
	colorSubtle = pair{"#6B7280", "#9CA3AF"}
	colorLabel  = pair{"#374151", "#D1D5DB"}
	colorValue  = pair{"#111827", "#F9FAFB"}
	colorGood   = pair{"#047857", "#6EE7B7"}
	colorBad    = pair{"#B91C1C", "#FCA5A5"}
	colorWarn   = pair{"#B45309", "#FCD34D"}
	colorBorder = pair{"#D1D5DB", "#4B5563"}
	colorSelect = pair{"#DBEAFE", "#1F2937"}
	colorBarA   = pair{"#5B21B6", "#4C1D95"}
	colorBarB   = pair{"#0E7490", "#155E75"}
	colorOnBar  = pair{"#F5F3FF", "#E0E7FF"}
)

// bandColors ramp from the lowest cutoff breakpoint to the highest. There are
// as many of them as there are breakpoints, and the display falls back to an
// unbanded style if it is ever handed a band beyond the end.
var bandColors = []pair{
	{"#7F1D1D", "#FCA5A5"},
	{"#9A3412", "#FDBA74"},
	{"#B45309", "#FCD34D"},
	{"#4D7C0F", "#D9F99D"},
	{"#15803D", "#86EFAC"},
	{"#0F766E", "#5EEAD4"},
	{"#1D4ED8", "#93C5FD"},
}

// NewTheme returns the default styles for a dark terminal.
//
// Dark is the assumption until the terminal answers, because the query is
// asynchronous and something has to be drawn in the meantime.
func NewTheme() Theme { return NewThemeFor(true) }

// NewThemeFor returns the styles for a terminal of the given background.
func NewThemeFor(isDark bool) Theme {
	pick := lipgloss.LightDark(isDark)
	of := func(p pair) color.Color { return pick(lipgloss.Color(p.light), lipgloss.Color(p.dark)) }

	bands := make([]lipgloss.Style, len(bandColors))
	for i, p := range bandColors {
		bands[i] = lipgloss.NewStyle().Foreground(of(p))
	}

	return Theme{
		Title:              lipgloss.NewStyle().Foreground(of(colorTitle)).Bold(true),
		Subtle:             lipgloss.NewStyle().Foreground(of(colorSubtle)),
		Label:              lipgloss.NewStyle().Foreground(of(colorLabel)),
		Value:              lipgloss.NewStyle().Foreground(of(colorValue)).Bold(true),
		Good:               lipgloss.NewStyle().Foreground(of(colorGood)),
		Bad:                lipgloss.NewStyle().Foreground(of(colorBad)),
		Warn:               lipgloss.NewStyle().Foreground(of(colorWarn)),
		Header:             lipgloss.NewStyle().Padding(0, 1),
		Footer:             lipgloss.NewStyle().Foreground(of(colorSubtle)).Padding(0, 1),
		Selected:           lipgloss.NewStyle().Foreground(of(colorValue)).Bold(true),
		Border:             lipgloss.NewStyle().Foreground(of(colorBorder)),
		Bands:              bands,
		Depths:             depthStyles(of),
		SelectedBackground: of(colorSelect),
		BannerFrom:         of(colorBarA),
		BannerTo:           of(colorBarB),
		BannerText:         of(colorOnBar),
	}
}

// bannerStep is how many cells share a colour.
//
// A gradient computed per cell needs an escape sequence per cell, which on a
// wide terminal is several kilobytes for one line, redrawn every time the
// clock ticks. Holding a colour for a few cells cuts that by the same factor
// and is indistinguishable at this width: the ramp spans the whole screen, so
// neighbouring cells differ by about one part in three hundred anyway.
const bannerStep = 4

// paintBanner lays text over a gradient.
//
// lipgloss will put a gradient behind a whole string, but only as a single
// background: the ramp has to be walked here so that each part of the line
// gets the colour belonging to its position. Everything before boldTo - the
// mark and the program's name - is set in bold, which is what makes the banner
// read as a wordmark rather than as a coloured line.
func (t Theme) paintBanner(text string, boldTo int) string {
	if text == "" {
		return ""
	}

	width := runewidth.StringWidth(text)
	ramp := lipgloss.Blend1D(max(width/bannerStep, 1), t.BannerFrom, t.BannerTo)

	var b strings.Builder
	var run strings.Builder
	column, segment, bold := 0, 0, boldTo > 0

	flush := func() {
		if run.Len() == 0 {
			return
		}
		style := lipgloss.NewStyle().
			Foreground(t.BannerText).
			Background(ramp[min(segment, len(ramp)-1)]).
			Bold(bold)
		b.WriteString(style.Render(run.String()))
		run.Reset()
	}

	for _, r := range text {
		// A run ends where its colour or its weight does.
		if column/bannerStep != segment || (column >= boldTo) == bold {
			flush()
			segment, bold = column/bannerStep, column < boldTo
		}
		run.WriteRune(r)
		// Wide characters occupy two cells, so the ramp advances by what was
		// drawn rather than by how many runes went past.
		column += runewidth.RuneWidth(r)
	}
	flush()

	return b.String()
}

// depthColors name the usual bit depths, in the order Depths lists them.
//
// They are deliberately not a good-to-bad ramp: 8, 16, 20, 24 and 32 bits are
// five different things a file can be, and which of them is right depends on
// where it came from. The colours are here to make a column of them readable
// and to make a declared depth and a measured one visibly disagree.
var depthColors = []pair{
	{"#7C2D12", "#FDBA74"},
	{"#1D4ED8", "#93C5FD"},
	{"#6D28D9", "#C4B5FD"},
	{"#0F766E", "#5EEAD4"},
	{"#9D174D", "#F9A8D4"},
}

// depthStyles turns the depth colours into styles.
func depthStyles(of func(pair) color.Color) []lipgloss.Style {
	styles := make([]lipgloss.Style, len(depthColors))
	for i, p := range depthColors {
		styles[i] = lipgloss.NewStyle().Foreground(of(p))
	}
	return styles
}
