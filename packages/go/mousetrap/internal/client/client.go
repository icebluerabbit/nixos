// Package client is the download-client integration — qBittorrent's WebUI today.
// Kept behind a small surface (Add) so a second client would slot in without
// touching grab policy. mousetrap is expected to be auth-whitelisted on qBittorrent
// (its VLAN in the WebUI subnet whitelist), so no login is performed.
package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"homelab/mousetrap/internal/types"
)

// ErrRejected is returned when qBittorrent accepts the request but refuses the
// torrent (replies "Fails."), as opposed to a transport/HTTP error.
var ErrRejected = errors.New("client: torrent rejected")

// rawBodyLimit caps a logged response body; qBittorrent's add reply is a word, but
// an error page is not.
const rawBodyLimit = 8 << 10

// clip truncates s to rawBodyLimit, marking the result so a cut body is never read
// as a complete one.
func clip(s string) string {
	if len(s) <= rawBodyLimit {
		return s
	}
	return s[:rawBodyLimit] + "…(truncated)"
}

// Client adds torrents to qBittorrent.
type Client struct {
	url        string
	categories map[string]string // MAM media type → category
	addPaused  bool
	http       *http.Client
}

// New constructs the download client from config.
func New(cfg types.ClientConfig) *Client {
	return &Client{
		url:        strings.TrimRight(cfg.URL, "/"),
		categories: cfg.Categories,
		addPaused:  cfg.AddPaused,
		http:       &http.Client{Timeout: 30 * time.Second},
	}
}

// Add uploads a .torrent to qBittorrent under the category mapped from MAM's media
// type, with auto torrent management on. An unmapped media type yields the empty
// category, which qBittorrent treats as uncategorized. Returns ErrRejected if
// qBittorrent replies "Fails.".
func (c *Client) Add(ctx context.Context, torrent []byte, name, mediaType string) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("torrents", name+".torrent")
	if err != nil {
		return err
	}
	if _, err := fw.Write(torrent); err != nil {
		return err
	}
	_ = mw.WriteField("category", c.categories[mediaType])
	_ = mw.WriteField("autoTMM", "true")
	_ = mw.WriteField("paused", strconv.FormatBool(c.addPaused))
	if err := mw.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/api/v2/torrents/add", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body
	body, _ := io.ReadAll(resp.Body)

	// "Ok." / "Fails." carries no reason, so log the reply verbatim alongside the
	// torrent it answers.
	slog.Info("client_raw",
		"path", "/api/v2/torrents/add",
		"name", name,
		"category", c.categories[mediaType],
		"status", resp.StatusCode,
		"dur_ms", time.Since(start).Milliseconds(),
		"body", clip(strings.TrimSpace(string(body))),
	)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("client: add http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// qBittorrent replies 200 with "Ok." on success or "Fails." on rejection.
	if strings.TrimSpace(string(body)) == "Fails." {
		return ErrRejected
	}
	return nil
}
