package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"homelab/flac-util/internal/inventory"
	"homelab/flac-util/internal/report"
)

// collectedTrack copies a test track into dir and collects it, leaving the
// track and its report the way the tool does in a library.
func collectedTrack(t *testing.T, dir, name string) string {
	t.Helper()

	source := filepath.Join("..", "audio", "testdata", "sine16.flac")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	track := filepath.Join(dir, name+".flac")
	if err := os.WriteFile(track, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"collect", track,
		"--display", "never",
		"--spec-width", "64",
		"--spec-height", "32",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("collect failed: %v\n%s", err, out.String())
	}
	return track
}

// runView runs the view command over a path and returns what it printed.
func runViewCommand(t *testing.T, args ...string) string {
	t.Helper()

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"view", "--display", "never"}, args...))

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, out.String())
	}
	return out.String()
}

func TestRootCommandHasView(t *testing.T) {
	var found bool
	for _, c := range NewRootCommand().Commands() {
		if c.Name() == "view" {
			found = true
		}
	}
	if !found {
		t.Error("the root command has no view subcommand")
	}
}

func TestViewEndToEnd(t *testing.T) {
	dir := t.TempDir()
	collectedTrack(t, dir, "01 Track")

	got := runViewCommand(t, dir)

	for _, want := range []string{"01 Track", dir, "track", "cutoff", "16bit"} {
		if !strings.Contains(got, want) {
			t.Errorf("the listing does not mention %q:\n%s", want, got)
		}
	}
}

func TestViewShowsATrackWithNoReport(t *testing.T) {
	dir := t.TempDir()
	track := collectedTrack(t, dir, "01 Track")
	if err := os.Remove(report.SidecarPath(track, report.DefaultReportSuffix)); err != nil {
		t.Fatalf("removing the report: %v", err)
	}

	if got := runViewCommand(t, dir); !strings.Contains(got, string(report.StateMissing)) {
		t.Errorf("a track with no report is not shown as such:\n%s", got)
	}
}

func TestViewWritesNothing(t *testing.T) {
	// The command is for reading a library, including one full of files that
	// are hardlinked into a torrent. It must leave the directory as it found
	// it.
	dir := t.TempDir()
	collectedTrack(t, dir, "01 Track")

	before, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	runViewCommand(t, dir)

	after, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(before) != len(after) {
		t.Errorf("the directory held %d files and now holds %d", len(before), len(after))
	}
}

func TestViewOfAnEmptyDirectory(t *testing.T) {
	if got := runViewCommand(t, t.TempDir()); !strings.Contains(got, "no tracks found") {
		t.Errorf("view of an empty directory printed:\n%s", got)
	}
}

func TestViewReportsAMissingPath(t *testing.T) {
	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"view", "--display", "never", filepath.Join(t.TempDir(), "absent")})

	if err := root.Execute(); err == nil {
		t.Error("view of a path that is not there returned no error")
	}
}

func TestViewCutoffFloorDefaultsToTheSpectrogramRange(t *testing.T) {
	// A tighter floor reads the faint top end of a lossless track as an
	// ending. The default has to be the range the image is drawn over, or the
	// table and the picture disagree about the same file.
	cmd := newViewCommandForTest(t)
	flag := cmd.Flags().Lookup("cutoff-floor-db")
	if flag == nil {
		t.Fatal("there is no cutoff-floor-db flag")
	}
	if got, want := flag.DefValue, "120"; got != want {
		t.Errorf("cutoff-floor-db defaults to %q, want %q", got, want)
	}
	if inventory.DefaultCutoffFloorDB != 120 {
		t.Errorf("DefaultCutoffFloorDB = %v, want 120", inventory.DefaultCutoffFloorDB)
	}
}

// newViewCommandForTest returns the view command with its flags registered.
func newViewCommandForTest(t *testing.T) *cobra.Command {
	t.Helper()

	var global GlobalOptions
	return newViewCommand(&global)
}
