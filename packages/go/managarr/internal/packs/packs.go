// Package packs records which episode torrents a season pack was built from.
//
// seasonpackarr assembles a pack by hardlinking episodes already in the client
// into a pack directory, after which the pack is added as an ordinary torrent.
// Nothing then records what went into it: a pack is indistinguishable from
// something downloaded, and the episodes behind it are indistinguishable from
// episodes nothing depends on.
//
// That matters for deletion. A pack and its episodes are the same bytes under
// two names, so acting on one while the other still seeds breaks the survivor.
//
// TWO SOURCES, and neither is an infohash:
//
//	pack contents  the files inside the pack directory, named after the
//	               episodes they were linked from. Describes what is on disk
//	               NOW, which is what deletion actually cares about.
//	the log        seasonpackarr's own statements. Recorded by the ingest, not
//	               here; weaker, because it is an intention at a moment.
//
// Both resolve through client_torrents.name — the name qBittorrent gave the
// torrent, which is the identifier seasonpackarr quotes. This is string
// equality on a name, deliberately, and it is the only route available:
// seasonpackarr emits no infohash in its log, its notifier or its API, and its
// two routes are POST-only decision endpoints with nothing to read back.
//
// The join is exact rather than fuzzy — no normalisation, no similarity, no
// stripping of extensions beyond the one the pack directory adds. A name that
// does not match resolves to nothing and is reported, which is the honest
// outcome for an identifier that has drifted.
package packs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
	"homelab/managarr/internal/query"
)

// Config says where packs live.
type Config struct {
	// ClientName scopes the torrents considered.
	ClientName string
	// PacksDir is seasonpackarr's preImportPath. A torrent whose content sits
	// under it is an assembled pack.
	PacksDir string
}

// Resolver links packs to their sources.
type Resolver struct {
	q      *query.Query
	dao    *dao.DAO
	logger *slog.Logger
	cfg    Config
}

// New builds a resolver.
func New(db *gorm.DB, d *dao.DAO, logger *slog.Logger, cfg Config) *Resolver {
	return &Resolver{q: query.Use(db), dao: d, logger: logger, cfg: cfg}
}

// Stats reports what a run did.
type Stats struct {
	// Packs is how many assembled packs were examined.
	Packs int
	// Files is how many files were found inside them.
	Files int
	// Linked is how many resolved to a client torrent.
	Linked int
	// Unresolved is how many did not. Worth surfacing rather than counting
	// silently: each one is an episode this cannot account for, and the cause
	// is a name that no longer matches — not an absence of data.
	Unresolved int
	// Sample shows a few unresolved names, so the run can be diagnosed rather
	// than merely distrusted.
	Unmatched []string `json:",omitempty"`
	DryRun    bool
	Took      time.Duration
}

// maxUnmatched bounds the diagnostic sample.
const maxUnmatched = 25

// Run resolves every assembled pack's contents to the torrents behind them.
func (r *Resolver) Run(ctx context.Context, dryRun bool) (Stats, error) {
	st := Stats{DryRun: dryRun}
	started := time.Now()

	if r.cfg.PacksDir == "" {
		st.Took = time.Since(started)
		return st, nil
	}

	packs, err := r.assembledPacks(ctx)
	if err != nil {
		return st, err
	}
	st.Packs = len(packs)
	if len(packs) == 0 {
		st.Took = time.Since(started)
		return st, nil
	}

	byName, err := r.torrentsByName(ctx)
	if err != nil {
		return st, err
	}

	now := time.Now().UTC()
	var rows []model.PackSource

	for _, p := range packs {
		names, err := packFiles(p.ContentPath)
		if err != nil {
			// A pack whose directory has gone is not an error for this pass —
			// the reap sweep is what reports missing data.
			r.logger.WarnContext(ctx, "pack contents unreadable",
				"infohash", p.Infohash, "path", p.ContentPath, "err", err)
			continue
		}

		for _, entry := range names {
			// One count per ENTRY, not per spelling. Each entry is tried under
			// both the name it has and the name without its extension, and
			// counting those separately reported 321 files across 21 packs
			// holding 259 — and listed the same unresolved episode twice.
			st.Files++
			source, matched, ok := resolve(byName, entry)
			if !ok {
				st.Unresolved++
				if len(st.Unmatched) < maxUnmatched {
					st.Unmatched = append(st.Unmatched, entry.name)
				}
				continue
			}
			name := matched
			// A pack whose own name matches one of its files would otherwise
			// claim itself as a source.
			if source == p.Infohash {
				continue
			}
			st.Linked++
			rows = append(rows, model.PackSource{
				PackInfohash:   p.Infohash,
				SourceInfohash: source,
				Via:            model.SourceViaPackContents,
				MatchedName:    name,
				ObservedAt:     now,
			})
		}
	}

	if dryRun || len(rows) == 0 {
		st.Took = time.Since(started)
		return st, nil
	}

	// Upserted rather than replaced: the log path writes rows for the same
	// pairs from the other direction, and a pass that found the file this time
	// should not delete what a previous one established.
	if err := dao.Upsert(ctx, r.dao, rows,
		[]string{"pack_infohash", "source_infohash"},
		[]string{"via", "matched_name", "observed_at"},
	); err != nil {
		return st, fmt.Errorf("packs: record sources: %w", err)
	}

	st.Took = time.Since(started)
	return st, nil
}

// pack is one assembled season pack.
type pack struct {
	Infohash    string
	ContentPath string
}

// assembledPacks is every live torrent whose content sits under PacksDir.
func (r *Resolver) assembledPacks(ctx context.Context) ([]pack, error) {
	ct := r.q.ClientTorrent
	rows, err := ct.WithContext(ctx).
		Select(ct.TorrentInfohash, ct.ContentPath).
		Where(
			ct.ClientName.Eq(r.cfg.ClientName),
			ct.RemovedAt.IsNull(),
			ct.ContentPath.Like(strings.TrimSuffix(r.cfg.PacksDir, "/")+"/%"),
		).
		Find()
	if err != nil {
		return nil, fmt.Errorf("packs: assembled packs: %w", err)
	}

	out := make([]pack, 0, len(rows))
	for _, row := range rows {
		if row.ContentPath == nil {
			continue
		}
		out = append(out, pack{Infohash: row.TorrentInfohash, ContentPath: *row.ContentPath})
	}
	return out, nil
}

// torrentsByName indexes live torrents by the client's name for them.
//
// A name is not unique in principle — two torrents can carry the same one — so
// a collision keeps the FIRST and is not reported. That is a deliberate
// tolerance: two torrents with one name are the same release from two trackers,
// which is exactly the cross-seed case, and either is an equally true answer to
// "what provides these bytes".
func (r *Resolver) torrentsByName(ctx context.Context) (map[string]string, error) {
	ct := r.q.ClientTorrent
	rows, err := ct.WithContext(ctx).
		Select(ct.TorrentInfohash, ct.Name).
		Where(
			ct.ClientName.Eq(r.cfg.ClientName),
			ct.RemovedAt.IsNull(),
			ct.Name.IsNotNull(),
		).
		Find()
	if err != nil {
		return nil, fmt.Errorf("packs: torrents by name: %w", err)
	}

	out := make(map[string]string, len(rows))
	for _, row := range rows {
		if row.Name == nil {
			continue
		}
		if _, dup := out[*row.Name]; dup {
			continue
		}
		out[*row.Name] = row.TorrentInfohash
	}
	return out, nil
}

// mediaExtensions are the only suffixes treated as an extension.
//
// A WHITELIST, because release names are full of dots and filepath.Ext is not
// an oracle: it reports ".WEB-GRP" for "Show.S01E02.2160p.WEB-GRP" and
// ".264-FLUX" for an H.264 release. Stripping those invents a name that was
// never a torrent's, and an invented name is exactly what must not be offered
// to an exact-match lookup — it could resolve to something real and attach a
// pack to the wrong episode.
var mediaExtensions = map[string]struct{}{
	".mkv": {}, ".mp4": {}, ".avi": {}, ".m4v": {}, ".ts": {},
	".mov": {}, ".wmv": {}, ".flv": {}, ".mpg": {}, ".mpeg": {},
	".srt": {}, ".ass": {}, ".sub": {}, ".idx": {}, ".nfo": {},
}

// entry is one thing inside a pack, with the spellings it might be known by.
//
// A pack is laid out one of two ways and both occur here. Flat, where each
// episode is a file — Reacher.S04/Reacher.S04E01....mkv — and scene-style,
// where each episode is a DIRECTORY containing the media plus Proof and Sample
// subdirectories. The depth-one entry is named after the episode release
// either way, which is why one walk handles both.
type entry struct {
	// name is what the entry is called, and what an unresolved report shows.
	name string
	// spellings are the forms to try, most literal first.
	spellings []string
}

// resolve finds the torrent behind an entry, and which spelling matched.
func resolve(byName map[string]string, e entry) (infohash, matched string, ok bool) {
	for _, s := range e.spellings {
		if h, found := byName[s]; found {
			return h, s, true
		}
	}
	return "", "", false
}

// packFiles lists what is inside a pack, as the episodes were named.
//
// Each entry carries both spellings: the name as it sits in the pack, and —
// only where the suffix is a real media extension — the same without it.
// qBittorrent names a single-file torrent after the file INCLUDING its
// extension and a multi-file torrent after the folder WITHOUT one, and which of
// the two a given episode was is not knowable from here.
func packFiles(contentPath string) ([]entry, error) {
	info, err := os.Stat(contentPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		// A single-file pack has nothing to enumerate and no sources to find.
		return nil, nil
	}

	entries, err := os.ReadDir(contentPath)
	if err != nil {
		return nil, err
	}

	out := make([]entry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		spellings := []string{name}
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := mediaExtensions[ext]; ok {
			spellings = append(spellings, name[:len(name)-len(ext)])
		}
		out = append(out, entry{name: name, spellings: spellings})
	}
	return out, nil
}
