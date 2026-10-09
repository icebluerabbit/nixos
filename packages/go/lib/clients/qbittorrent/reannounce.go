package qbittorrent

import (
	"context"
	"net/url"
	"strings"
)

// pathTorrentsReannounce forces a tracker announce.
const pathTorrentsReannounce = "/api/v2/torrents/reannounce"

// Reannounce asks the client to announce these torrents to their trackers now.
//
// Idempotent and harmless: the worst outcome is an announce the tracker was
// going to receive anyway. It exists because a torrent whose trackers all
// errored at add time keeps its announce interval and will sit at zero
// progress until that interval elapses, which on a private tracker is half an
// hour of a grab doing nothing.
func (c *Client) Reannounce(ctx context.Context, hashes []string) error {
	if len(hashes) == 0 {
		return nil
	}
	form := url.Values{"hashes": {strings.Join(hashes, "|")}}
	_, err := c.postForm(ctx, pathTorrentsReannounce, form, "reannounce")
	return err
}
