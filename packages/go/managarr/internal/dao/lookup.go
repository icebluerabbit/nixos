package dao

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"homelab/managarr/internal/model"
)

// Lookups resolves dimension rows to keys, with a per-pass cache.
//
// Every pass resolves the same handful of trackers, indexers and clients
// thousands of times — once per torrent, once per release. Without a cache that
// is a round trip per row, which is most of what made the passes slow.
//
// The cache is reset per pass by Reset and must NOT outlive one: a miss is
// expected to become a hit once another pass has created the row, and caching
// misses forever is what defeated the content backfill in an earlier version.
type Lookups struct {
	dao *DAO

	mu sync.Mutex
	// content caches "provider:value" to a content id, including misses as the
	// zero UUID.
	content map[string]model.ID
	// seen records dimension rows already upserted this pass, so the same
	// tracker or indexer is not re-written per torrent.
	seen map[string]struct{}
}

// NewLookups builds a resolver.
func NewLookups(d *DAO) *Lookups {
	l := &Lookups{dao: d}
	l.Reset()
	return l
}

// Reset clears the cache. Call at the start of every pass.
func (l *Lookups) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.content = map[string]model.ID{}
	l.seen = map[string]struct{}{}
}

// once reports whether key has already been handled this pass.
func (l *Lookups) once(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.seen[key]; ok {
		return true
	}
	l.seen[key] = struct{}{}
	return false
}

// Tracker ensures a tracker row exists and returns its key.
//
// Empty host yields nil: DHT, PeX and LSD appear as tracker entries with no
// host, and so does a torrent with no working tracker. A junk row for each
// would pollute the table that answers "which trackers do I use".
func (l *Lookups) Tracker(ctx context.Context, host string) (*string, error) {
	if host == "" {
		return nil, nil
	}
	if !l.once("tracker:" + host) {
		if err := Insert(ctx, l.dao, []model.Tracker{{AnnounceHost: host}},
			[]string{"announce_host"}); err != nil {
			return nil, fmt.Errorf("dao: tracker %q: %w", host, err)
		}
	}
	return &host, nil
}

// Indexer ensures an indexer row exists and returns its key.
//
// Only the name is written. Every column beyond it is owned by the pass that
// authoritatively knows it — autobrr owns identifier and enabled, Prowlarr owns
// protocol and privacy — so this never overwrites their work.
func (l *Lookups) Indexer(ctx context.Context, name string) (*string, error) {
	if name == "" {
		return nil, nil
	}
	if !l.once("indexer:" + name) {
		if err := Insert(ctx, l.dao, []model.Indexer{{Name: name}},
			[]string{"name"}); err != nil {
			return nil, fmt.Errorf("dao: indexer %q: %w", name, err)
		}
	}
	return &name, nil
}

// Client ensures a download client row exists and returns its key.
func (l *Lookups) Client(ctx context.Context, name, kind string) (string, error) {
	if !l.once("client:" + name) {
		if err := Upsert(ctx, l.dao, []model.Client{{Name: name, Kind: kind}},
			[]string{"name"}, []string{"kind"}); err != nil {
			return "", fmt.Errorf("dao: client %q: %w", name, err)
		}
	}
	return name, nil
}

// Service ensures a service row exists and returns its key.
func (l *Lookups) Service(ctx context.Context, name, kind string) (string, error) {
	if !l.once("service:" + name) {
		if err := Upsert(ctx, l.dao, []model.Service{{Name: name, Kind: kind}},
			[]string{"name"}, []string{"kind"}); err != nil {
			return "", fmt.Errorf("dao: service %q: %w", name, err)
		}
	}
	return name, nil
}

// SoulseekPeer ensures a peer row exists and returns its key.
func (l *Lookups) SoulseekPeer(ctx context.Context, username string) (*string, error) {
	if username == "" {
		return nil, nil
	}
	if !l.once("peer:" + username) {
		if err := Insert(ctx, l.dao, []model.SoulseekPeer{{Username: username}},
			[]string{"username"}); err != nil {
			return nil, fmt.Errorf("dao: soulseek peer %q: %w", username, err)
		}
	}
	return &username, nil
}

// Quality ensures a quality row exists and returns its key.
func (l *Lookups) Quality(ctx context.Context, name string) (*string, error) {
	if name == "" {
		return nil, nil
	}
	if !l.once("quality:" + name) {
		if err := Insert(ctx, l.dao, []model.Quality{{Name: name}},
			[]string{"name"}); err != nil {
			return nil, fmt.Errorf("dao: quality %q: %w", name, err)
		}
	}
	return &name, nil
}

// Language ensures a language row exists and returns its key.
func (l *Lookups) Language(ctx context.Context, name string) (*string, error) {
	if name == "" {
		return nil, nil
	}
	if !l.once("language:" + name) {
		if err := Insert(ctx, l.dao, []model.Language{{Name: name}},
			[]string{"name"}); err != nil {
			return nil, fmt.Errorf("dao: language %q: %w", name, err)
		}
	}
	return &name, nil
}

// Torrent ensures a torrent row exists for a hash and returns it normalised.
//
// Called before anything that references a torrent, because the FK is enforced
// and facts about a torrent routinely arrive before the client reports it —
// autobrr's webhook fires seconds after a grab, the snapshot runs hourly. A
// hash from a trustworthy source IS evidence the torrent exists, so recording
// it is honest rather than a placeholder.
func (l *Lookups) Torrent(ctx context.Context, hash string) (*string, error) {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if hash == "" {
		return nil, nil
	}
	if !l.once("torrent:" + hash) {
		if err := Insert(ctx, l.dao, []model.Torrent{{Infohash: hash}},
			[]string{"infohash"}); err != nil {
			return nil, fmt.Errorf("dao: torrent %s: %w", hash, err)
		}
	}
	return &hash, nil
}

// Release finds or creates a release by title, returning the STORED id.
//
// Title is UNIQUE, so this is an upsert — but the id it returns has to come
// back out of the database rather than out of the struct that went in.
// BeforeCreate mints an id on every call, and on a conflict that id is
// discarded by Postgres while the Go struct keeps it. Returning it produced an
// id that matched no row, and every foreign key built from it would have been
// pointed at nothing.
//
// clause.Returning is what closes that: the upsert reports the row that now
// exists, whether it was inserted or already there.
//
// The release is the hinge every other table joins through, and creating it on
// first mention is what lets an announce and a torrent for the same release
// meet without either knowing about the other.
func (l *Lookups) Release(ctx context.Context, title string) (*model.ID, error) {
	if title == "" {
		return nil, nil
	}
	row := model.Release{Title: title}
	err := l.dao.db.WithContext(ctx).
		Clauses(
			clause.OnConflict{
				Columns: []clause.Column{{Name: "title"}},
				// A no-op assignment rather than DoNothing: DO NOTHING produces
				// no row for RETURNING to report, so the id would come back
				// zero for every release that already existed.
				DoUpdates: clause.AssignmentColumns([]string{"title"}),
			},
			clause.Returning{Columns: []clause.Column{{Name: "id"}}},
		).
		Create(&row).Error
	if err != nil {
		return nil, fmt.Errorf("dao: release %q: %w", title, err)
	}
	return &row.ID, nil
}

// ContentByExternalID resolves external ids to a content row.
//
// Tried in a fixed order so the answer does not depend on map iteration: tvdb
// is most specific for series, tmdb for film, musicbrainz for music, imdb spans
// film and television and is weakest.
//
// Returns nil when nothing matches, which is normal — searches and
// announcements are recorded for content no *arr tracks.
func (l *Lookups) ContentByExternalID(ctx context.Context, ids map[string]string) (*model.ID, error) {
	for _, provider := range [...]string{
		model.ProviderTVDB, model.ProviderTMDB,
		model.ProviderMusicBrainz, model.ProviderIMDB,
	} {
		value := ids[provider]
		if value == "" || value == "0" {
			continue
		}
		key := provider + ":" + value

		l.mu.Lock()
		cached, seen := l.content[key]
		l.mu.Unlock()
		if seen {
			if cached != (model.ID{}) {
				return &cached, nil
			}
			continue
		}

		var row model.ContentExternalID
		err := l.dao.db.WithContext(ctx).
			Where("provider = ? AND value = ?", provider, value).
			First(&row).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("dao: resolve %s: %w", key, err)
			}
			// Cached as a miss for THIS PASS only; Reset clears it, because a
			// miss is expected to resolve once the *arr pass has run.
			l.mu.Lock()
			l.content[key] = model.ID{}
			l.mu.Unlock()
			continue
		}

		l.mu.Lock()
		l.content[key] = row.ContentID
		l.mu.Unlock()
		return &row.ContentID, nil
	}
	return nil, nil
}

// ReleaseIDs resolves many titles at once, creating the ones that do not exist.
//
// The batch form of Release, and the reason it exists is round trips: a page of
// a thousand announces or cross-seed candidates resolved one at a time is a
// thousand upserts and a thousand RETURNING reads, which is most of what made a
// cold backfill wait rather than work.
//
// Two statements regardless of page size — insert what is missing, then read
// the ids back. Insert rather than upsert because a release row keyed by title
// has nothing to update: the parsed columns belong to whoever parsed them, and
// autobrr writes those separately.
func (l *Lookups) ReleaseIDs(ctx context.Context, titles []string) (map[string]model.ID, error) {
	unique := make([]string, 0, len(titles))
	seen := make(map[string]struct{}, len(titles))
	for _, t := range titles {
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		unique = append(unique, t)
	}
	if len(unique) == 0 {
		return map[string]model.ID{}, nil
	}

	rows := make([]model.Release, 0, len(unique))
	for _, t := range unique {
		rows = append(rows, model.Release{Title: t})
	}
	if err := Insert(ctx, l.dao, rows, []string{"title"}); err != nil {
		return nil, fmt.Errorf("dao: releases: %w", err)
	}

	var stored []model.Release
	if err := l.dao.db.WithContext(ctx).
		Select("id", "title").
		Where("title IN ?", unique).
		Find(&stored).Error; err != nil {
		return nil, fmt.Errorf("dao: release ids: %w", err)
	}

	out := make(map[string]model.ID, len(stored))
	for _, r := range stored {
		out[r.Title] = r.ID
	}
	return out, nil
}
