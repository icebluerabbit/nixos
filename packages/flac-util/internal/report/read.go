package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"
)

// State describes how a track and its report relate to each other.
//
// The distinction matters to a reader deciding whether to trust a number: a
// report that no longer describes the file beside it is worse than no report
// at all, because it looks like an answer.
type State string

// The states a track can be in.
const (
	// StateCurrent means the report describes the file as it is now.
	StateCurrent State = "current"
	// StateStale means the file has changed since the report was written.
	StateStale State = "stale"
	// StateMissing means the track has no report.
	StateMissing State = "no report"
	// StateOrphan means a report was found with no track beside it.
	StateOrphan State = "orphan"
	// StateUnreadable means the report exists but could not be parsed.
	StateUnreadable State = "unreadable"
)

// Read loads a report from disk.
func Read(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("report %s: %w", path, err)
	}
	return &r, nil
}

// Describes reports whether a report still describes a file of the given size
// and modification time.
//
// This is the same test Check makes before skipping work, phrased for a reader
// rather than for the pipeline. It compares what was recorded rather than
// re-reading the audio, so it catches a replaced file but not one edited in
// place without changing its size or timestamp.
func Describes(r *Report, size int64, modified time.Time) bool {
	return r != nil && r.File.Size == size && r.File.Modified.Equal(modified)
}

// Load reads the report beside a track and reports how the two relate.
//
// A track with no report and a report with no track are both ordinary results
// rather than errors: a library part way through being collected contains
// plenty of the first, and the second is how a moved file announces itself.
func Load(audioPath, suffix string) (*Report, State, error) {
	if suffix == "" {
		suffix = DefaultReportSuffix
	}

	r, err := Read(SidecarPath(audioPath, suffix))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, StateMissing, nil
	case err != nil:
		var syntax *json.SyntaxError
		var unmarshal *json.UnmarshalTypeError
		if errors.As(err, &syntax) || errors.As(err, &unmarshal) {
			return nil, StateUnreadable, nil
		}
		return nil, StateUnreadable, err
	}

	info, err := os.Stat(audioPath)
	if errors.Is(err, fs.ErrNotExist) {
		return r, StateOrphan, nil
	}
	if err != nil {
		return r, StateUnreadable, err
	}

	if !Describes(r, info.Size(), info.ModTime()) {
		return r, StateStale, nil
	}
	return r, StateCurrent, nil
}
