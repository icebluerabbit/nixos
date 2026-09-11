// Package slskd talks to a slskd instance's HTTP API.
//
// Only the search-history surface is modelled so far, because that is what has
// a consumer. slskd stores every search it has ever run in search.db and
// exposes retention settings for transfers, files, logs and the share cache —
// but none for searches, so the history is unbounded and the /api/v0/searches
// listing that the web UI loads grows with it. Pruning therefore has to be done
// from outside, one DELETE per search.
//
// Authentication is not modelled either: this homelab disables slskd's own
// login and gates the UI at Caddy, so the API is reached on loopback with no
// token. Add a bearer header through httpx.Config when that stops being true.
//
// https://github.com/slskd/slskd
package slskd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"homelab/httpx"
)

// pathSearches is the search-history collection. A single search is addressed
// as pathSearches + "/" + id.
const pathSearches = "/api/v0/searches"

// Client is a slskd API client.
type Client struct {
	http *httpx.Client
}

// ErrNoSearchID reports a call made with an empty search id.
var ErrNoSearchID = errors.New("slskd: search id is required")

// Search is one entry of the stored search history.
//
// The API returns more fields than these — notably a responses array, which is
// empty on the collection endpoint and megabytes long on the detail one. Only
// what a caller can act on is decoded; the rest is discarded.
type Search struct {
	// ID is the search's UUID, and what DeleteSearch takes.
	ID string `json:"id"`
	// SearchText is the query as typed, useful only for diagnostics.
	SearchText string `json:"searchText"`
	// StartedAt is when the search was issued. slskd sends RFC 3339 with
	// seven fractional digits, which time.Time parses as-is.
	StartedAt time.Time `json:"startedAt"`
	// EndedAt is when it completed, zero while it is still running.
	EndedAt time.Time `json:"endedAt"`
	// IsComplete reports whether the search has finished.
	IsComplete bool `json:"isComplete"`
	// FileCount is how many files the responses carried.
	FileCount int `json:"fileCount"`
	// ResponseCount is how many peers answered.
	ResponseCount int `json:"responseCount"`
	// State is slskd's own label, e.g. "Completed, ResponseLimitReached".
	State string `json:"state"`
}

// New builds a client for the instance at cfg.BaseURL.
func New(cfg httpx.Config) (*Client, error) {
	c, err := httpx.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{http: c}, nil
}

// Searches returns the whole stored search history.
//
// There is no server-side paging or filtering on this endpoint: slskd answers
// with every search it holds, which is precisely the problem a caller is
// usually here to fix. Expect the response to be large.
func (c *Client) Searches(ctx context.Context) ([]Search, error) {
	var out []Search
	if err := c.http.Do(ctx, http.MethodGet, pathSearches, nil, &out); err != nil {
		return nil, fmt.Errorf("slskd: list searches: %w", err)
	}
	return out, nil
}

// DeleteSearch removes one stored search by id.
//
// Deleting frees pages inside search.db for reuse but does not shrink the file;
// that needs a VACUUM, which cannot be done while slskd holds the database
// open.
func (c *Client) DeleteSearch(ctx context.Context, id string) error {
	if id == "" {
		return ErrNoSearchID
	}
	// The id goes in unescaped: httpx builds the request URL through
	// url.URL.Path, which encodes it once. Pre-escaping would double-encode.
	if err := c.http.Do(ctx, http.MethodDelete, pathSearches+"/"+id, nil, nil); err != nil {
		return fmt.Errorf("slskd: delete search %s: %w", id, err)
	}
	return nil
}
