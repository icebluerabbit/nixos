package ingest

import (
	"slices"
	"testing"
)

// TestUnpackerrArchivesJoinsDirAndMember covers the payload's odd shape.
//
// unpackerr reports archives as a map of containing DIRECTORY to member
// filenames, not as a list of paths. Storing the member alone would give a
// primary key of "x.rar" shared by every release that ever shipped one.
func TestUnpackerrArchivesJoinsDirAndMember(t *testing.T) {
	var h unpackerrHook
	h.Data.Archives = map[string][]string{
		"/downloads/Some.Release": {"some.rar", "some.r00"},
	}

	got := h.archives()
	slices.Sort(got)
	want := []string{
		"/downloads/Some.Release/some.r00",
		"/downloads/Some.Release/some.rar",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// TestUnpackerrArchivesKeepsAbsoluteMembers covers the other version's shape.
//
// Some builds report the member as a full path. Prefixing it with the directory
// would produce /downloads/x//downloads/x/some.rar.
func TestUnpackerrArchivesKeepsAbsoluteMembers(t *testing.T) {
	var h unpackerrHook
	h.Data.Archives = map[string][]string{
		"/downloads/x": {"/downloads/x/some.rar"},
	}
	got := h.archives()
	if len(got) != 1 || got[0] != "/downloads/x/some.rar" {
		t.Errorf("got %q", got)
	}
}

// TestUnpackerrArchivesFallsBackToPath is the failure event.
//
// An extractfailed carries no archive map, and the extraction still needs a key
// — otherwise the one event that says WHY an extraction failed is the one event
// that cannot be stored.
func TestUnpackerrArchivesFallsBackToPath(t *testing.T) {
	h := unpackerrHook{Path: "/downloads/Some.Release"}
	got := h.archives()
	if len(got) != 1 || got[0] != "/downloads/Some.Release" {
		t.Errorf("got %q", got)
	}
}

// TestUnpackerrArchivesEmpty means there is nothing to record against.
func TestUnpackerrArchivesEmpty(t *testing.T) {
	var h unpackerrHook
	if got := h.archives(); len(got) != 0 {
		t.Errorf("got %q, want none", got)
	}
}

// TestExtractionFinished separates terminal events from progress.
//
// Stamping finished_at on "extracting" would make every in-flight extraction
// look complete, and the column would answer "has this finished" with yes
// always.
func TestExtractionFinished(t *testing.T) {
	terminal := []string{"extracted", "imported", "deleted", "extractfailed",
		"EXTRACTED", "ExtractFailed"}
	for _, e := range terminal {
		if !extractionFinished(e) {
			t.Errorf("%q should be terminal", e)
		}
	}
	inflight := []string{"queued", "extracting", "", "unknown"}
	for _, e := range inflight {
		if extractionFinished(e) {
			t.Errorf("%q should not be terminal", e)
		}
	}
}

// TestUnpackerrDecodesTheRealShape guards the JSON tags.
//
// The ids block is where the infohash lives, and extractions are keyed by it —
// a wrong tag would make every event fall out at the "no download id" guard and
// the table would simply stay empty.
func TestUnpackerrDecodesTheRealShape(t *testing.T) {
	const body = `{
	  "path": "/downloads/Some.Release",
	  "app": "Sonarr",
	  "event": "extracted",
	  "ids": {"downloadId": "ABCDEF0123456789", "title": "Some Release"},
	  "data": {
	    "archives": {"/downloads/Some.Release": ["some.rar"]},
	    "output": "/downloads/Some.Release/_unpackerred",
	    "start": "2026-09-13T12:00:00Z",
	    "bytes": 4096,
	    "error": ""
	  }
	}`
	var h unpackerrHook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if h.IDs.DownloadID != "ABCDEF0123456789" {
		t.Errorf("download id = %q", h.IDs.DownloadID)
	}
	if h.Data.Output != "/downloads/Some.Release/_unpackerred" {
		t.Errorf("output = %q", h.Data.Output)
	}
	if h.Data.Start.IsZero() {
		t.Error("start did not decode")
	}
	if got := h.archives(); len(got) != 1 {
		t.Errorf("archives = %q", got)
	}
}
