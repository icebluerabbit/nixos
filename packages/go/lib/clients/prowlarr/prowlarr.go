// Package prowlarr talks to a Prowlarr instance's HTTP API.
//
// Prowlarr proxies every indexer query in this stack, which makes its history
// the only record of what was searched for as opposed to what was grabbed. The
// *arr apps log their grabs; none of them log a search that returned nothing.
//
// Authentication is a static key in the X-Api-Key header.
//
// https://github.com/Prowlarr/Prowlarr
package prowlarr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"homelab/httpx"
)

const (
	pathHistory  = "/api/v1/history"
	pathIndexers = "/api/v1/indexer"
)

// EventType selects one kind of history record.
//
// Numeric because that is what the API filters on; the strings it returns are
// the names below.
type EventType int

const (
	// EventReleaseGrabbed is a release pulled through Prowlarr by a client.
	EventReleaseGrabbed EventType = 1
	// EventIndexerQuery is a search: a client asked an indexer a question.
	EventIndexerQuery EventType = 2
	// EventIndexerRss is a scheduled feed poll, not a user-driven search.
	EventIndexerRss EventType = 3
)

// Client is a Prowlarr API client.
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
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Privacy    string `json:"privacy"`
	Enable     bool   `json:"enable"`
	Priority   int    `json:"priority"`
	Identifier string `json:"identifier"`
}

// HistoryRecord is one history entry.
//
// Which keys Data carries depends on EventType; see the Event constants.
type HistoryRecord struct {
	ID        int64     `json:"id"`
	IndexerID int       `json:"indexerId"`
	Date      time.Time `json:"date"`
	EventType string    `json:"eventType"`
	Data      DataMap   `json:"data"`
}

// DataMap is Prowlarr's per-event data bag.
//
// Every value observed is a quoted string, including the numbers and the
// booleans — Prowlarr stores this as a string/string dictionary. A plain
// map[string]string would nonetheless be a liability: one value that is not a
// string fails the decode for the entire *page*, not the record, silently
// truncating a walk. This accepts whatever arrives and renders it as text.
type DataMap map[string]string

// UnmarshalJSON accepts string, number, boolean or null values.
func (d *DataMap) UnmarshalJSON(b []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(DataMap, len(raw))
	for k, v := range raw {
		switch t := v.(type) {
		case nil:
			out[k] = ""
		case string:
			out[k] = t
		case bool:
			out[k] = strconv.FormatBool(t)
		case float64:
			// Rendered without an exponent and without a trailing ".0", so a
			// value that arrived as a number still parses as an integer.
			out[k] = strconv.FormatFloat(t, 'f', -1, 64)
		default:
			// Objects and arrays: keep the JSON rather than lose the field.
			enc, err := json.Marshal(t)
			if err != nil {
				return fmt.Errorf("prowlarr: data key %q: %w", k, err)
			}
			out[k] = string(enc)
		}
	}
	*d = out
	return nil
}

// Str returns a data value, or "" when absent.
func (r HistoryRecord) Str(key string) string { return r.Data[key] }

// Int returns a data value parsed as an integer, and whether it was present
// and numeric. Prowlarr quotes its numbers, hence the conversion.
func (r HistoryRecord) Int(key string) (int, bool) {
	v, ok := r.Data[key]
	if !ok || v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

// Bool returns a data value parsed as Prowlarr's "0"/"1" boolean.
func (r HistoryRecord) Bool(key string) (value, ok bool) {
	n, ok := r.Int(key)
	if !ok {
		return false, false
	}
	return n != 0, true
}

// historyPage is one page of history.
type historyPage struct {
	Page         int             `json:"page"`
	PageSize     int             `json:"pageSize"`
	TotalRecords int             `json:"totalRecords"`
	Records      []HistoryRecord `json:"records"`
}

// Indexers returns every configured indexer.
func (c *Client) Indexers(ctx context.Context) ([]Indexer, error) {
	var out []Indexer
	if err := c.http.Do(ctx, http.MethodGet, pathIndexers, nil, &out); err != nil {
		return nil, fmt.Errorf("prowlarr: list indexers: %w", err)
	}
	return out, nil
}

// HistoryPage is one page of history plus the total Prowlarr reports.
type HistoryPage struct {
	Records      []HistoryRecord
	TotalRecords int
}

// HistoryPageAt returns one page of one event type, OLDEST first.
//
// The opposite ordering to History, and deliberately so. History is newest-first
// because its caller stops as soon as it recognises a record; this is for a
// cursor, and only an ascending order gives a stable offset — a record arriving
// mid-walk lands at the end and cannot shift a page already read.
//
// Prowlarr has no keyset filter: /history takes page, pageSize, eventType and a
// sort, and nothing else.
func (c *Client) HistoryPageAt(ctx context.Context, event EventType, page, pageSize int) (HistoryPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 500
	}

	path := fmt.Sprintf("%s?page=%d&pageSize=%d&eventType=%d"+
		"&sortKey=date&sortDirection=ascending",
		pathHistory, page, pageSize, event)

	var p historyPage
	if err := c.http.Do(ctx, http.MethodGet, path, nil, &p); err != nil {
		return HistoryPage{}, fmt.Errorf("prowlarr: history page %d: %w", page, err)
	}
	return HistoryPage{Records: p.Records, TotalRecords: p.TotalRecords}, nil
}

// History walks history of one event type, newest first, calling fn per page.
//
// Streamed rather than accumulated: the corpus is tens of thousands of records
// and a caller writes each row and forgets it.
//
// fn returning false stops the walk, which is how an incremental pass ends once
// it reaches records it already holds.
//
// Paging is by page number over a newest-first ordering, so a record arriving
// mid-walk shifts everything down by one and can hide a row. The caller must
// therefore rely on an idempotent upsert and a later pass rather than treating
// one completed walk as exhaustive.
func (c *Client) History(ctx context.Context, event EventType, pageSize int, fn func([]HistoryRecord) bool) error {
	if pageSize <= 0 {
		pageSize = 500
	}
	for page := 1; ; page++ {
		// sortKey/sortDirection are sent explicitly rather than relying on the
		// API's default. The caller's incremental stop condition — "this page
		// held nothing new, so everything after it is older and already
		// stored" — is only sound on a newest-first ordering, and an unstated
		// default that changed would silently turn the pass into a no-op while
		// still logging success.
		path := fmt.Sprintf("%s?page=%d&pageSize=%d&eventType=%d"+
			"&sortKey=date&sortDirection=descending",
			pathHistory, page, pageSize, event)

		var p historyPage
		if err := c.http.Do(ctx, http.MethodGet, path, nil, &p); err != nil {
			return fmt.Errorf("prowlarr: history page %d: %w", page, err)
		}
		if len(p.Records) == 0 {
			return nil
		}
		if !fn(p.Records) {
			return nil
		}
		if len(p.Records) < pageSize {
			return nil
		}
	}
}
