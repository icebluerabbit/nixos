// Package mam is a client for the MyAnonaMouse API. It is standalone (imports
// nothing of mousetrap's) so it reads as a reusable library.
//
// MAM spans two hosts: the main site (www.myanonamouse.net — search, jsonLoad,
// download, bonus, bonusBuy, jsonIp) and the tracker (t.myanonamouse.net —
// dynamicSeedbox; jsonIp works there too). The same mam_id session is valid on
// both, so the client holds both bases.
//
// It owns the session: MAM rotates the mam_id cookie on every request, so all calls
// are serialized under a mutex and the rotated cookie is persisted via a pluggable
// SessionStore. The transport core (do) is generic — every typed endpoint method is
// a thin wrapper over it.
package mam

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// SessionStore persists the rotating mam_id across requests and restarts. The store
// implementation (SQLite, a file, memory for tests) lives outside this package.
type SessionStore interface {
	Load() (mamID string, err error) // "" when no session yet
	Save(mamID string) error         // persist the rotated cookie
}

// HTTPError is returned for a non-2xx MAM response; Body holds the response payload
// (MAM often returns a JSON error body with a meaningful message, e.g. the 406 on
// download.php), so callers can inspect it.
type HTTPError struct {
	Code   int
	Status string
	Body   string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("mam: http %d: %s", e.Code, e.Status) }

// jsonErr wraps a decode failure with a snippet of the response body for debugging.
func jsonErr(what string, err error, body []byte) error {
	snip := body
	if len(snip) > 200 {
		snip = snip[:200]
	}
	return fmt.Errorf("mam: decode %s: %w (body: %s)", what, err, snip)
}

// Client is a single-owner MAM API session. Safe for concurrent use: requests are
// serialized because MAM rotates the session token per request.
type Client struct {
	apiUrl     string // main site host (www.myanonamouse.net)
	trackerUrl string // tracker host (t.myanonamouse.net) — dynamicSeedbox etc.
	seedPath   string
	userAgent  string
	http       *http.Client
	session    SessionStore
	mu         sync.Mutex
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the HTTP client (timeouts, transport).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithUserAgent sets the request User-Agent.
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgent = ua } }

// WithTrackerUrl overrides the tracker host (default: apiUrl with www.→t.).
func WithTrackerUrl(u string) Option { return func(c *Client) { c.trackerUrl = u } }

// New constructs a MAM client for apiUrl (the main site). The tracker host defaults
// to apiUrl with www.→t. (override with WithTrackerUrl). seedPath bootstraps the
// session on first use; session persists the rotated cookie thereafter.
func New(apiUrl, seedPath string, session SessionStore, opts ...Option) *Client {
	api := strings.TrimRight(apiUrl, "/")
	c := &Client{
		apiUrl:     api,
		trackerUrl: strings.Replace(api, "www.", "t.", 1),
		seedPath:   seedPath,
		session:    session,
		userAgent:  "mousetrap/1.0",
		http:       &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// do is the single transport path for every MAM call. Under the session lock it
// loads the current mam_id (seeding from seedPath on first use), sends the request
// with that cookie, and persists the rotated cookie MAM returns. path may be an
// absolute URL (e.g. a trackerUrl endpoint) or a path under apiUrl.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id, err := c.session.Load()
	if err != nil {
		return nil, fmt.Errorf("mam: load session: %w", err)
	}
	if id == "" {
		if id, err = c.seed(); err != nil {
			return nil, err
		}
	}

	u := path
	if !strings.HasPrefix(path, "http") {
		u = c.apiUrl + path
	}
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", "mam_id="+id)
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// MAM rotates mam_id per request; persist the new one if it set one.
	for _, ck := range resp.Cookies() {
		if ck.Name == "mam_id" && ck.Value != "" {
			_ = c.session.Save(ck.Value)
			break
		}
	}

	if resp.StatusCode >= 400 {
		return data, &HTTPError{Code: resp.StatusCode, Status: resp.Status, Body: string(data)}
	}
	return data, nil
}

// seed bootstraps the session from the seed file (a sops secret holding the mam_id),
// accepting either a bare value or a "mam_id=…;" cookie form, and persists it.
func (c *Client) seed() (string, error) {
	if c.seedPath == "" {
		return "", errors.New("mam: no session and no seed path")
	}
	b, err := os.ReadFile(c.seedPath)
	if err != nil {
		return "", fmt.Errorf("mam: read seed: %w", err)
	}
	id := strings.TrimSpace(string(b))
	id = strings.TrimPrefix(id, "mam_id=")
	id = strings.TrimSuffix(strings.TrimSpace(id), ";")
	if id == "" {
		return "", errors.New("mam: empty seed")
	}
	if err := c.session.Save(id); err != nil {
		return "", err
	}
	return id, nil
}
