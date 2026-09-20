// Package bazarr talks to Bazarr's HTTP API.
//
// Bazarr is the only service in this stack that knows anything about subtitles,
// and it knows two distinct things that nothing else does: which subtitle files
// exist beside which video, and which languages are still WANTED and have not
// been found. The second is the more valuable — an absence is not visible on
// disk.
//
// It is also the cheapest correlation in the whole stack. Bazarr carries
// Sonarr's and Radarr's own ids on every record (sonarrEpisodeId, radarrId), so
// its rows join to the *arr catalog by shared identifier with no title matching
// anywhere.
//
// Read-only by construction: nothing here downloads, deletes or blacklists a
// subtitle.
package bazarr

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"homelab/httpx"
)

// Client talks to one Bazarr instance.
type Client struct {
	http *httpx.Client
}

// New builds a client. Bazarr authenticates with X-API-KEY — note the hyphen
// and the capitalisation differ from the Servarr family's X-Api-Key.
func New(cfg httpx.Config, apiKey string) (*Client, error) {
	c, err := httpx.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{http: c.WithHeader("X-API-KEY", apiKey)}, nil
}

// envelope wraps every list response.
type envelope[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
}

// allPages is Bazarr's "give me everything" paging argument. Unlike the Servarr
// family it honours a negative length rather than requiring a walk.
const allPages = "-1"

// Version reports the running build.
func (c *Client) Version(ctx context.Context) (string, error) {
	var out struct {
		Data struct {
			BazarrVersion string `json:"bazarr_version"`
		} `json:"data"`
	}
	if err := c.http.Do(ctx, "GET", "/api/system/status", nil, &out); err != nil {
		return "", fmt.Errorf("bazarr: status: %w", err)
	}
	return out.Data.BazarrVersion, nil
}

// Subtitle is a subtitle track Bazarr has found on disk.
//
// Path is empty for an embedded track: Bazarr reports subtitles muxed into the
// video alongside external files, and the difference is exactly whether there
// is a file.
type Subtitle struct {
	Name     string `json:"name"`
	Code2    string `json:"code2"`
	Code3    string `json:"code3"`
	Path     string `json:"path"`
	Forced   bool   `json:"forced"`
	HI       bool   `json:"hi"`
	FileSize int64  `json:"file_size"`
}

// Wanted is a language Bazarr is still looking for.
type Wanted struct {
	Name   string `json:"name"`
	Code2  string `json:"code2"`
	Code3  string `json:"code3"`
	Forced bool   `json:"forced"`
	HI     bool   `json:"hi"`
}

// Series is one show Bazarr tracks, keyed by Sonarr's id.
type Series struct {
	SonarrSeriesID int64  `json:"sonarrSeriesId"`
	Title          string `json:"title"`
	Path           string `json:"path"`
}

// SeriesList returns every show Bazarr tracks.
func (c *Client) SeriesList(ctx context.Context) ([]Series, error) {
	var out envelope[Series]
	q := url.Values{"start": {"0"}, "length": {allPages}}
	if err := c.http.Do(ctx, "GET", "/api/series?"+q.Encode(), nil, &out); err != nil {
		return nil, fmt.Errorf("bazarr: series: %w", err)
	}
	return out.Data, nil
}

// Episode is one episode's subtitle state.
type Episode struct {
	SonarrSeriesID   int64      `json:"sonarrSeriesId"`
	SonarrEpisodeID  int64      `json:"sonarrEpisodeId"`
	Title            string     `json:"title"`
	Path             string     `json:"path"`
	Season           int        `json:"season"`
	Episode          int        `json:"episode"`
	Subtitles        []Subtitle `json:"subtitles"`
	MissingSubtitles []Wanted   `json:"missing_subtitles"`
}

// episodeBatch is how many series are asked for at once.
//
// The endpoint takes a repeated seriesid[], and unlike the Servarr file
// collections it honours all of them. Batched because a library of a thousand
// shows is a thousand round trips otherwise; bounded because the ids go in the
// query string and a URL has a length limit.
const episodeBatch = 50

// Episodes returns the subtitle state of every episode of the given series.
func (c *Client) Episodes(ctx context.Context, seriesIDs []int64) ([]Episode, error) {
	var all []Episode
	for start := 0; start < len(seriesIDs); start += episodeBatch {
		end := min(start+episodeBatch, len(seriesIDs))

		q := url.Values{}
		for _, id := range seriesIDs[start:end] {
			q.Add("seriesid[]", strconv.FormatInt(id, 10))
		}
		var out envelope[Episode]
		if err := c.http.Do(ctx, "GET", "/api/episodes?"+q.Encode(), nil, &out); err != nil {
			return all, fmt.Errorf("bazarr: episodes: %w", err)
		}
		all = append(all, out.Data...)
	}
	return all, nil
}

// Movie is one film's subtitle state, keyed by Radarr's id.
type Movie struct {
	RadarrID         int64      `json:"radarrId"`
	Title            string     `json:"title"`
	Path             string     `json:"path"`
	Subtitles        []Subtitle `json:"subtitles"`
	MissingSubtitles []Wanted   `json:"missing_subtitles"`
}

// Movies returns the subtitle state of every film Bazarr tracks.
func (c *Client) Movies(ctx context.Context) ([]Movie, error) {
	var out envelope[Movie]
	q := url.Values{"start": {"0"}, "length": {allPages}}
	if err := c.http.Do(ctx, "GET", "/api/movies?"+q.Encode(), nil, &out); err != nil {
		return nil, fmt.Errorf("bazarr: movies: %w", err)
	}
	return out.Data, nil
}

// HistoryRecord is one thing Bazarr did to a subtitle.
//
// Only the download action carries a provider and a score — the numbers that
// say how good the match was and who supplied it, which exist nowhere else.
// Timestamp is a rendered string ("2 days ago"); ParsedTimestamp is the real
// one, and is the field worth storing.
type HistoryRecord struct {
	ID              int64  `json:"id"`
	Action          int    `json:"action"`
	SonarrSeriesID  int64  `json:"sonarrSeriesId"`
	SonarrEpisodeID int64  `json:"sonarrEpisodeId"`
	RadarrID        int64  `json:"radarrId"`
	Provider        string `json:"provider"`
	SubsID          string `json:"subs_id"`
	Score           string `json:"score"`
	// Language is an object on history records, unlike the flat code on a
	// subtitle listing.
	Language struct {
		Name   string `json:"name"`
		Code2  string `json:"code2"`
		Code3  string `json:"code3"`
		Forced bool   `json:"forced"`
		HI     bool   `json:"hi"`
	} `json:"language"`
	Matches         []string `json:"matches"`
	DontMatches     []string `json:"dont_matches"`
	SubtitlesPath   string   `json:"subtitles_path"`
	VideoPath       string   `json:"video_path"`
	ParsedTimestamp string   `json:"parsed_timestamp"`
	Upgradable      bool     `json:"upgradable"`
	Blacklisted     bool     `json:"blacklisted"`
}

// Action values Bazarr records. Only downloads produce a file.
const (
	ActionDeleted  = 0
	ActionDownload = 1
	ActionManual   = 2
	ActionUpgrade  = 3
	ActionSync     = 4
)

// collection maps a half of the library onto Bazarr's URL noun.
//
// Bazarr keeps series and movies in separate collections rather than one with a
// discriminator, and names the TV one after the episode rather than the series:
// the routes are /api/episodes/* and /api/movies/*.
//
// The collection comes FIRST and the resource second — /api/episodes/history,
// not /api/history/episodes. Getting that backwards does not 404: Bazarr serves
// its single-page app on anything it does not recognise, so the wrong path
// returns 200 with an HTML body and fails later as a JSON decode error that
// says nothing about the route.
func collection(kind string) (string, error) {
	switch kind {
	case "series", "episodes":
		return "episodes", nil
	case "movies":
		return "movies", nil
	default:
		return "", fmt.Errorf("bazarr: unknown collection %q", kind)
	}
}

// History returns Bazarr's subtitle history for one half of the library.
//
// kind is "series" or "movies".
func (c *Client) History(ctx context.Context, kind string) ([]HistoryRecord, error) {
	name, err := collection(kind)
	if err != nil {
		return nil, err
	}
	var out envelope[HistoryRecord]
	q := url.Values{"start": {"0"}, "length": {allPages}}
	path := "/api/" + name + "/history?" + q.Encode()
	if err := c.http.Do(ctx, "GET", path, nil, &out); err != nil {
		return nil, fmt.Errorf("bazarr: history %s: %w", kind, err)
	}
	return out.Data, nil
}

// BlacklistRecord is a subtitle Bazarr has been told never to download again.
type BlacklistRecord struct {
	SonarrSeriesID  int64  `json:"sonarrSeriesId"`
	SonarrEpisodeID int64  `json:"sonarrEpisodeId"`
	RadarrID        int64  `json:"radarrId"`
	Provider        string `json:"provider"`
	SubsID          string `json:"subs_id"`
	SubtitlesPath   string `json:"subtitles_path"`
	Language        struct {
		Name  string `json:"name"`
		Code2 string `json:"code2"`
		Code3 string `json:"code3"`
	} `json:"language"`
	ParsedTimestamp string `json:"parsed_timestamp"`
}

// Blacklist returns the refused subtitles for one half of the library.
func (c *Client) Blacklist(ctx context.Context, kind string) ([]BlacklistRecord, error) {
	name, err := collection(kind)
	if err != nil {
		return nil, err
	}
	var out envelope[BlacklistRecord]
	if err := c.http.Do(ctx, "GET", "/api/"+name+"/blacklist", nil, &out); err != nil {
		return nil, fmt.Errorf("bazarr: blacklist %s: %w", kind, err)
	}
	return out.Data, nil
}
