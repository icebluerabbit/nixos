package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarshalRejectsNil(t *testing.T) {
	if _, err := Marshal(nil); err == nil {
		t.Fatal("Marshal(nil) succeeded, want error")
	}
}

func TestMarshalIsCompactAndNewlineTerminated(t *testing.T) {
	data, err := Marshal(&Report{Note: Note})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Error("output does not end with a newline")
	}
	if n := bytes.Count(data, []byte("\n")); n != 1 {
		t.Errorf("output holds %d newlines, want only the terminator", n)
	}
	if !json.Valid(data) {
		t.Error("output is not valid JSON")
	}
}

// TestMarshalSpendsNothingOnLayout is the reason Marshal is compact: the
// spectrum arrays are thousands of values long, and indenting them costs more
// bytes in leading spaces and newlines than the values themselves take.
func TestMarshalSpendsNothingOnLayout(t *testing.T) {
	const bins = 1024

	spectrum := &Spectrum{AverageDBFS: make([]Number, bins)}
	for i := range spectrum.AverageDBFS {
		spectrum.AverageDBFS[i] = Number(-90.5)
	}

	// The note is left off: it is prose, and its spaces would be counted as
	// layout below.
	data, err := Marshal(&Report{Spectrum: spectrum})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	indented, err := json.MarshalIndent(&Report{Spectrum: spectrum}, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}

	if len(data) >= len(indented) {
		t.Errorf("compact output is %d bytes against %d indented, want smaller",
			len(data), len(indented))
	}
	// Whitespace is what separates one form from the other, so it is what the
	// saving has to come from.
	if got := bytes.Count(data, []byte(" ")) + bytes.Count(data, []byte("\n")); got > 1 {
		t.Errorf("output carries %d bytes of layout, want only the terminator", got)
	}
}

func TestWriteCreatesTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "track.report.json")
	want := []byte(`{"hello":"world"}`)

	if err := Write(path, want); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("contents = %s, want %s", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != FileMode {
		t.Errorf("mode = %v, want %v", info.Mode().Perm(), FileMode)
	}
}

func TestWriteReplacesAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "track.report.json")

	if err := Write(path, []byte("old")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := Write(path, []byte("new")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "new" {
		t.Errorf("contents = %s, want new", got)
	}
}

func TestWriteLeavesNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "track.report.json")

	if err := Write(path, []byte("payload")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want only the report", names)
	}
}

// TestWriteCleansUpAfterAFailedRename covers the case the temporary file
// exists for: a write that cannot complete must not leave debris beside the
// music.
func TestWriteCleansUpAfterAFailedRename(t *testing.T) {
	dir := t.TempDir()
	// Renaming onto an existing directory fails, which exercises the cleanup
	// path without needing to break the filesystem.
	path := filepath.Join(dir, "occupied")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	if err := Write(path, []byte("payload")); err == nil {
		t.Fatal("Write() onto a directory succeeded, want error")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temporary file %q was left behind", e.Name())
		}
	}
}

func TestWriteToMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent", "track.report.json")
	if err := Write(path, []byte("payload")); err == nil {
		t.Fatal("Write() into a missing directory succeeded, want error")
	}
}

func TestWriteEmptyPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := Write(path, nil); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("size = %d, want 0", info.Size())
	}
}
