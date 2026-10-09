package mam

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Download fetches the .torrent for tid via /tor/download.php. When wedge is true it
// appends the freeleech flag (fl), spending a Personal Freeleech wedge at download
// time — even on VIP torrents, with no refunds — so the caller must be certain. On a
// non-2xx response the returned error is an *HTTPError whose Body carries MAM's
// message (e.g. the 406 "Account issue").
func (c *Client) Download(ctx context.Context, tid int64, wedge bool) ([]byte, error) {
	q := url.Values{}
	q.Set("tid", strconv.FormatInt(tid, 10))
	if wedge {
		q.Set("fl", "")
	}
	return c.do(ctx, http.MethodGet, "/tor/download.php", q, nil)
}
