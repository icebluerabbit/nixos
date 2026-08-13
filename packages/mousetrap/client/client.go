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
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mousetrap/types"
)

// ErrRejected is returned when qBittorrent accepts the request but refuses the
// torrent (replies "Fails."), as opposed to a transport/HTTP error.
var ErrRejected = errors.New("client: torrent rejected")

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

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("client: add http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// qBittorrent replies 200 with "Ok." on success or "Fails." on rejection.
	if strings.TrimSpace(string(body)) == "Fails." {
		return ErrRejected
	}
	return nil
}
