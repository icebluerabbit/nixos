package qbittorrent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// pathTorrentsSetTags replaces a torrent's tags outright.
const pathTorrentsSetTags = "/api/v2/torrents/setTags"

// pathTorrentsTags lists every tag the client knows.
const pathTorrentsTags = "/api/v2/torrents/tags"

// pathTorrentsDeleteTags unregisters tags entirely.
const pathTorrentsDeleteTags = "/api/v2/torrents/deleteTags"

// Tags lists every tag registered with the client.
//
// The REGISTRY, not the tags in use. qBittorrent keeps a tag in its list after
// the last torrent carrying it lets it go, which is why removing a tag from
// every torrent leaves it sitting in the sidebar with a count of zero. Anything
// that intends to clean up after a retired tool has to read this, not the
// torrents.
func (c *Client) Tags(ctx context.Context) ([]string, error) {
	var out []string
	if err := c.http.Do(ctx, "GET", pathTorrentsTags, nil, &out); err != nil {
		return nil, fmt.Errorf("qbittorrent: list tags: %w", err)
	}
	return out, nil
}

// DeleteTags unregisters tags, removing them from the client's tag list.
//
// Does not touch torrents: a tag still in use is removed from the ones carrying
// it as well, so a caller that means "tidy the sidebar" must pass only tags
// nothing carries.
func (c *Client) DeleteTags(ctx context.Context, tags []string) error {
	if len(tags) == 0 {
		return nil
	}

	form := url.Values{"tags": {strings.Join(tags, ",")}}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.http.BaseURL()+pathTorrentsDeleteTags, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("qbittorrent: delete tags request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.HTTP().Do(req)
	if err != nil {
		return fmt.Errorf("qbittorrent: delete %d tags: %w", len(tags), err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("qbittorrent: delete %d tags: %s: %s",
			len(tags), resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

// SetTags REPLACES the tags on the given torrents.
//
// Replace, not merge. addTags/removeTags exist alongside this and would let a
// caller diff, but diffing means deciding which existing tags are yours — and
// the tags on this client came from three tools, two of which no longer exist
// and whose tags have been frozen since the day they were removed. A single
// authority stating the whole set is the only arrangement in which a stale tag
// cannot outlive the thing that wrote it.
//
// Passing an empty tags slice CLEARS every tag on those torrents. That is the
// endpoint's documented behaviour and not a special case here, so a caller
// computing an empty set had better mean it.
//
// Requires WebAPI 2.11.0 (qBittorrent 5.0). Verified against 2.15.1 / v5.2.2,
// which answers 204.
//
// Form-encoded, like Delete: the WebUI API predates JSON bodies and silently
// ignores one, answering success while changing nothing.
func (c *Client) SetTags(ctx context.Context, hashes, tags []string) error {
	if len(hashes) == 0 {
		return nil
	}

	form := url.Values{
		"hashes": {strings.Join(hashes, "|")},
		// Comma-separated, and empty when clearing. A tag containing a comma
		// would split in two; the caller owns the vocabulary and is expected
		// not to mint one that does.
		"tags": {strings.Join(tags, ",")},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.http.BaseURL()+pathTorrentsSetTags, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("qbittorrent: set tags request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.HTTP().Do(req)
	if err != nil {
		return fmt.Errorf("qbittorrent: set tags on %d torrents: %w", len(hashes), err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	// Any 2xx. Checked as a range rather than against a literal because this
	// endpoint answers 204 while Delete's answers 200 — hardcoding either is
	// how one of them breaks on an upgrade.
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("qbittorrent: set tags on %d torrents: %s: %s",
			len(hashes), resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
