package tui

import (
	"strings"
	"testing"
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
// dark variant, since the display has no idea what terminal it lands in.
func TestThemeColoursAreAdaptive(t *testing.T) {
	colours := map[string]struct{ light, dark string }{
		"title":  {colorTitle.Light, colorTitle.Dark},
		"subtle": {colorSubtle.Light, colorSubtle.Dark},
		"label":  {colorLabel.Light, colorLabel.Dark},
		"value":  {colorValue.Light, colorValue.Dark},
		"good":   {colorGood.Light, colorGood.Dark},
		"bad":    {colorBad.Light, colorBad.Dark},
		"warn":   {colorWarn.Light, colorWarn.Dark},
	}

	for name, pair := range colours {
		t.Run(name, func(t *testing.T) {
			if pair.light == "" || pair.dark == "" {
				t.Errorf("colour %q = %+v, want both variants set", name, pair)
			}
			if pair.light == pair.dark {
				t.Errorf("colour %q uses %q for both themes", name, pair.light)
			}
		})
	}
}
