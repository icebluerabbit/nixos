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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
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

// pathDownloads is the current-transfer collection, grouped by peer.
const pathDownloads = "/api/v0/transfers/downloads"

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

// Time is a timestamp that may arrive with or without a zone.
//
// slskd is not consistent with itself: /searches sends RFC 3339 with an offset,
// /transfers/downloads sends "2026-09-13T18:39:17.8220349" with none. A plain
// time.Time field decodes the first and fails the second, which aborts the
// whole listing rather than one record.
//
// A zoneless value is read as UTC, which is an assumption rather than a
// certainty — slskd runs in the container's local timezone. That is a bounded
// error on a display timestamp, where refusing to parse loses the record.
type Time struct{ time.Time }

// layouts accepted for a transfer timestamp, most specific first.
var timeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.9999999",
	"2006-01-02T15:04:05",
}

// UnmarshalJSON accepts either form, plus null and empty.
func (t *Time) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		t.Time = time.Time{}
		return nil
	}
	for _, layout := range timeLayouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("slskd: unrecognised timestamp %q", s)
}

// SearchDetail is one search together with the peer responses it drew.
//
// Only returned when responses are requested explicitly. They are large — a
// single search with a hundred responders runs to ~180 KB — which is why the
// collection endpoint omits them and why a caller should fetch detail only for
// searches it has a reason to care about.
type SearchDetail struct {
	Search
	Responses []Response `json:"responses"`
}

// Response is one peer's answer to a search.
type Response struct {
	Username    string         `json:"username"`
	FileCount   int            `json:"fileCount"`
	QueueLength int            `json:"queueLength"`
	UploadSpeed int64          `json:"uploadSpeed"`
	Files       []ResponseFile `json:"files"`
}

// ResponseFile is one file a peer offered.
//
// The audio attributes are present only when the peer supplied them, and
// Soulseek clients are inconsistent about that, so zero means "not stated"
// rather than "zero".
type ResponseFile struct {
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	Extension  string `json:"extension"`
	BitRate    int    `json:"bitRate"`
	BitDepth   int    `json:"bitDepth"`
	SampleRate int    `json:"sampleRate"`
	// Length is the track duration in seconds.
	Length   int  `json:"length"`
	IsLocked bool `json:"isLocked"`
}

// SearchDetail returns one search with its responses.
func (c *Client) SearchDetail(ctx context.Context, id string) (SearchDetail, error) {
	var out SearchDetail
	if id == "" {
		return out, ErrNoSearchID
	}
	path := pathSearches + "/" + id + "?includeResponses=true"
	if err := c.http.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return out, fmt.Errorf("slskd: search detail %s: %w", id, err)
	}
	return out, nil
}

// Download is one peer's current transfers, as slskd groups them.
//
// This is live state, not history. slskd expires completed transfers on its own
// retention schedule, so anything not in flight has already gone — a caller
// wanting a durable record has to keep its own.
type Download struct {
	Username  string          `json:"username"`
	Directory []DownloadGroup `json:"directories"`
}

// DownloadGroup is one directory within a peer's transfers.
type DownloadGroup struct {
	Directory string         `json:"directory"`
	FileCount int            `json:"fileCount"`
	Files     []DownloadFile `json:"files"`
}

// DownloadFile is a single file being fetched from a peer.
type DownloadFile struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	// Username is set on the transfer embedded in an event payload. The
	// /transfers/downloads listing nests files under the peer instead and
	// leaves this empty.
	Username string `json:"username"`
	Size     int64  `json:"size"`
	// BytesTransferred is progress; State is slskd's own label, e.g.
	// "Completed, Succeeded" or "Completed, Errored".
	BytesTransferred int64  `json:"bytesTransferred"`
	State            string `json:"state"`
	RequestedAt      Time   `json:"requestedAt"`
	StartedAt        Time   `json:"startedAt"`
	EndedAt          Time   `json:"endedAt"`
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

// Downloads returns the transfers slskd currently holds, grouped by peer.
//
// Live state only. Completed transfers age out on slskd's retention schedule,
// so a caller that wants a durable record of what was fetched from whom has to
// persist these itself and accept that anything finishing between two polls is
// only visible via slskd's own log.
func (c *Client) Downloads(ctx context.Context) ([]Download, error) {
	var out []Download
	if err := c.http.Do(ctx, http.MethodGet, pathDownloads, nil, &out); err != nil {
		return nil, fmt.Errorf("slskd: list downloads: %w", err)
	}
	return out, nil
}

// pathEvents is slskd's persisted event log.
const pathEvents = "/api/v0/events"

// Event types slskd records. Only the download completions are read here.
const (
	// EventDownloadFileComplete fires once per finished file and carries the
	// one fact no other endpoint states: where slskd PUT it.
	EventDownloadFileComplete = "DownloadFileComplete"
	// EventUploadFileComplete fires once per file a PEER took from this
	// share. Same payload shape as the download side, read in the same walk.
	EventUploadFileComplete = "UploadFileComplete"
	// EventDownloadDirectoryComplete fires once per finished folder.
	EventDownloadDirectoryComplete = "DownloadDirectoryComplete"
)

// Event is one record from the log.
//
// Data is a JSON document encoded as a STRING, not an object — slskd stores the
// serialised payload in a text column and hands it back verbatim. It therefore
// takes two decodes, which is why it is kept raw here rather than typed.
type Event struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Data      string    `json:"data"`
}

// FileComplete is the payload of a finished transfer, in either direction.
//
// The reason this log is worth reading at all. /transfers is a live view that
// expires a completed transfer, so it only ever shows what is in flight; this is
// persisted, and it is the only place slskd states the mapping between the
// remote path and the local file.
//
// The two directions read the same and mean opposite things. On a download,
// LocalFilename is where slskd PUT the file; on an upload it is the file of
// yours a peer TOOK, which for this homelab is a library path.
type FileComplete struct {
	LocalFilename  string       `json:"localFilename"`
	RemoteFilename string       `json:"remoteFilename"`
	Username       string       `json:"username"`
	Transfer       DownloadFile `json:"transfer"`
}

// Peer returns the username, preferring the transfer's own.
//
// The payload carries it at the top level on some versions and only inside the
// transfer on others.
func (d FileComplete) Peer() string {
	if d.Username != "" {
		return d.Username
	}
	return d.Transfer.Username
}

// AsFileComplete decodes the payload of a transfer-complete event, in either
// direction.
func (e Event) AsFileComplete() (FileComplete, error) {
	var out FileComplete
	if err := json.Unmarshal([]byte(e.Data), &out); err != nil {
		return out, fmt.Errorf("slskd: decode %s %s: %w", e.Type, e.ID, err)
	}
	return out, nil
}

// maxEventPages bounds a walk of the event log.
//
// The log is append-only and never trimmed by slskd, so it grows without limit;
// at the default page size this is a quarter of a million events, far past the
// ~64k this homelab has accumulated in two months.
const maxEventPages = 500

// Events walks the event log newest-first, calling fn per page.
//
// Streamed rather than returned whole: the log is tens of thousands of records
// and the full dump is measured in tens of megabytes, because every download
// event embeds its entire transfer object.
//
// fn returning false stops the walk, which is how an incremental pass ends once
// it reaches events it has already recorded.
func (c *Client) Events(ctx context.Context, pageSize int, fn func([]Event) bool) error {
	if pageSize <= 0 {
		pageSize = 1000
	}
	var offset int
	for page := 0; page < maxEventPages; page++ {
		path := fmt.Sprintf("%s?offset=%d&limit=%d", pathEvents, offset, pageSize)

		var events []Event
		if err := c.http.Do(ctx, http.MethodGet, path, nil, &events); err != nil {
			return fmt.Errorf("slskd: events at offset %d: %w", offset, err)
		}
		if len(events) == 0 {
			return nil
		}
		if !fn(events) {
			return nil
		}
		if len(events) < pageSize {
			return nil
		}
		offset += len(events)
	}
	return fmt.Errorf("slskd: event log exceeded %d pages", maxEventPages)
}

// EventsPageAt returns one page of the event log at a descending offset.
//
// slskd serves this newest-first and reports no total, so an offset is the only
// position available. Walking in that native direction is what makes it safe:
// events are prepended, so one arriving mid-walk pushes the rest to a HIGHER
// offset and a page is re-read rather than skipped.
func (c *Client) EventsPageAt(ctx context.Context, offset, limit int) ([]Event, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 1000
	}

	var events []Event
	path := fmt.Sprintf("%s?offset=%d&limit=%d", pathEvents, offset, limit)
	if err := c.http.Do(ctx, http.MethodGet, path, nil, &events); err != nil {
		return nil, fmt.Errorf("slskd: events at offset %d: %w", offset, err)
	}
	return events, nil
}
