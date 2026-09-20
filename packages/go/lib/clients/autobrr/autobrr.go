// Package autobrr talks to an autobrr instance's HTTP API.
//
// autobrr is the only component in this stack that sees IRC announces, and it
// keeps every one it has ever parsed — including the ones it rejected, which
// exist nowhere else. Its /api/release listing is therefore the whole record of
// what was offered versus what was taken.
//
// Authentication is a static token in the X-API-Token header. autobrr also
// accepts a session cookie for the web UI; that path is not modelled.
//
// https://github.com/autobrr/autobrr
package autobrr

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"homelab/httpx"
)

const (
	pathReleases = "/api/release"
	pathFilters  = "/api/filters"
	pathIRC      = "/api/irc"
	pathIndexers = "/api/indexer"
)

// Client is an autobrr API client.
type Client struct {
	http *httpx.Client
}

// New builds a client for the instance at cfg.BaseURL.
func New(cfg httpx.Config) (*Client, error) {
	c, err := httpx.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{http: c}, nil
}

// Indexer is one configured indexer.
type Indexer struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Identifier         string `json:"identifier"`
	IdentifierExternal string `json:"identifier_external"`
	Implementation     string `json:"implementation"`
	Enabled            bool   `json:"enabled"`
	Privacy            string `json:"privacy"`
	Protocol           string `json:"protocol"`
	Description        string `json:"description"`
}

// Filter is one configured filter.
//
// Indexers is many: a filter can apply across several trackers, which is why
// the relationship cannot be carried as a column on either side.
type Filter struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Enabled      bool      `json:"enabled"`
	Priority     int       `json:"priority"`
	ActionsCount int       `json:"actions_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Indexers     []Indexer `json:"indexers"`
}

// Network is one IRC network and the channels autobrr joins on it.
type Network struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Enabled  bool      `json:"enabled"`
	Server   string    `json:"server"`
	Port     int       `json:"port"`
	TLS      bool      `json:"tls"`
	Nick     string    `json:"nick"`
	Channels []Channel `json:"channels"`
}

// Channel is one joined channel.
type Channel struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	Monitoring bool   `json:"monitoring"`
	// LastAnnounce is zero on a channel that has not announced since autobrr
	// started, which is why it is a pointer-free time compared with IsZero
	// rather than trusted as a fact.
	LastAnnounce time.Time `json:"last_announce"`
}

// Release is one parsed announce and what became of it.
type Release struct {
	ID             int64     `json:"id"`
	FilterStatus   string    `json:"filter_status"`
	Rejections     []string  `json:"rejections"`
	Indexer        Indexer   `json:"indexer"`
	FilterName     string    `json:"filter"`
	Protocol       string    `json:"protocol"`
	Implementation string    `json:"implementation"`
	Timestamp      time.Time `json:"timestamp"`
	AnnounceType   string    `json:"announce_type"`
	InfoURL        string    `json:"info_url"`
	DownloadURL    string    `json:"download_url"`
	Name           string    `json:"name"`
	// TorrentID is the TRACKER's own id for the torrent, not autobrr's.
	//
	// The only field autobrr both serialises in its API and exposes as a
	// template macro, which makes (indexer, torrent_id) the one key by which an
	// announce can be identified from both sides. Everything else the API sends
	// is absent from the macros, and the infohash is the reverse — it is a
	// macro and is never serialised.
	//
	// Empty when the indexer's definition does not parse one.
	TorrentID string `json:"torrent_id"`
	GroupID   string `json:"group_id"`
	// NormalizedHash is autobrr's dedupe digest over the parsed release. It is
	// not an infohash and must not be used as one.
	NormalizedHash string   `json:"normalized_hash"`
	Size           int64    `json:"size"`
	Title          string   `json:"title"`
	SubTitle       string   `json:"sub_title"`
	Category       string   `json:"category"`
	Season         int      `json:"season"`
	Episode        int      `json:"episode"`
	Year           int      `json:"year"`
	Resolution     string   `json:"resolution"`
	Source         string   `json:"source"`
	Codec          []string `json:"codec"`
	Container      string   `json:"container"`
	HDR            []string `json:"hdr"`
	Group          string   `json:"group"`
	// Edition and Cut are ARRAYS, like Codec and HDR — a release can carry
	// several of each. autobrr stores them joined in its own database, which is
	// what made a string look right here; the API serialises the raw slice.
	Edition      []string `json:"edition"`
	Cut          []string `json:"cut"`
	Proper       bool     `json:"proper"`
	Repack       bool     `json:"repack"`
	Website      string   `json:"website"`
	Uploader     string   `json:"uploader"`
	Origin       string   `json:"origin"`
	PreTime      string   `json:"pre_time"`
	ActionStatus []Action `json:"action_status"`
}

// Action is one action's outcome for a release.
//
// Status is PUSH_APPROVED, PUSH_REJECTED or PUSH_ERROR. The three are not
// interchangeable: REJECTED means the target refused the release (unknown
// series, already grabbed), ERROR means the push itself failed, which is a
// fault in this stack rather than a decision about the release.
type Action struct {
	// ID identifies this outcome — one row per release per action.
	ID int64 `json:"id"`
	// ActionID identifies the CONFIGURED action that produced it, and is
	// stable across every release. Distinct from ID, and the only field that
	// joins an outcome back to the action definition on a filter.
	ActionID   int       `json:"action_id"`
	Status     string    `json:"status"`
	Action     string    `json:"action"`
	Type       string    `json:"type"`
	Client     string    `json:"client"`
	FilterID   int       `json:"filter_id"`
	Rejections []string  `json:"rejections"`
	ReleaseID  int64     `json:"release_id"`
	Timestamp  time.Time `json:"timestamp"`
}

// releasePage is one page of releases.
type releasePage struct {
	Data []Release `json:"data"`
	// NextCursor is reported by autobrr but is not usable: passing it back as
	// ?cursor= returns an empty page, including when it is the exact value
	// autobrr just supplied. Verified against 1.82.1. Paging is by ?offset=.
	NextCursor int64 `json:"next_cursor"`
	// Count is the total number of releases, not the size of this page.
	Count int64 `json:"count"`
}

// Indexers returns every configured indexer.
func (c *Client) Indexers(ctx context.Context) ([]Indexer, error) {
	var out []Indexer
	if err := c.http.Do(ctx, http.MethodGet, pathIndexers, nil, &out); err != nil {
		return nil, fmt.Errorf("autobrr: list indexers: %w", err)
	}
	return out, nil
}

// Filters returns every configured filter with its indexers.
func (c *Client) Filters(ctx context.Context) ([]Filter, error) {
	var out []Filter
	if err := c.http.Do(ctx, http.MethodGet, pathFilters, nil, &out); err != nil {
		return nil, fmt.Errorf("autobrr: list filters: %w", err)
	}
	return out, nil
}

// Networks returns every IRC network with its channels.
func (c *Client) Networks(ctx context.Context) ([]Network, error) {
	var out []Network
	if err := c.http.Do(ctx, http.MethodGet, pathIRC, nil, &out); err != nil {
		return nil, fmt.Errorf("autobrr: list irc networks: %w", err)
	}
	return out, nil
}

// ReleasePage is one ascending page of releases plus autobrr's total.
type ReleasePage struct {
	Records []Release
	// Total is every release autobrr holds, not the size of this page.
	Total int64
}

// ReleasesPageAsc returns releases OLDEST first, starting at an ascending
// offset.
//
// autobrr serves this list newest-first and pages it by ?offset=, which makes a
// descending offset useless as a cursor: a release arriving mid-walk shifts
// every later page down by one and a row is skipped. An ASCENDING offset does
// not move — new releases land at the far end of the list from where counting
// starts — so it survives exactly the writes a descending offset does not.
//
// The cost is a probe request to learn the current total before the ascending
// position can be turned into autobrr's descending one. Two requests per page,
// which is the price of a cursor on an API that offers none. autobrr does
// report a next_cursor, but passing it back as ?cursor= returns an empty page
// even when it is the value autobrr just supplied — verified against 1.82.1.
func (c *Client) ReleasesPageAsc(ctx context.Context, offset, limit int) (ReleasePage, error) {
	if limit <= 0 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	var probe releasePage
	probePath := fmt.Sprintf("%s?limit=1&offset=0", pathReleases)
	if err := c.http.Do(ctx, http.MethodGet, probePath, nil, &probe); err != nil {
		return ReleasePage{}, fmt.Errorf("autobrr: release count: %w", err)
	}
	total := probe.Count
	if int64(offset) >= total {
		return ReleasePage{Total: total}, nil
	}

	// Ascending [offset, offset+limit) is descending
	// [total-offset-limit, total-offset). Clamped at the start of the list,
	// which is where the OLDEST releases are.
	descOffset := total - int64(offset) - int64(limit)
	want := limit
	if descOffset < 0 {
		want = limit + int(descOffset)
		descOffset = 0
	}

	var page releasePage
	path := fmt.Sprintf("%s?limit=%d&offset=%d", pathReleases, want, descOffset)
	if err := c.http.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
		return ReleasePage{}, fmt.Errorf("autobrr: releases at ascending %d: %w", offset, err)
	}

	// autobrr sent them newest-first; the caller asked for oldest-first.
	records := make([]Release, 0, len(page.Data))
	for i := len(page.Data) - 1; i >= 0; i-- {
		records = append(records, page.Data[i])
	}
	return ReleasePage{Records: records, Total: total}, nil
}

// Releases walks the release history newest-first, calling fn for each page.
//
// The corpus is large — six figures — so it is streamed by page rather than
// accumulated: holding every release in memory to return one slice would cost
// hundreds of megabytes for a caller that writes each row and forgets it.
//
// fn returning false stops the walk, which is how an incremental pass ends
// once it reaches releases it has already recorded. Ordering is newest-first,
// so that stop condition is reached early on a pass that has little new work.
//
// Paging is by offset. That is a real weakness for a walk this long: releases
// arriving mid-walk shift every later page by one and a row can be skipped.
// The caller must therefore treat the walk as best-effort per pass and rely on
// its own idempotent upsert plus a later pass to close gaps, rather than
// assuming one completed walk saw everything.
func (c *Client) Releases(ctx context.Context, pageSize int, fn func([]Release) bool) error {
	if pageSize <= 0 {
		pageSize = 1000
	}
	var offset int
	for {
		path := fmt.Sprintf("%s?limit=%d&offset=%d", pathReleases, pageSize, offset)

		var page releasePage
		if err := c.http.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
			return fmt.Errorf("autobrr: list releases at offset %d: %w", offset, err)
		}
		if len(page.Data) == 0 {
			return nil
		}
		if !fn(page.Data) {
			return nil
		}
		// A short page is the last one.
		if len(page.Data) < pageSize {
			return nil
		}
		offset += len(page.Data)
	}
}
