package main

import (
	"os"
	"testing"

	"flac-util/cmd"
)

// main itself is one call to os.Exit and cannot be exercised directly, so
// these tests cover the only thing it does: hand the process's arguments to
// the command tree and turn the result into an exit code.

// withArgs runs fn with the process arguments replaced.
func withArgs(t *testing.T, args []string, fn func()) {
	t.Helper()

	original := os.Args
	t.Cleanup(func() { os.Args = original })

	os.Args = args
	fn()
}

func TestExecuteSucceedsForHelp(t *testing.T) {
	withArgs(t, []string{"flac-util", "--help"}, func() {
		if code := cmd.Execute(); code != 0 {
			t.Errorf("Execute() = %d, want 0", code)
		}
	})
}

func TestExecuteSucceedsForVersion(t *testing.T) {
	withArgs(t, []string{"flac-util", "--version"}, func() {
		if code := cmd.Execute(); code != 0 {
			t.Errorf("Execute() = %d, want 0", code)
		}
	})
}

func TestExecuteFailsForAnUnknownCommand(t *testing.T) {
	withArgs(t, []string{"flac-util", "analyse"}, func() {
		if code := cmd.Execute(); code == 0 {
			t.Error("Execute() = 0 for an unknown command, want a non-zero exit code")
		}
	})
}

func TestExecuteFailsWhenCollectHasNoPath(t *testing.T) {
	withArgs(t, []string{"flac-util", "collect"}, func() {
		if code := cmd.Execute(); code == 0 {
			t.Error("Execute() = 0 for collect without a path, want a non-zero exit code")
		}
	})
}
