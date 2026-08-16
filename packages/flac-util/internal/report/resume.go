package report

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Default suffixes for the files written beside a track. A track at
// "Foo.flac" gets "Foo.report.json" and "Foo.spec.png", which sort next to it
// in any listing and make the pairing obvious to anyone who receives a single
// file rather than a whole folder.
const (
	DefaultReportSuffix = ".report.json"
	DefaultSpecSuffix   = ".spec.png"
)

// SidecarPath returns the path of a file written beside the given track.
func SidecarPath(audioPath, suffix string) string {
	return strings.TrimSuffix(audioPath, filepath.Ext(audioPath)) + suffix
}

// SkipReason explains why a file needs no work.
type SkipReason string

// The reasons a file can be skipped.
const (
	// SkipNone means the file must be processed.
	SkipNone SkipReason = ""
	// SkipUpToDate means a report already describes the file as it is now.
	SkipUpToDate SkipReason = "already reported"
)

// CheckOptions configures the resume decision.
type CheckOptions struct {
	// ReportSuffix names the report file. Empty selects DefaultReportSuffix.
	ReportSuffix string
	// ToolVersion is the current version. A report written by a different
	// version is redone, since the numbers may not be comparable.
	ToolVersion string
	// Force processes every file regardless of what is already there.
	Force bool
}

// Check reports whether a track already has a usable report.
//
// The test is deliberately cheap: it reads the existing report and compares
// the recorded size, modification time and tool version. Confirming the audio
// itself would mean decoding it, which is the very work being avoided.
func Check(audioPath string, info fs.FileInfo, opts CheckOptions) (SkipReason, error) {
	if opts.Force {
		return SkipNone, nil
	}

	suffix := opts.ReportSuffix
	if suffix == "" {
		suffix = DefaultReportSuffix
	}

	data, err := os.ReadFile(SidecarPath(audioPath, suffix))
	if errors.Is(err, fs.ErrNotExist) {
		return SkipNone, nil
	}
	if err != nil {
		return SkipNone, err
	}

	var existing Report
	if err := json.Unmarshal(data, &existing); err != nil {
		// An unreadable report is treated as absent rather than fatal: the
		// remedy is to write a good one over it.
		return SkipNone, nil
	}

	switch {
	case existing.Tool.Version != opts.ToolVersion:
		return SkipNone, nil
	case info == nil:
		return SkipNone, nil
	case existing.File.Size != info.Size():
		return SkipNone, nil
	case !existing.File.Modified.Equal(info.ModTime()):
		return SkipNone, nil
	}
	return SkipUpToDate, nil
}
