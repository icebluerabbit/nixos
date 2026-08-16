package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func TestDefaultKeyMapBindings(t *testing.T) {
	keys := DefaultKeyMap()

	if !keys.Quit.Enabled() {
		t.Error("the quit binding is disabled")
	}
	if !keys.Help.Enabled() {
		t.Error("the help binding is disabled")
	}
	if keys.Quit.Help().Desc == "" {
		t.Error("the quit binding has no description, so it would not appear in the hints")
	}
}

func TestQuitBindingMatchesTheUsualKeys(t *testing.T) {
	keys := DefaultKeyMap()

	tests := []struct {
		name string
		msg  tea.KeyMsg
		want bool
	}{
		{"letter q", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}, true},
		{"control c", tea.KeyMsg{Type: tea.KeyCtrlC}, true},
		{"escape", tea.KeyMsg{Type: tea.KeyEsc}, true},
		{"an unrelated letter", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := key.Matches(tt.msg, keys.Quit); got != tt.want {
				t.Errorf("Matches(%v) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestHelpBindingMatchesQuestionMark(t *testing.T) {
	keys := DefaultKeyMap()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")}
	if !key.Matches(msg, keys.Help) {
		t.Error("the help binding does not match a question mark")
	}
}

func TestKeyMapHelpSections(t *testing.T) {
	keys := DefaultKeyMap()

	short := keys.ShortHelp()
	if len(short) != 2 {
		t.Errorf("ShortHelp() has %d entries, want 2", len(short))
	}

	full := keys.FullHelp()
	if len(full) != 1 {
		t.Fatalf("FullHelp() has %d columns, want 1", len(full))
	}
	if len(full[0]) != 2 {
		t.Errorf("FullHelp()[0] has %d entries, want 2", len(full[0]))
	}
}
