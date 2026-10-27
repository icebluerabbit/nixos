package qbittorrent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strconv"
	"strings"
)

// pathTorrentsAdd uploads a .torrent file.
const pathTorrentsAdd = "/api/v2/torrents/add"

// torrentContentType is the part type qBittorrent expects for the file.
const torrentContentType = "application/x-bittorrent"

// AddOptions are the per-add settings this package models.
type AddOptions struct {
	// Category the torrent is filed under. Empty means uncategorised.
	Category string
	// AutoTMM puts the torrent under automatic torrent management, so the
	// category's save path applies and keeps applying if it is changed.
	AutoTMM bool
}

// AddTorrentFile uploads one .torrent.
//
// Reports whether the client accepted it. A rejection is not an error: a
// torrent already present comes back refused, and that is the normal outcome
// of re-importing a watch directory, not a failure to report.
func (c *Client) AddTorrentFile(
	ctx context.Context, filename string, content []byte, opts AddOptions,
) (bool, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	if err := w.WriteField("category", opts.Category); err != nil {
		return false, fmt.Errorf("qbittorrent: add %s: %w", filename, err)
	}
	if err := w.WriteField("autoTMM", strconv.FormatBool(opts.AutoTMM)); err != nil {
		return false, fmt.Errorf("qbittorrent: add %s: %w", filename, err)
	}

	// CreateFormFile would label the part application/octet-stream.
	// qBittorrent accepts that, but naming the real type costs nothing and
	// is what every other client sends.
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="torrents"; filename=%q`, filename))
	header.Set("Content-Type", torrentContentType)
	part, err := w.CreatePart(header)
	if err != nil {
		return false, fmt.Errorf("qbittorrent: add %s: %w", filename, err)
	}
	if _, err := part.Write(content); err != nil {
		return false, fmt.Errorf("qbittorrent: add %s: %w", filename, err)
	}
	if err := w.Close(); err != nil {
		return false, fmt.Errorf("qbittorrent: add %s: %w", filename, err)
	}

	answer, err := c.post(ctx, pathTorrentsAdd, w.FormDataContentType(), &body,
		"add "+filename)
	if err != nil {
		return false, err
	}
	return addAccepted(answer), nil
}

// addAccepted reads the two answers this endpoint has had.
//
// Before 5.1 it replied with the bare string "Ok." and said nothing about how
// many of the uploaded torrents were taken; from 5.1 it replies with a JSON
// summary. Anything else — including a body that parses but reports zero —
// is a rejection.
func addAccepted(answer string) bool {
	if strings.EqualFold(answer, "Ok.") {
		return true
	}
	var summary struct {
		SuccessCount int `json:"success_count"`
	}
	if err := json.Unmarshal([]byte(answer), &summary); err != nil {
		return false
	}
	return summary.SuccessCount >= 1
}
