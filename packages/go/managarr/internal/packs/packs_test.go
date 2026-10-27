package packs

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A multi-file pack yields both spellings of each entry.
//
// qBittorrent names a single-file torrent after the file WITH its extension and
// a multi-file torrent after the folder WITHOUT one. Which of the two a given
// episode was cannot be known from the pack directory, so both are offered and
// the lookup decides.
func TestPackFilesOffersBothSpellings(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Show.S01E01.1080p.WEB-DL-GRP.mkv"), "a")

	got, err := packFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := allSpellings(got)
	for _, want := range []string{
		"Show.S01E01.1080p.WEB-DL-GRP.mkv",
		"Show.S01E01.1080p.WEB-DL-GRP",
	} {
		if !slices.Contains(names, want) {
			t.Errorf("%q missing from %v", want, got)
		}
	}
}

// An entry with no extension is offered once, not twice.
func TestPackFilesNoExtension(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Show.S01E02.2160p.WEB-GRP"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := packFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].name != "Show.S01E02.2160p.WEB-GRP" {
		t.Errorf("got %v, want one entry", got)
	}
}

// A single-file pack has nothing to enumerate — there are no episodes inside it
// to attribute to anything.
func TestPackFilesOnSingleFileIsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Show.S01.1080p.WEB-DL-GRP.mkv")
	write(t, path, "a")

	got, err := packFiles(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}

// A missing pack directory is an error the caller reports, not a silent empty
// result — an empty list would read as "this pack contains nothing".
func TestPackFilesMissingDirectoryErrors(t *testing.T) {
	if _, err := packFiles(filepath.Join(t.TempDir(), "gone")); err == nil {
		t.Error("expected an error for a missing pack")
	}
}

// The join is EXACT. A name that differs by normalisation must not resolve:
// this is deliberately string equality, not similarity, so a drifted name
// reports as unresolved rather than attaching a pack to the wrong episode.
func TestPackFilesDoesNotNormalise(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Show.from.Accounts.S03E03.1080p.mkv"), "a")

	got, err := packFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := allSpellings(got)
	// Case is preserved verbatim — "Show.From.Accounts" would be a different
	// torrent as far as this is concerned, which is the point.
	if !slices.Contains(names, "Show.from.Accounts.S03E03.1080p.mkv") {
		t.Errorf("name was altered: %v", got)
	}
	if slices.Contains(names, "show.from.accounts.s03e03.1080p.mkv") {
		t.Error("name was lowercased; the join must be exact")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A release-name suffix is NOT an extension.
//
// filepath.Ext reports ".WEB-GRP" here and ".264-FLUX" for an H.264 release.
// Stripping either invents a name no torrent ever had, and offering an invented
// name to an exact-match lookup risks attaching a pack to the wrong episode.
func TestReleaseSuffixIsNotStripped(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Show.S01E04.1080p.AMZN.WEB-DL.DDP5.1.H.264-FLUX.mkv"), "a")
	write(t, filepath.Join(dir, "Show.S01E05.2160p.WEB-GRP"), "b")

	got, err := packFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := allSpellings(got)
	// The real extension goes.
	if !slices.Contains(names, "Show.S01E04.1080p.AMZN.WEB-DL.DDP5.1.H.264-FLUX") {
		t.Errorf(".mkv was not stripped: %v", got)
	}
	// These would be inventions.
	for _, never := range []string{
		"Show.S01E04.1080p.AMZN.WEB-DL.DDP5.1.H",
		"Show.S01E05.2160p",
	} {
		if slices.Contains(names, never) {
			t.Errorf("INVENTED NAME %q in %v", never, got)
		}
	}
}

// allSpellings flattens the entries into every form that would be looked up.
func allSpellings(entries []entry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.spellings...)
	}
	return out
}

// One entry is ONE thing, however many spellings it is tried under.
//
// The counts are what a decision to apply gets made on, and offering two
// spellings as two lookups reported 321 files across packs holding 259 — and
// listed the same unresolved episode twice, once with its extension and once
// without.
func TestEntryIsCountedOnceNotPerSpelling(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "Show.S01E01.1080p.WEB-DL-GRP.mkv"), "a")

	got, err := packFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if n := len(got[0].spellings); n != 2 {
		t.Errorf("got %d spellings, want both forms tried", n)
	}
	if got[0].name != "Show.S01E01.1080p.WEB-DL-GRP.mkv" {
		t.Errorf("name = %q, want the form as it sits on disk", got[0].name)
	}
}

// resolve tries the spellings in order and reports which one matched, so a
// recorded association names the string it was actually made on.
func TestResolveReportsTheMatchedSpelling(t *testing.T) {
	byName := map[string]string{"Show.S01E01.1080p.WEB-DL-GRP": "abc123"}
	e := entry{
		name:      "Show.S01E01.1080p.WEB-DL-GRP.mkv",
		spellings: []string{"Show.S01E01.1080p.WEB-DL-GRP.mkv", "Show.S01E01.1080p.WEB-DL-GRP"},
	}

	hash, matched, ok := resolve(byName, e)
	if !ok {
		t.Fatal("did not resolve")
	}
	if hash != "abc123" {
		t.Errorf("hash = %q", hash)
	}
	if matched != "Show.S01E01.1080p.WEB-DL-GRP" {
		t.Errorf("matched = %q, want the extensionless form", matched)
	}
}

// Nothing matching resolves to nothing — no partial, no nearest.
func TestResolveMissIsAMiss(t *testing.T) {
	if _, _, ok := resolve(map[string]string{"Other": "x"}, entry{
		name:      "Show.S01E01.mkv",
		spellings: []string{"Show.S01E01.mkv", "Show.S01E01"},
	}); ok {
		t.Error("resolved something it should not have")
	}
}
