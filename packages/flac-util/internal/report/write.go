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

// Marshal renders a report as indented JSON.
//
// It is indented because a person will open it: the file sits next to the
// music, and whoever finds it is owed something readable.
func Marshal(r *Report) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("report: report is nil")
	}
	data, err := json.MarshalIndent(r, "", "  ")
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

	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Chmod(FileMode); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
