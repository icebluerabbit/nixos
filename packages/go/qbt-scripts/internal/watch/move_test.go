package watch

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The watch directory and the export directory are frequently separate
// mounts, where rename returns EXDEV and the import would otherwise be
// reported as a backup failure on every single file.
func TestMoveFallsBackToCopyAcrossFilesystems(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.torrent")
	dst := filepath.Join(dir, "backup", "a.torrent")

	if err := os.WriteFile(src, []byte("d8:announce..."), 0o600); err != nil {
		t.Fatalf("write %s: %v", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	rename = func(string, string) error { return syscall.EXDEV }
	t.Cleanup(func() { rename = os.Rename })

	if err := move(src, dst); err != nil {
		t.Fatalf("move() error = %v", err)
	}

	got, err := os.ReadFile(dst) //nolint:gosec // the test's own temporary directory
	if err != nil {
		t.Fatalf("read %s: %v", dst, err)
	}
	if string(got) != "d8:announce..." {
		t.Errorf("copied contents = %q, want the original", got)
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat(src) error = %v, want the source to be gone", err)
	}
}

func TestMoveReportsACopyFailure(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.torrent")
	if err := os.WriteFile(src, nil, 0o600); err != nil {
		t.Fatalf("write %s: %v", src, err)
	}

	rename = func(string, string) error { return syscall.EXDEV }
	t.Cleanup(func() { rename = os.Rename })

	// No such directory, so the copy cannot create its destination.
	if err := move(src, filepath.Join(dir, "absent", "a.torrent")); err == nil {
		t.Fatal("move() error = nil, want a failure")
	}
}

func TestCopyFileReportsAMissingSource(t *testing.T) {
	dir := t.TempDir()
	if err := copyFile(filepath.Join(dir, "absent"), filepath.Join(dir, "dst")); err == nil {
		t.Fatal("copyFile() error = nil, want a failure")
	}
}
