package tui

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
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
		msg  tea.KeyPressMsg
		want bool
	}{
		{"letter q", keyPress("q"), true},
		{"control c", keyPress("ctrl+c"), true},
		{"escape", keyPress("esc"), true},
		{"an unrelated letter", keyPress("x"), false},
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

	msg := keyPress("?")
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
