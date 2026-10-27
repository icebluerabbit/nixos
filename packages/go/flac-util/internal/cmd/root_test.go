package cmd

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestDisplayModes(t *testing.T) {
	want := []string{"auto", "always", "never"}
	if got := DisplayModes(); !slices.Equal(got, want) {
		t.Errorf("DisplayModes() = %v, want %v", got, want)
	}
}

func TestGlobalOptionsTool(t *testing.T) {
	tool := GlobalOptions{}.Tool()

	if tool.Name != Name {
		t.Errorf("Tool().Name = %q, want %q", tool.Name, Name)
	}
	if tool.Version != Version {
		t.Errorf("Tool().Version = %q, want %q", tool.Version, Version)
	}
	if tool.Version == "" {
		t.Error("the version is empty, which would break the resume check")
	}
}

func TestUseInteractive(t *testing.T) {
	tests := []struct {
		name       string
		display    string
		isTerminal bool
		want       bool
		wantErr    bool
	}{
		{"auto on a terminal", DisplayAuto, true, true, false},
		{"auto in a pipe", DisplayAuto, false, false, false},
		{"empty defaults to auto", "", true, true, false},
		{"forced on in a pipe", DisplayAlways, false, true, false},
		{"forced off on a terminal", DisplayNever, true, false, false},
		{"unknown mode", "fancy", true, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GlobalOptions{Display: tt.display}.UseInteractive(tt.isTerminal)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UseInteractive() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("UseInteractive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewRootCommand(t *testing.T) {
	root := NewRootCommand()

	if root.Use != Name {
		t.Errorf("Use = %q, want %q", root.Use, Name)
	}
	if root.Version != Version {
		t.Errorf("Version = %q, want %q", root.Version, Version)
	}
	if !root.SilenceUsage {
		t.Error("SilenceUsage = false; a failed run should not print the whole usage")
	}

	for _, name := range []string{"display", "verbose"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("the %q flag is missing", name)
		}
	}
}

func TestRootCommandHasCollect(t *testing.T) {
	root := NewRootCommand()

	var found bool
	for _, sub := range root.Commands() {
		if strings.HasPrefix(sub.Use, "collect") {
			found = true
		}
	}
	if !found {
		t.Error("the collect subcommand is not registered")
	}
}

// TestRootCommandDescribesWhatItDoesNotDo checks the documentation states the
// two promises the program actually makes, since a reader who assumes
// otherwise could do real damage with it.
func TestRootCommandDescribesWhatItDoesNotDo(t *testing.T) {
	root := NewRootCommand()

	for _, want := range []string{"read-only", "never"} {
		if !strings.Contains(root.Long, want) {
			t.Errorf("the description does not mention %q:\n%s", want, root.Long)
		}
	}
}

func TestRootCommandHelp(t *testing.T) {
	root := NewRootCommand()

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "collect") {
		t.Errorf("help does not mention the collect command:\n%s", out.String())
	}
}

func TestRootCommandRejectsUnknownSubcommand(t *testing.T) {
	root := NewRootCommand()

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"analyse"})

	if err := root.Execute(); err == nil {
		t.Fatal("Execute() with an unknown subcommand succeeded, want error")
	}
}

func TestCollectRequiresAPath(t *testing.T) {
	root := NewRootCommand()

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"collect"})

	if err := root.Execute(); err == nil {
		t.Fatal("collect without a path succeeded, want error")
	}
}
