package report

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FileMode is the permission applied to files written beside a track.
const FileMode fs.FileMode = 0o644

// Marshal renders a report as compact JSON.
//
// Indenting it would put every spectrum value on its own line, and there are
// thousands of them: the leading spaces and newlines came to 56% of a report,
// more than all of its numbers together. What indentation buys is a file that
// reads well under a pager, and a report is 5,000 lines of frequency bins that
// no pager makes legible anyway. Whoever opens one reaches for jq, which does
// not care.
func Marshal(r *Report) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("report: report is nil")
	}
	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("report: marshal: %w", err)
	}
	return append(data, '\n'), nil
}

// Write puts data at path without ever leaving a partial file behind.
//
// The content is written to a temporary neighbour and renamed into place,
// which is atomic within a directory. A run interrupted halfway therefore
// leaves either the old file or the new one, never half of either.
func Write(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	// The cleanup path runs only when the write already failed, so a failure
	// to close or remove the temporary file has nothing left to report.
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	// These assign the named return rather than shadowing it: the deferred
	// cleanup above tests err to decide whether to remove the temporary
	// file, so a := here would leave one behind after a failed write.
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Chmod(FileMode); err != nil { //nolint:gocritic // assigns the named return on purpose
		return err
	}
	if err = tmp.Close(); err != nil { //nolint:gocritic // assigns the named return on purpose
		return err
	}
	return os.Rename(tmpName, path)
}
