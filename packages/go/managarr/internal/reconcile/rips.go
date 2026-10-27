package reconcile

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// Rips records CDs ripped locally.
//
// A self-rip has no announce, no indexer, no torrent and no grab — it entered
// the library through a disc drive. Every other pass here is blind to it, so
// without this the music library has a hole exactly the size of everything that
// was not downloaded.
//
// The rip log is the whole record. whipper and EAC both write one beside the
// audio, and it carries the two things worth keeping: the MusicBrainz release
// the disc was identified as, and whether AccurateRip agreed the bits are
// right. The second is a claim about correctness that nothing else in the stack
// can make about any file.
//
// The staging directory is scanned rather than the library, because the log
// does not follow the audio: beets moves the tracks and leaves the log behind.
//
// Content is NOT resolved here. A rip log names a MusicBrainz RELEASE while
// Lidarr tracks a release GROUP; joining them needs beets, which holds both, so
// that link is made by the resolve pass alongside every other cross-service
// one rather than duplicated here.
type Rips struct {
	dao     *dao.DAO
	lookups *dao.Lookups
	logger  *slog.Logger
	dir     string
}

// NewRips builds the pass.
func NewRips(d *dao.DAO, l *dao.Lookups, logger *slog.Logger, dir string) *Rips {
	return &Rips{dao: d, lookups: l, logger: logger, dir: dir}
}

// RipStats reports what a run did.
type RipStats struct {
	Scanned  int
	Logged   int
	Accurate int
	Took     time.Duration
}

// Run scans the staging directory.
func (r *Rips) Run(ctx context.Context) (RipStats, error) {
	var st RipStats
	if r.dir == "" {
		return st, nil
	}
	started := time.Now()

	var rows []model.Rip
	err := filepath.WalkDir(r.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".log") {
			return nil
		}
		st.Scanned++

		rip, ok, err := r.readLog(path)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		st.Logged++
		if deref(rip.AccurateRipStatus) == RipAccurate {
			st.Accurate++
		}
		rows = append(rows, rip)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			// The staging directory is created by the ripping host, not by
			// this service. Its absence means nothing has been ripped.
			return st, nil
		}
		return st, fmt.Errorf("rips: walk %s: %w", r.dir, err)
	}

	if err := dao.Upsert(ctx, r.dao, rows, []string{"source_path"},
		[]string{"musicbrainz_disc_id", "release_mbid", "cddb_disc_id",
			"accurate_rip_status", "ripped_at"}); err != nil {
		return st, fmt.Errorf("rips: upsert: %w", err)
	}

	st.Took = time.Since(started)
	return st, nil
}

// readLog reads one log and reports whether it describes a rip.
//
// Not every .log in the tree is one. whipper's wrapper writes a
// whippit-summary.log beside the real log for the same disc, holding the same
// AccurateRip verdict and nothing identifying — counting both would double
// every rip. A log qualifies only if it carries a disc identifier, which the
// aggregates do not.
func (r *Rips) readLog(path string) (model.Rip, bool, error) {
	// Rip logs are a few kilobytes. whipper writes UTF-8 and EAC UTF-16, and
	// the patterns below are ASCII, so both match once the UTF-16 NUL bytes
	// are dropped.
	raw, err := os.ReadFile(path) //nolint:gosec // path comes from walking the operator-set rips dir
	if err != nil {
		return model.Rip{}, false, fmt.Errorf("rips: read %s: %w", path, err)
	}
	rip := parseRipLog(stripUTF16(raw))
	if rip.MusicBrainzDiscID == nil && rip.CddbDiscID == nil {
		return model.Rip{}, false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return model.Rip{}, false, fmt.Errorf("rips: stat %s: %w", path, err)
	}

	// The directory, not the log file: the rip is the disc, and a re-rip that
	// wrote a second log into the same folder is the same disc ripped again.
	rip.SourcePath = filepath.Dir(path)
	rip.RippedAt = timePtr(info.ModTime())
	return rip, true, nil
}

// AccurateRip verdicts this pass records.
//
// Four values rather than a free-text copy of the log, because whipper and EAC
// word the same three outcomes differently and a column that has to be read
// with an if-chain per ripper is not a column anyone will use.
const (
	// RipAccurate means AccurateRip had the disc and the checksums agreed.
	RipAccurate = "accurate"
	// RipNoMatch means AccurateRip had the disc and the checksums did not
	// agree — the strongest available evidence that a rip is bad.
	RipNoMatch = "no_match"
	// RipNotInDatabase means nobody has submitted this disc, so the rip is
	// unverified through no fault of its own.
	RipNotInDatabase = "not_in_database"
	// RipUnverified means the log said nothing about AccurateRip at all.
	RipUnverified = "unverified"
)

// Rip-log patterns.
//
// Anchored loosely on purpose: whipper indents its header under a "Ripping
// result" block and EAC does not indent at all, so a line-start anchor matches
// one ripper and not the other.
var (
	reDiscID   = regexp.MustCompile(`(?i)musicbrainz disc id\s*:?\s*(\S+)`)
	reCddbID   = regexp.MustCompile(`(?i)cddb disc id\s*:?\s*(\S+)`)
	reReleaseM = regexp.MustCompile(`musicbrainz\.org/release/([0-9a-fA-F-]{36})`)

	// Verified, in each ripper's own words. whipper prints a per-disc summary
	// ("All tracks accurately ripped") and a per-track result ("Found, exact
	// match"); EAC prints "Accurately ripped" per track.
	reRipOK = regexp.MustCompile(`(?i)(accurately ripped|found, exact match)`)
	// Present in the database, checksums disagree.
	reRipBad = regexp.MustCompile(`(?i)(cannot be verified as accurate|found, no match)`)
	// Not present at all. whipper's disc-level wording is "None of the tracks
	// are present"; the per-track wording is "Track not present in database".
	reRipAbsent = regexp.MustCompile(`(?i)(none of the tracks are present|not present in database|no accuraterip)`)
)

// parseRipLog extracts what a rip log states.
//
// Kept as a pure function of the log text so it can be exercised against real
// logs from both rippers without a filesystem.
func parseRipLog(text string) model.Rip {
	var rip model.Rip

	if m := reDiscID.FindStringSubmatch(text); m != nil {
		rip.MusicBrainzDiscID = strPtr(m[1])
	}
	if m := reCddbID.FindStringSubmatch(text); m != nil {
		rip.CddbDiscID = strPtr(m[1])
	}
	if m := reReleaseM.FindStringSubmatch(text); m != nil {
		rip.ReleaseMBID = strPtr(strings.ToLower(m[1]))
	}
	rip.AccurateRipStatus = strPtr(accurateRipStatus(text))
	return rip
}

// accurateRipStatus reduces a log's per-track results to one verdict.
//
// A disagreement outranks a match: if any track failed verification the rip is
// not accurate, whatever the rest of it did. An absent disc is reported only
// when nothing was verified either way, because whipper prints the
// "not present" line per track and a partially-submitted disc would otherwise
// read as unverified when most of it checked out.
func accurateRipStatus(text string) string {
	switch {
	case reRipBad.MatchString(text):
		return RipNoMatch
	case reRipOK.MatchString(text):
		return RipAccurate
	case reRipAbsent.MatchString(text):
		return RipNotInDatabase
	default:
		return RipUnverified
	}
}

// stripUTF16 decodes a log that may be UTF-16.
//
// EAC writes UTF-16LE with a BOM. Stripping the NUL bytes rather than decoding
// properly is enough here: the fields being matched are ASCII, and a full
// decode would pull in a dependency to read four fields out of a text file.
func stripUTF16(raw []byte) string {
	if len(raw) < 2 || raw[0] != 0xFF || raw[1] != 0xFE {
		return string(raw)
	}
	out := make([]byte, 0, len(raw)/2)
	for i := 2; i < len(raw); i += 2 {
		if raw[i] != 0 {
			out = append(out, raw[i])
		}
	}
	return string(out)
}
