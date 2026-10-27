package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

func TestNewThemeRendersEveryStyle(t *testing.T) {
	theme := NewTheme()

	styles := map[string]func(...string) string{
		"Title":    theme.Title.Render,
		"Subtle":   theme.Subtle.Render,
		"Label":    theme.Label.Render,
		"Value":    theme.Value.Render,
		"Good":     theme.Good.Render,
		"Bad":      theme.Bad.Render,
		"Warn":     theme.Warn.Render,
		"Header":   theme.Header.Render,
		"Footer":   theme.Footer.Render,
		"Selected": theme.Selected.Render,
		"Border":   theme.Border.Render,
	}

	for name, render := range styles {
		t.Run(name, func(t *testing.T) {
			got := render("sample")
			if !strings.Contains(got, "sample") {
				t.Errorf("%s.Render() = %q, want it to contain the text", name, got)
			}
		})
	}
}

// TestThemeColoursAreAdaptive checks every colour carries both a light and a
// dark variant, since the display has no idea what terminal it lands in until
// the terminal answers.
func TestThemeColoursAreAdaptive(t *testing.T) {
	colours := map[string]pair{
		"title":  colorTitle,
		"subtle": colorSubtle,
		"label":  colorLabel,
		"value":  colorValue,
		"good":   colorGood,
		"bad":    colorBad,
		"warn":   colorWarn,
		"border": colorBorder,
	}
	for i, p := range bandColors {
		colours["band "+string(rune('0'+i))] = p
	}

	for name, p := range colours {
		t.Run(name, func(t *testing.T) {
			if p.light == "" || p.dark == "" {
				t.Errorf("colour %q = %+v, want both variants set", name, p)
			}
			if p.light == p.dark {
				t.Errorf("colour %q uses %q for both themes", name, p.light)
			}
		})
	}
}

// TestNewThemeForPicksTheRightVariant checks the light and dark themes are
// actually different, which is the whole reason the terminal is asked.
func TestNewThemeForPicksTheRightVariant(t *testing.T) {
	light, dark := NewThemeFor(false), NewThemeFor(true)

	if light.Title.Render("x") == dark.Title.Render("x") {
		t.Error("the light and dark themes render identically")
	}
	if got, want := light.Title.GetForeground(), lipgloss.Color(colorTitle.light); got != want {
		t.Errorf("the light theme's title is %v, want %v", got, want)
	}
	if got, want := dark.Title.GetForeground(), lipgloss.Color(colorTitle.dark); got != want {
		t.Errorf("the dark theme's title is %v, want %v", got, want)
	}
}

func TestPaintBannerFillsExactlyTheTextGiven(t *testing.T) {
	theme := NewTheme()
	text := " ♪  FLAC-UTIL  view      18 releases "

	got := theme.paintBanner(text, 4)
	if want := runewidth.StringWidth(text); lipgloss.Width(got) != want {
		t.Errorf("the banner is %d cells wide, want %d", lipgloss.Width(got), want)
	}
	if ansi.Strip(got) != text {
		t.Errorf("the banner reads %q, want %q", ansi.Strip(got), text)
	}
}

func TestPaintBannerOfNothing(t *testing.T) {
	if got := NewTheme().paintBanner("", 0); got != "" {
		t.Errorf("paintBanner(\"\") = %q, want empty", got)
	}
}

// TestPaintBannerRunsInSteps guards the reason the gradient is quantised. A
// colour per cell means an escape sequence per cell, which on a wide terminal
// is several kilobytes for one line, redrawn every time the clock ticks.
func TestPaintBannerRunsInSteps(t *testing.T) {
	theme := NewTheme()
	text := strings.Repeat("x", 200)

	got := theme.paintBanner(text, 0)
	if runs := strings.Count(got, "\x1b[0m"); runs > len(text)/bannerStep+2 {
		t.Errorf("the banner drew %d runs over %d cells, want about %d",
			runs, len(text), len(text)/bannerStep)
	}
	if lipgloss.Width(got) != len(text) {
		t.Errorf("width = %d, want %d", lipgloss.Width(got), len(text))
	}
}

// TestPaintBannerEndsTheBoldWhereTheNameEnds checks the wordmark and the rest
// of the bar are told apart by weight, which is what stops the banner reading
// as one undifferentiated coloured line.
func TestPaintBannerEndsTheBoldWhereTheNameEnds(t *testing.T) {
	theme := NewTheme()
	got := theme.paintBanner(" FLAC-UTIL  view ", len(" FLAC-UTIL "))

	before, after, found := strings.Cut(got, "view")
	if !found {
		t.Fatalf("the banner does not contain the subcommand: %q", got)
	}
	if !strings.Contains(before, "\x1b[1;") {
		t.Error("the name is not set in bold")
	}
	if strings.Contains(after, "\x1b[1;") {
		t.Error("the bold runs past the name")
	}
}

// TestPaintBannerCountsWideCharacters covers the ramp advancing by cells
// rather than by runes, without which a wide glyph would shift every colour
// after it and leave the bar a cell short.
func TestPaintBannerCountsWideCharacters(t *testing.T) {
	theme := NewTheme()
	text := "♪ 클락션 view"

	got := theme.paintBanner(text, 0)
	if want := runewidth.StringWidth(text); lipgloss.Width(got) != want {
		t.Errorf("width = %d, want %d", lipgloss.Width(got), want)
	}
}
