package inventory

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"flac-util/internal/report"
)

// DefaultExtension is the file extension looked for unless told otherwise.
const DefaultExtension = ".flac"

// ScanOptions configures a scan.
type ScanOptions struct {
	// Roots are the directories, or individual files, to look through.
	Roots []string
	// Extensions are the track extensions to look for, compared without
	// regard to case. Empty selects DefaultExtension.
	Extensions []string
	// ReportSuffix names the report written beside a track. Empty selects
	// report.DefaultReportSuffix.
	ReportSuffix string
	// Workers is how many reports are read at once. Zero selects one per CPU.
	Workers int
}

// normalise fills in the defaults and returns the extension set.
func (o *ScanOptions) normalise() ([]string, error) {
	if len(o.Roots) == 0 {
		return nil, errors.New("inventory: no roots to scan")
	}
	if o.ReportSuffix == "" {
		o.ReportSuffix = report.DefaultReportSuffix
	}
	if o.Workers <= 0 {
		o.Workers = runtime.NumCPU()
	}

	source := o.Extensions
	if len(source) == 0 {
		source = []string{DefaultExtension}
	}
	extensions := make([]string, 0, len(source))
	for _, ext := range source {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extensions = append(extensions, ext)
	}
	if len(extensions) == 0 {
		return nil, errors.New("inventory: no usable extensions")
	}
	return extensions, nil
}

// Scan finds every track under the given roots, reads the report beside each
// one, and returns them grouped by directory.
//
// Reports with no track beside them are included as their own entries rather
// than passed over. A report is the only record that a file was ever there,
// and losing it silently is how a moved album becomes a mystery.
func Scan(opts ScanOptions) ([]Group, error) {
	extensions, err := opts.normalise()
	if err != nil {
		return nil, err
	}

	found, err := walk(opts, extensions)
	if err != nil {
		return nil, err
	}

	groups := make([]Group, 0, len(found))
	for dir, paths := range found {
		slices.Sort(paths)
		groups = append(groups, Group{Dir: dir, Entries: load(paths, opts)})
	}

	for i := range groups {
		groups[i].describe()
	}
	slices.SortFunc(groups, func(a, b Group) int { return strings.Compare(a.Dir, b.Dir) })
	return groups, nil
}

// walk returns the track paths found under the roots, keyed by directory.
//
// Orphaned reports are turned back into the track path they describe, so that
// a directory holding nothing but reports still forms a group.
func walk(opts ScanOptions, extensions []string) (map[string][]string, error) {
	found := map[string][]string{}
	seen := map[string]bool{}

	add := func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		dir := filepath.Dir(path)
		found[dir] = append(found[dir], path)
	}

	// An orphaned report names its track by suffix alone, so the extension it
	// had has to be assumed. The first configured extension is the assumption.
	orphanExtension := extensions[0]

	for _, root := range opts.Roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			name := d.Name()
			if strings.HasSuffix(name, opts.ReportSuffix) {
				track := strings.TrimSuffix(path, opts.ReportSuffix) + orphanExtension
				if _, err := os.Stat(track); errors.Is(err, fs.ErrNotExist) {
					add(track)
				}
				return nil
			}
			if slices.Contains(extensions, strings.ToLower(filepath.Ext(name))) {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scanning %s: %w", root, err)
		}
	}
	return found, nil
}

// load reads the report for each track, several at a time, since a scan of a
// library is dominated by reading thousands of small files.
func load(paths []string, opts ScanOptions) []Entry {
	entries := make([]Entry, len(paths))

	var wg sync.WaitGroup
	jobs := make(chan int)

	for range min(opts.Workers, len(paths)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				r, state, err := report.Load(paths[i], opts.ReportSuffix)
				entries[i] = Entry{
					AudioPath: paths[i],
					State:     state,
					Report:    r,
					Err:       err,
				}
			}
		}()
	}

	for i := range paths {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return entries
}
