package inventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"flac-util/internal/report"
)

// library builds a directory of tracks and reports to scan.
type library struct {
	t    *testing.T
	root string
}

// newLibrary returns an empty library under a temporary directory.
func newLibrary(t *testing.T) *library {
	t.Helper()
	return &library{t: t, root: t.TempDir()}
}

// track writes a track, and a report beside it when one is given.
func (l *library) track(rel string, r *report.Report) string {
	l.t.Helper()

	path := filepath.Join(l.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		l.t.Fatalf("making %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("not really flac"), 0o644); err != nil {
		l.t.Fatalf("writing %s: %v", path, err)
	}
	if r != nil {
		l.report(path, r)
	}
	return path
}

// report writes a report describing the track as it is on disk.
func (l *library) report(track string, r *report.Report) {
	l.t.Helper()

	if info, err := os.Stat(track); err == nil {
		r.File.Path, r.File.Size, r.File.Modified = track, info.Size(), info.ModTime()
	}
	data, err := json.Marshal(r)
	if err != nil {
		l.t.Fatalf("marshalling: %v", err)
	}
	if err := os.WriteFile(report.SidecarPath(track, report.DefaultReportSuffix), data, 0o644); err != nil {
		l.t.Fatalf("writing the report: %v", err)
	}
}

// orphan writes a report with no track beside it.
func (l *library) orphan(rel string, r *report.Report) {
	l.t.Helper()

	track := l.track(rel, r)
	if err := os.Remove(track); err != nil {
		l.t.Fatalf("removing %s: %v", track, err)
	}
}

// scan runs a scan over the library.
func (l *library) scan() []Group {
	l.t.Helper()

	groups, err := Scan(ScanOptions{Roots: []string{l.root}})
	if err != nil {
		l.t.Fatalf("Scan() error = %v", err)
	}
	return groups
}

// tracksOf names a group's entries.
func tracksOf(g Group) []string {
	out := make([]string, len(g.Entries))
	for i, e := range g.Entries {
		out[i] = e.Track()
	}
	return out
}

func TestScanGroupsByDirectory(t *testing.T) {
	l := newLibrary(t)
	l.track("i-dle/I SWAY (2024)/01 - Klaxon.flac", trackReport())
	l.track("i-dle/I SWAY (2024)/02 - Last Forever.flac", trackReport())
	l.track("i-dle/HEAT (2023)/01 - I DO.flac", trackReport())

	groups := l.scan()
	if len(groups) != 2 {
		t.Fatalf("scanned %d groups, want 2", len(groups))
	}

	// Groups come back in directory order, so HEAT precedes I SWAY.
	if got, want := tracksOf(groups[0]), []string{"01 - I DO"}; !sameOrder(got, want) {
		t.Errorf("first group = %v, want %v", got, want)
	}
	if got, want := tracksOf(groups[1]), []string{"01 - Klaxon", "02 - Last Forever"}; !sameOrder(got, want) {
		t.Errorf("second group = %v, want %v", got, want)
	}
}

func TestScanNamesGroupsFromTags(t *testing.T) {
	l := newLibrary(t)
	r := trackReport()
	r.Tags = report.Tags{Comments: [][2]string{
		{"ALBUMARTIST", "(G)I-DLE"},
		{"ALBUM", "I SWAY"},
	}}
	l.track("i-dle/I SWAY (2024)/01 - Klaxon.flac", r)

	groups := l.scan()
	if len(groups) != 1 {
		t.Fatalf("scanned %d groups, want 1", len(groups))
	}
	if got := groups[0].Label(); got != "(G)I-DLE — I SWAY" {
		t.Errorf("Label() = %q", got)
	}
}

func TestScanReadsTheStateOfEachTrack(t *testing.T) {
	l := newLibrary(t)
	l.track("A/01 - reported.flac", trackReport())
	l.track("A/02 - bare.flac", nil)
	l.orphan("A/03 - gone.flac", trackReport())

	stale := l.track("A/04 - stale.flac", trackReport())
	if err := os.WriteFile(stale, []byte("this file has been replaced since"), 0o644); err != nil {
		t.Fatalf("replacing: %v", err)
	}

	groups := l.scan()
	if len(groups) != 1 {
		t.Fatalf("scanned %d groups, want 1", len(groups))
	}

	want := map[string]report.State{
		"01 - reported": report.StateCurrent,
		"02 - bare":     report.StateMissing,
		"03 - gone":     report.StateOrphan,
		"04 - stale":    report.StateStale,
	}
	if len(groups[0].Entries) != len(want) {
		t.Fatalf("scanned %d entries, want %d: %v", len(groups[0].Entries), len(want), tracksOf(groups[0]))
	}
	for _, entry := range groups[0].Entries {
		if got := entry.State; got != want[entry.Track()] {
			t.Errorf("%s is %q, want %q", entry.Track(), got, want[entry.Track()])
		}
	}
}

func TestScanIgnoresOtherFiles(t *testing.T) {
	l := newLibrary(t)
	l.track("A/01 - Track.flac", trackReport())
	l.track("A/cover.jpg", nil)
	l.track("A/album.nfo", nil)

	groups := l.scan()
	if len(groups) != 1 {
		t.Fatalf("scanned %d groups, want 1", len(groups))
	}
	if got, want := tracksOf(groups[0]), []string{"01 - Track"}; !sameOrder(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}
}

func TestScanAcceptsASingleFile(t *testing.T) {
	l := newLibrary(t)
	path := l.track("A/01 - Track.flac", trackReport())

	groups, err := Scan(ScanOptions{Roots: []string{path}})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(groups) != 1 || len(groups[0].Entries) != 1 {
		t.Fatalf("scanning one file gave %d groups", len(groups))
	}
}

func TestScanHonoursExtensions(t *testing.T) {
	l := newLibrary(t)
	l.track("A/01 - Track.flac", nil)
	l.track("A/02 - Track.wav", nil)

	groups, err := Scan(ScanOptions{Roots: []string{l.root}, Extensions: []string{"WAV"}})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if got, want := tracksOf(groups[0]), []string{"02 - Track"}; !sameOrder(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}
}

func TestScanRejectsNoRoots(t *testing.T) {
	if _, err := Scan(ScanOptions{}); err == nil {
		t.Error("Scan() with no roots returned no error")
	}
}

func TestScanRejectsNoUsableExtensions(t *testing.T) {
	if _, err := Scan(ScanOptions{Roots: []string{t.TempDir()}, Extensions: []string{" "}}); err == nil {
		t.Error("Scan() with no usable extensions returned no error")
	}
}

func TestScanReportsAMissingRoot(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-here")
	if _, err := Scan(ScanOptions{Roots: []string{missing}}); err == nil {
		t.Error("Scan() of an absent root returned no error")
	}
}
