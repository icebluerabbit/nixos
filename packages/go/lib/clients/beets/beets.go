// Package beets talks to beets' web plugin over HTTP.
//
// beets holds the tag layer — MusicBrainz ids resolved per track, the audio
// properties it probed, and the label, country and medium of the physical
// release. mb_releasegroupid is the valuable one: it is the same identifier
// Lidarr calls foreignAlbumId, so a tagged file on disk joins to the album that
// was searched for without comparing a single string.
//
// This used to read beets' SQLite library directly over a bind mount, on the
// belief that the web plugin served a query UI rather than the library. That
// was simply wrong: /item/ returns every item as JSON, with more fields than the
// hand-picked column list the SQL reader used. Reading another service's
// database file is worth avoiding on its own — it needs a mount, it needs
// matching uids, and it means knowing a schema that upstream is free to change.
//
// One requirement the deployment has to meet: the plugin's include_paths option
// must be on. It defaults to false, and without it every item returns
// "path": null — which removes the only field that ties a tagged item to a file
// on disk.
//
// https://beets.readthedocs.io/en/stable/plugins/web.html
package beets

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"homelab/httpx"
)

// Client talks to one beets instance.
type Client struct {
	http *httpx.Client
}

// New builds a client. The web plugin has no authentication of its own; it is
// reachable only on its VLAN.
func New(cfg httpx.Config) (*Client, error) {
	c, err := httpx.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{http: c}, nil
}

// Item is one tagged file in beets' library.
//
// Every field beyond Path is optional. beets writes a row as soon as a file is
// imported and fills the tags as it resolves them, so an unmatched import is a
// real item with almost nothing on it.
type Item struct {
	ID int64 `json:"id"`
	// Path is where the file is now, and is null unless the plugin's
	// include_paths option is enabled.
	Path string `json:"path"`

	MbTrackID        string `json:"mb_trackid"`
	MbAlbumID        string `json:"mb_albumid"`
	MbReleaseGroupID string `json:"mb_releasegroupid"`
	MbArtistID       string `json:"mb_artistid"`
	AcoustID         string `json:"acoustid_id"`
	// ISRC identifies the RECORDING rather than this release of it, so the
	// same code appears on an album and on every compilation that reuses it.
	// The one identifier here that ties two files together across releases.
	ISRC string `json:"isrc"`

	Album       string `json:"album"`
	AlbumArtist string `json:"albumartist"`
	Artist      string `json:"artist"`
	Title       string `json:"title"`
	Track       int    `json:"track"`
	Disc        int    `json:"disc"`
	Year        int    `json:"year"`
	Label       string `json:"label"`
	Country     string `json:"country"`
	Media       string `json:"media"`

	Format      string  `json:"format"`
	Bitrate     int     `json:"bitrate"`
	BitrateMode string  `json:"bitrate_mode"`
	BitDepth    int     `json:"bitdepth"`
	SampleRate  int     `json:"samplerate"`
	Channels    int     `json:"channels"`
	Length      float64 `json:"length"`

	// Added is beets' import time as a float of epoch seconds.
	Added float64 `json:"added"`
}

// AddedAt returns the import time, zero when beets never recorded one.
func (i Item) AddedAt() time.Time {
	if i.Added <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(int64(i.Added * 1000)).UTC()
}

// itemList is the envelope /item/ returns.
type itemList struct {
	Items []Item `json:"items"`
}

// Items returns every tagged file.
//
// Returned whole rather than streamed: the endpoint has no paging, and a beets
// library is items rather than events — this homelab's is a few hundred, and
// even a large one is thousands, not the six figures the torrent side deals in.
func (c *Client) Items(ctx context.Context) ([]Item, error) {
	var out itemList
	if err := c.http.Do(ctx, http.MethodGet, "/item/", nil, &out); err != nil {
		return nil, fmt.Errorf("beets: list items: %w", err)
	}
	return out.Items, nil
}

// Stats is beets' own count of what it holds.
type Stats struct {
	Items  int `json:"items"`
	Albums int `json:"albums"`
}

// Stats returns the library's size, which is also the cheapest reachability
// check.
func (c *Client) Stats(ctx context.Context) (Stats, error) {
	var out Stats
	if err := c.http.Do(ctx, http.MethodGet, "/stats", nil, &out); err != nil {
		return out, fmt.Errorf("beets: stats: %w", err)
	}
	return out, nil
}
