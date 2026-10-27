package reconcile

import (
	"os"
	"path/filepath"
	"testing"
)

// whipperLog is the header of a real log from this homelab's library, trimmed
// to the parts the parser reads.
//
// Verbatim rather than paraphrased, including the indentation: whipper nests
// the metadata under a "CD metadata:" block and EAC does not indent at all,
// which is why the patterns cannot anchor to a line start.
const whipperLog = `Log created by: whipper 0.10.0 (internal logger)
Log creation date: 2026-07-30T04:01:47Z

Ripping phase information:
  Drive: ASUS    SDRW-08U9M-U     (revision A114)
  Read offset correction: 6

CD metadata:
  Release:
    Artist: i-dle
    Title: We are (version B)
  CDDB Disc ID: 3b03f506
  MusicBrainz Disc ID: 1GXbgtxeFR0uLzVtMwY0XItr6Gs-
  MusicBrainz lookup URL: https://musicbrainz.org/cdtoc/attach?toc=1+6+76166&id=1GXbgtxeFR0uLzVtMwY0XItr6Gs-
  MusicBrainz Release URL: https://musicbrainz.org/release/6531D5F5-0E07-46F2-98E3-371A2E565DE7

Tracks:
  1:
    AccurateRip v1:
      Result: Found, exact match
    AccurateRip v2:
      Result: Found, exact match

AccurateRip summary: All tracks accurately ripped
`

// whippitSummary is the wrapper's aggregate log, written beside the real one
// for the same disc. It carries the same AccurateRip verdict and nothing
// identifying — which is exactly why the parser must reject it.
const whippitSummary = `==== AccurateRip summary: /tmp/rip.2csD0aXm ====
FLAC tracks : 6
AccurateRip : 1 accurate, 0 not verified

---- per-track ----
    Peak level: 0.99292
    Status: Copy OK
  AccurateRip summary: All tracks accurately ripped
  Health status: No errors occurred
====================================
`

func TestParseRipLogReadsWhipperMetadata(t *testing.T) {
	rip := parseRipLog(whipperLog)

	if got := deref(rip.MusicBrainzDiscID); got != "1GXbgtxeFR0uLzVtMwY0XItr6Gs-" {
		t.Errorf("disc id = %q", got)
	}
	if got := deref(rip.CddbDiscID); got != "3b03f506" {
		t.Errorf("cddb id = %q", got)
	}
	// Lowercased: MusicBrainz ids are case-insensitive UUIDs and beets writes
	// them lowercase, so the resolve join only works if both sides agree.
	if got := deref(rip.ReleaseMBID); got != "6531d5f5-0e07-46f2-98e3-371a2e565de7" {
		t.Errorf("release mbid = %q, want lowercase", got)
	}
	if got := deref(rip.AccurateRipStatus); got != RipAccurate {
		t.Errorf("status = %q, want %q", got, RipAccurate)
	}
}

// TestParseRipLogPrefersTheLookupURLOverTheReleaseURL checks the pattern does
// not match the wrong link.
//
// Both URLs are musicbrainz.org and both carry a long hex-ish token; only the
// release one has /release/ in the path.
func TestParseRipLogIgnoresTheLookupURL(t *testing.T) {
	const onlyLookup = `  MusicBrainz Disc ID: abc-
  MusicBrainz lookup URL: https://musicbrainz.org/cdtoc/attach?toc=1+6+76166&id=abc-
`
	if got := parseRipLog(onlyLookup).ReleaseMBID; got != nil {
		t.Errorf("release mbid = %q, want nil", *got)
	}
}

func TestAccurateRipStatus(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"whipper all accurate", "AccurateRip summary: All tracks accurately ripped", RipAccurate},
		{"whipper per track", "Result: Found, exact match", RipAccurate},
		{"eac", "Track 1  accurately ripped  (confidence 4)", RipAccurate},
		{"whipper none present", "AccurateRip summary: None of the tracks are present", RipNotInDatabase},
		{"whipper per track absent", "Track not present in database", RipNotInDatabase},
		{"eac unverifiable", "Track 1  cannot be verified as accurate", RipNoMatch},
		{"whipper mismatch", "Result: Found, no match", RipNoMatch},
		{"silent", "Log created by: whipper 0.10.0", RipUnverified},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := accurateRipStatus(c.text); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestAccurateRipStatusPrefersFailure pins the precedence.
//
// A disc where most tracks matched and one did not is not an accurate rip. If
// "accurate" won, a single corrupted track would be invisible — which is the
// entire thing AccurateRip exists to tell you.
func TestAccurateRipStatusPrefersFailure(t *testing.T) {
	const mixed = `
	  Track 1: Result: Found, exact match
	  Track 2: Result: Found, no match
	  AccurateRip summary: All tracks accurately ripped
	`
	if got := accurateRipStatus(mixed); got != RipNoMatch {
		t.Errorf("got %q, want %q", got, RipNoMatch)
	}
}

// TestStripUTF16 covers EAC, which writes UTF-16LE with a BOM.
func TestStripUTF16(t *testing.T) {
	utf8In := []byte("MusicBrainz Disc ID: abc")
	if got := stripUTF16(utf8In); got != string(utf8In) {
		t.Errorf("UTF-8 was mangled: %q", got)
	}

	utf16 := []byte{0xFF, 0xFE}
	for _, b := range []byte("CDDB Disc ID: 3b03f506") {
		utf16 = append(utf16, b, 0x00)
	}
	if got := stripUTF16(utf16); got != "CDDB Disc ID: 3b03f506" {
		t.Errorf("UTF-16 decode = %q", got)
	}

	// A truncated file must not panic.
	if got := stripUTF16([]byte{0xFF}); got != "\xff" {
		t.Errorf("one byte = %q", got)
	}
}

// TestReadLogRejectsAggregates is the bug this classification exists to stop.
//
// Each rip directory holds two .log files. Counting both would double every
// rip in the table, and the duplicate would have no disc id to resolve it.
func TestReadLogRejectsAggregates(t *testing.T) {
	dir := t.TempDir()
	album := filepath.Join(dir, "artist", "album")
	if err := os.MkdirAll(album, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) string {
		p := filepath.Join(album, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	whipper := write("artist - album.log", whipperLog)
	agg := write("whippit-summary.log", whippitSummary)

	r := &Rips{}
	if _, ok, err := r.readLog(agg); err != nil || ok {
		t.Fatalf("aggregate accepted: ok=%v err=%v", ok, err)
	}
	rip, ok, err := r.readLog(whipper)
	if err != nil || !ok {
		t.Fatalf("real log rejected: ok=%v err=%v", ok, err)
	}
	// The directory, not the file: the rip is the disc.
	if rip.SourcePath != album {
		t.Errorf("source path = %q, want %q", rip.SourcePath, album)
	}
	if rip.RippedAt == nil {
		t.Error("ripped_at not taken from the log's mtime")
	}
}
