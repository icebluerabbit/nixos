package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// keyPress builds the message a terminal sends for a keystroke, named the way
// a key binding names it.
//
// Bubble Tea describes a key by code and modifiers rather than by the string
// it prints, so a test that wants to press "ctrl+c" has to say so in those
// terms. Doing it once here keeps that translation out of every test.
func keyPress(name string) tea.KeyPressMsg {
	k := tea.Key{}

	for {
		modifier, rest, found := strings.Cut(name, "+")
		if !found || rest == "" {
			break
		}
		switch modifier {
		case "ctrl":
			k.Mod |= tea.ModCtrl
		case "alt":
			k.Mod |= tea.ModAlt
		case "shift":
			k.Mod |= tea.ModShift
		default:
			// Not a modifier after all: the key itself contains a plus.
			return namedKey(k, name)
		}
		name = rest
	}
	return namedKey(k, name)
}

// namedKey fills in the code for a key given by name.
func namedKey(k tea.Key, name string) tea.KeyPressMsg {
	switch name {
	case "tab":
		k.Code = tea.KeyTab
	case "esc":
		k.Code = tea.KeyEscape
	case "enter":
		k.Code = tea.KeyEnter
	case "up":
		k.Code = tea.KeyUp
	case "down":
		k.Code = tea.KeyDown
	case "left":
		k.Code = tea.KeyLeft
	case "right":
		k.Code = tea.KeyRight
	default:
		k.Code = []rune(name)[0]
		// Text is what the key printed, which only applies when nothing was
		// held down with it.
		if k.Mod == 0 {
			k.Text = name
		}
	}
	return tea.KeyPressMsg(k)
}

// plain strips the styling from rendered output.
//
// Styles are applied in runs, so a word can be split across several escape
// sequences - the banner's gradient breaks "collect" into three. Searching the
// raw string for text would miss it even though the terminal draws it.
func plain(s string) string { return ansi.Strip(s) }
