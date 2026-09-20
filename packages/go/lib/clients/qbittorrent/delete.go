package qbittorrent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// pathTorrentsDelete removes torrents from the client.
const pathTorrentsDelete = "/api/v2/torrents/delete"

// Delete removes torrents, optionally with their data.
//
// The ONLY mutating call in this package, and the reason the package comment
// says the rest is read-only. It exists for one job: a torrent the tracker has
// unregistered is dead weight — it can never be announced again, so seeding it
// accomplishes nothing and it occupies a slot forever.
//
// deleteFiles unlinks qBittorrent's copy of the data. That is safe for anything
// else holding the same content: an *arr import and a cross-seed inject are
// hardlinks, so removing this link leaves theirs intact and the bytes in place.
// Only when no other link exists does the data actually go, which is precisely
// the case where nothing else wanted it.
//
// Form-encoded rather than JSON: the WebUI API predates that convention and
// silently ignores a JSON body, answering 200 while deleting nothing.
func (c *Client) Delete(ctx context.Context, hashes []string, deleteFiles bool) error {
	if len(hashes) == 0 {
		return nil
	}

	form := url.Values{
		"hashes":      {strings.Join(hashes, "|")},
		"deleteFiles": {fmt.Sprint(deleteFiles)},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.http.BaseURL()+pathTorrentsDelete, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("qbittorrent: delete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.HTTP().Do(req)
	if err != nil {
		return fmt.Errorf("qbittorrent: delete %d torrents: %w", len(hashes), err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("qbittorrent: delete %d torrents: %s: %s",
			len(hashes), resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
