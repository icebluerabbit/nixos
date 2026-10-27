// Package arr talks to the Servarr family's HTTP APIs — Sonarr, Radarr and
// Lidarr — over the surface needed to reconcile a library.
//
// Read-only by construction. Nothing here deletes a file, removes a queue item
// or blocklists a release; the package cannot be misused into mutating a
// library because the methods do not exist.
//
// Two shapes differ between the apps and both have bitten this homelab before:
//
//   - API version. Sonarr and Radarr are v3, Lidarr is v1. Getting this wrong
//     returns 404 rather than an obvious error.
//   - The file collection. Sonarr has episodefile, Radarr moviefile, Lidarr
//     trackfile, and their records name the parent differently.
//
// History is the interesting endpoint: it is retrospective, so a single walk
// recovers everything that happened before this client existed. Webhooks only
// run forward from the moment they are wired.
package arr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"homelab/httpx"
)

// Kind identifies which member of the family a client talks to.
type Kind string

// The supported apps. Readarr is deliberately absent: this homelab routes books
// through Bindery and the packaged Readarr is from the archived upstream.
const (
	Sonarr Kind = "sonarr"
	Radarr Kind = "radarr"
	Lidarr Kind = "lidarr"
)

// apiVersion is the path segment each app serves its API under.
func (k Kind) apiVersion() string {
	if k == Lidarr {
		return "v1"
	}
	return "v3"
}

// filePath is the collection of imported files.
func (k Kind) filePath() string {
	switch k {
	case Radarr:
		return "moviefile"
	case Lidarr:
		return "trackfile"
	default:
		return "episodefile"
	}
}

// ErrUnknownKind reports a client built for an unsupported app.
var ErrUnknownKind = errors.New("arr: unknown kind")

// Client talks to one instance.
type Client struct {
	kind Kind
	http *httpx.Client
}

// New builds a client. The API key goes in the X-Api-Key header, which every
// member of the family accepts.
func New(kind Kind, cfg httpx.Config, apiKey string) (*Client, error) {
	switch kind {
	case Sonarr, Radarr, Lidarr:
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownKind, kind)
	}
	c, err := httpx.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{kind: kind, http: c.WithHeader("X-Api-Key", apiKey)}, nil
}

// Kind reports which app this client talks to.
func (c *Client) Kind() Kind { return c.kind }

func (c *Client) path(resource string) string {
	return "/api/" + c.kind.apiVersion() + "/" + resource
}

// EventKind is a history event's meaning, normalized across the three apps.
//
// The apps do not share a vocabulary, do not share numbering, and do not even
// agree on which events exist. Taken from the enums themselves:
//
//	Sonarr  EpisodeHistoryEventType  src/NzbDrone.Core/History/EpisodeHistory.cs
//	  Unknown 0, Grabbed 1, SeriesFolderImported 2, DownloadFolderImported 3,
//	  DownloadFailed 4, EpisodeFileDeleted 5, EpisodeFileRenamed 6,
//	  DownloadIgnored 7
//
//	Radarr  MovieHistoryEventType    src/NzbDrone.Core/History/History.cs
//	  Unknown 0, Grabbed 1, DownloadFolderImported 3, DownloadFailed 4,
//	  MovieFileDeleted 6, MovieFolderImported 7, MovieFileRenamed 8,
//	  DownloadIgnored 9        (2 and 5 deprecated, numbering has holes)
//
//	Lidarr  EntityHistoryEventType   src/NzbDrone.Core/History/EntityHistory.cs
//	  Unknown 0, Grabbed 1, ArtistFolderImported 2, TrackFileImported 3,
//	  DownloadFailed 4, TrackFileDeleted 5, TrackFileRenamed 6,
//	  AlbumImportIncomplete 7, DownloadImported 8, TrackFileRetagged 9,
//	  DownloadIgnored 10
//
// Lidarr is the trap: it has no downloadFolderImported at all. Matching on that
// one string — the obvious choice from reading Sonarr's payloads — silently
// links nothing for music.
type EventKind string

// The normalized vocabulary.
const (
	EventUnknown          EventKind = "unknown"
	EventGrabbed          EventKind = "grabbed"
	EventImported         EventKind = "imported"
	EventFailed           EventKind = "failed"
	EventDeleted          EventKind = "deleted"
	EventRenamed          EventKind = "renamed"
	EventIgnored          EventKind = "ignored"
	EventRetagged         EventKind = "retagged"
	EventImportIncomplete EventKind = "import_incomplete"
)

// eventKinds maps every serialized eventType of every app to its meaning. The
// API emits the enum's name camelCased, so these are the names above with a
// lowercased first letter.
//
// One table for all three: the names do not collide, and a per-app table would
// invite exactly the copy-paste divergence that hid the Lidarr case.
var eventKinds = map[string]EventKind{
	"unknown": EventUnknown,
	"grabbed": EventGrabbed,

	// Import, under five different names.
	"downloadFolderImported": EventImported, // sonarr, radarr
	"seriesFolderImported":   EventImported, // sonarr
	"movieFolderImported":    EventImported, // radarr
	"artistFolderImported":   EventImported, // lidarr
	"trackFileImported":      EventImported, // lidarr
	"downloadImported":       EventImported, // lidarr

	"downloadFailed":  EventFailed,
	"downloadIgnored": EventIgnored,

	"episodeFileDeleted": EventDeleted,
	"movieFileDeleted":   EventDeleted,
	"trackFileDeleted":   EventDeleted,

	"episodeFileRenamed": EventRenamed,
	"movieFileRenamed":   EventRenamed,
	"trackFileRenamed":   EventRenamed,

	"trackFileRetagged":     EventRetagged,
	"albumImportIncomplete": EventImportIncomplete,
}

// HistoryRecord is one entry of the *arr's own history.
//
// EventType is the raw vendor string; Kind reports its normalized meaning.
type HistoryRecord struct {
	ID          int64  `json:"id"`
	EventType   string `json:"eventType"`
	SourceTitle string `json:"sourceTitle"`
	// DownloadID is the infohash, uppercase, empty for events with no
	// download behind them. This is the join key to the torrent client.
	DownloadID string            `json:"downloadId"`
	Date       time.Time         `json:"date"`
	Data       map[string]string `json:"data"`
	// Parent ids, whichever applies to this app.
	SeriesID  int64 `json:"seriesId"`
	EpisodeID int64 `json:"episodeId"`
	MovieID   int64 `json:"movieId"`
	ArtistID  int64 `json:"artistId"`
	AlbumID   int64 `json:"albumId"`

	// The *arr's own assessment of the release, carried on every record.
	// customFormatScore is what "preferred" means here — it is the profile
	// score, not an opinion this package forms.
	Quality           Quality        `json:"quality"`
	CustomFormats     []CustomFormat `json:"customFormats"`
	CustomFormatScore int            `json:"customFormatScore"`
	Languages         []Language     `json:"languages"`
}

// Kind returns what the record means. An unrecognised eventType yields
// EventUnknown rather than being dropped, so a new upstream value shows up as
// an unknown count instead of vanishing.
func (r HistoryRecord) Kind() EventKind {
	if k, ok := eventKinds[r.EventType]; ok {
		return k
	}
	return EventUnknown
}

// Data keys carried on history records.
//
// Declared as constants in the History model classes (DOWNLOAD_CLIENT,
// RELEASE_GROUP, SIZE, INDEXER, …) plus the ones the import and deletion paths
// add. Reading them by name from the map is unavoidable; getting the name
// wrong is silent, which is how data.path — a key that does not exist — went
// unnoticed while reporting zero supersedes.
const (
	DataDownloadClient = "downloadClient"
	DataReleaseGroup   = "releaseGroup"
	DataSize           = "size"
	DataIndexer        = "indexer"
	DataIndexerFlags   = "indexerFlags"
	DataReleaseType    = "releaseType"
	DataImportedPath   = "importedPath"
	DataDroppedPath    = "droppedPath"
	// Rename keys. Sonarr, Radarr and Lidarr all add SourcePath and Path on a
	// rename, so one pair covers the family. Note that Path exists ONLY on a
	// rename — the deletion path does not add it, which is the trap that made
	// an earlier reader of data.path report zero results forever.
	DataSourcePath = "sourcePath"
	DataPath       = "path"
	DataFileID     = "fileId"
	DataReason     = "reason"
	DataMessage    = "message"
	// DataStatusMessages is Lidarr's per-file complaint list on a partial album
	// import, serialised as a JSON array rather than as prose.
	DataStatusMessages = "statusMessages"
)

// historyPage is the paged envelope history is served in.
type historyPage struct {
	Page         int             `json:"page"`
	PageSize     int             `json:"pageSize"`
	TotalRecords int             `json:"totalRecords"`
	Records      []HistoryRecord `json:"records"`
}

// maxHistoryPages bounds a walk. History is unbounded and grows forever; a
// backfill that ran away would hammer the instance for hours. At the default
// page size this is ~250k records, far past any real library.
const maxHistoryPages = 500

// HistoryPage is one page of history plus the total the *arr reports.
type HistoryPage struct {
	Records      []HistoryRecord
	TotalRecords int
}

// HistoryPageAt returns one page of history, oldest first.
//
// Offset paging, because the *arr offers nothing better. /history takes only
// page, pageSize and a sort; there is no "id greater than" filter. There is a
// /history/since?date= endpoint, but it is unpaged — it returns every record
// from that date in one response, which is hundreds of kilobytes per day here
// and the entire history on a first walk.
//
// Ascending is what makes offsets usable: history is appended, so a new record
// lands at the end and cannot shift a page already read. The hazard is
// deletion — removing a series drops its history and shifts everything after it
// back — which can skip records. Consumers here upsert idempotently and re-walk
// in full periodically, so a skip costs latency rather than data.
func (c *Client) HistoryPageAt(ctx context.Context, page, pageSize int) (HistoryPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 500
	}

	q := url.Values{
		"page":          {strconv.Itoa(page)},
		"pageSize":      {strconv.Itoa(pageSize)},
		"sortKey":       {"date"},
		"sortDirection": {"ascending"},
	}
	var out historyPage
	if err := c.http.Do(ctx, "GET", c.path("history")+"?"+q.Encode(), nil, &out); err != nil {
		return HistoryPage{}, fmt.Errorf("arr %s: history page %d: %w", c.kind, page, err)
	}
	return HistoryPage{Records: out.Records, TotalRecords: out.TotalRecords}, nil
}

// History walks the instance's history newest-first and returns every record.
//
// Paged deliberately rather than asking for one enormous page: the *arr builds
// the whole response in memory before sending it.
func (c *Client) History(ctx context.Context, pageSize int) ([]HistoryRecord, error) {
	if pageSize <= 0 {
		pageSize = 500
	}

	var all []HistoryRecord
	for page := 1; page <= maxHistoryPages; page++ {
		q := url.Values{
			"page":     {strconv.Itoa(page)},
			"pageSize": {strconv.Itoa(pageSize)},
			"sortKey":  {"date"},
			// Ascending so a partial walk holds the oldest records and
			// a later run can resume forward rather than re-reading.
			"sortDirection": {"ascending"},
		}
		var out historyPage
		if err := c.http.Do(ctx, "GET", c.path("history")+"?"+q.Encode(), nil, &out); err != nil {
			return all, fmt.Errorf("arr %s: history page %d: %w", c.kind, page, err)
		}
		all = append(all, out.Records...)
		if len(out.Records) < pageSize || len(all) >= out.TotalRecords {
			return all, nil
		}
	}
	return all, fmt.Errorf("arr %s: history exceeded %d pages", c.kind, maxHistoryPages)
}

// Quality is the *arr's QualityModel: a quality plus a revision.
//
// The revision matters. Two files of the same quality are not equivalent if one
// is a PROPER or REPACK — that is precisely what makes the second one replace
// the first.
type Quality struct {
	Quality struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Source     string `json:"source"`
		Resolution int    `json:"resolution"`
	} `json:"quality"`
	Revision struct {
		Version  int  `json:"version"`
		Real     int  `json:"real"`
		IsRepack bool `json:"isRepack"`
	} `json:"revision"`
}

// CustomFormat is one matched format. Per-instance: the same name may be
// defined differently in Sonarr and Radarr and the ids are unrelated.
type CustomFormat struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Language as the *arr reports it.
type Language struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Flex decodes a JSON value that may arrive as either a number or a string,
// and keeps it as text.
//
// The apps disagree about bitrates. Sonarr's MediaInfoResource types them as
// int64; Lidarr's is a different class entirely whose AudioBitRate is a
// preformatted string like "1411 kbps". Go matches JSON keys
// case-insensitively, so Lidarr's audioBitRate binds to an audioBitrate field
// and fails the decode — which took the whole pass down.
type Flex string

// UnmarshalJSON accepts a string, a number, or null.
func (f *Flex) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	switch {
	case s == "null":
		*f = ""
	case strings.HasPrefix(s, `"`):
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		*f = Flex(str)
	default:
		*f = Flex(s)
	}
	return nil
}

// String returns the value as text.
func (f Flex) String() string { return string(f) }

// MediaInfo is the probe result for a file. Every field is optional — it is
// absent until the *arr has analysed the file, and the video half is absent
// entirely for Lidarr, whose resource carries only audio.
type MediaInfo struct {
	VideoCodec            string  `json:"videoCodec"`
	VideoBitrate          Flex    `json:"videoBitrate"`
	VideoBitDepth         int     `json:"videoBitDepth"`
	VideoFps              float64 `json:"videoFps"`
	VideoDynamicRange     string  `json:"videoDynamicRange"`
	VideoDynamicRangeType string  `json:"videoDynamicRangeType"`
	Resolution            string  `json:"resolution"`
	RunTime               string  `json:"runTime"`
	ScanType              string  `json:"scanType"`
	AudioCodec            string  `json:"audioCodec"`
	AudioBitrate          Flex    `json:"audioBitrate"`
	AudioChannels         float64 `json:"audioChannels"`
	AudioLanguages        string  `json:"audioLanguages"`
	AudioStreamCount      int     `json:"audioStreamCount"`
	Subtitles             string  `json:"subtitles"`

	// Lidarr only.
	AudioBits       Flex `json:"audioBits"`
	AudioSampleRate Flex `json:"audioSampleRate"`
}

// MediaFile is one imported file as the instance currently sees it.
//
// Path is absolute. RelativePath is relative to the parent's folder and is what
// the *arr renames; both are recorded because a rename changes one and not the
// other.
//
// QualityCutoffNotMet is the *arr's own statement that it would still upgrade
// this file — the difference between "best I have" and "best there is".
type MediaFile struct {
	ID           int64     `json:"id"`
	Path         string    `json:"path"`
	RelativePath string    `json:"relativePath"`
	Size         int64     `json:"size"`
	DateAdded    time.Time `json:"dateAdded"`
	SeriesID     int64     `json:"seriesId"`
	MovieID      int64     `json:"movieId"`
	AlbumID      int64     `json:"albumId"`
	ArtistID     int64     `json:"artistId"`

	SeasonNumber        int            `json:"seasonNumber"`
	SceneName           string         `json:"sceneName"`
	ReleaseGroup        string         `json:"releaseGroup"`
	Quality             Quality        `json:"quality"`
	CustomFormats       []CustomFormat `json:"customFormats"`
	CustomFormatScore   int            `json:"customFormatScore"`
	QualityCutoffNotMet bool           `json:"qualityCutoffNotMet"`
	Languages           []Language     `json:"languages"`
	IndexerFlags        *int           `json:"indexerFlags"`
	ReleaseType         string         `json:"releaseType"`
	MediaInfo           *MediaInfo     `json:"mediaInfo"`
}

// parentParam is the filter key for the app's file collection.
//
// All three require one — none of them serves the collection whole. Radarr is
// explicit about it:
//
//	GET /api/v3/moviefile -> 400 "movieId or movieFileIds must be provided"
func (k Kind) parentParam() string {
	switch k {
	case Radarr:
		return "movieId"
	case Lidarr:
		return "artistId"
	default:
		return "seriesId"
	}
}

// Files lists every imported file, one request per parent.
//
// All three apps require a parent filter and take EXACTLY ONE. Given a repeated
// parameter they honour the first and silently ignore the rest — no error, no
// warning, just a short answer:
//
//	?seriesId=1                  -> 19 files
//	?seriesId=999999&seriesId=1  ->  0 files
//
// Batching was tried twice and failed differently each time. All ids in one
// query was a 414 from Sonarr; fifty per request returned 44 files across 939
// series and reported success. Neither failure names itself, which is why this
// is one call per parent however chatty that looks.
func (c *Client) Files(ctx context.Context, parentIDs []int64) ([]MediaFile, error) {
	param := c.kind.parentParam()
	base := c.path(c.kind.filePath())

	var all []MediaFile
	for _, id := range parentIDs {
		q := url.Values{param: {strconv.FormatInt(id, 10)}}

		var out []MediaFile
		if err := c.http.Do(ctx, "GET", base+"?"+q.Encode(), nil, &out); err != nil {
			return all, fmt.Errorf("arr %s: files for %s=%d: %w", c.kind, param, id, err)
		}
		all = append(all, out...)
	}
	return all, nil
}

// DeleteFile removes one imported file from the library.
//
// DESTRUCTIVE, and more so than the name suggests: the *arr unlinks the file
// from disk as well as dropping its own record. That is the point — an orphaned
// import is a file nothing is seeding and nothing wants — but it means a caller
// that only meant to forget the record has deleted media instead. There is no
// "forget but keep" on this endpoint.
//
// What happens next is the *arr's own policy, not this call's: the parent stays
// monitored, so the release becomes missing and the next search replaces it.
// Anything that wants it to stay gone has to unmonitor separately.
//
// One method for all three apps. The resource differs — episodefile, moviefile,
// trackfile — and Kind already knows which, the same mapping Files uses.
func (c *Client) DeleteFile(ctx context.Context, fileID int64) error {
	path := c.path(c.kind.filePath()) + "/" + strconv.FormatInt(fileID, 10)
	// No response body to decode: all three return 200 with an empty object.
	if err := c.http.Do(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("arr %s: delete %s %d: %w", c.kind, c.kind.filePath(), fileID, err)
	}
	return nil
}

// Parent is a series, movie or artist — the thing a file belongs to.
//
// One struct for all three because the overlap is nearly total, with one
// exception that has to be handled: Lidarr's artist resource has no title. It
// carries artistName and sortName where the others carry title and sortTitle,
// so reading Title directly yields an empty string for every artist. Name and
// SortName resolve that; the raw fields stay exported because a caller that
// knows which app it is talking to may legitimately want them.
type Parent struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	SortTitle  string `json:"sortTitle"`
	ArtistName string `json:"artistName"`
	SortName   string `json:"sortName"`
	Path       string `json:"path"`
	Year       int    `json:"year"`
	Overview   string `json:"overview"`
	// Runtime is MINUTES: per-film for Radarr, per-episode for Sonarr. Lidarr
	// does not report one at artist level.
	Runtime   int        `json:"runtime"`
	Monitored bool       `json:"monitored"`
	Added     *time.Time `json:"added"`
	Genres    []string   `json:"genres"`
	// Studio is Radarr's production company; Network is Sonarr's broadcaster.
	// Two names for the same idea, which the model folds into one Studio row
	// distinguished by kind.
	Studio  string   `json:"studio"`
	Network string   `json:"network"`
	Seasons []Season `json:"seasons"`
	// External ids. Which are populated depends on the app.
	TvdbID          int64  `json:"tvdbId"`
	TmdbID          int64  `json:"tmdbId"`
	ImdbID          string `json:"imdbId"`
	ForeignArtistID string `json:"foreignArtistId"`
}

// Name returns the display title whichever field the app used.
func (p Parent) Name() string {
	if p.Title != "" {
		return p.Title
	}
	return p.ArtistName
}

// SortKey returns the sort title whichever field the app used.
func (p Parent) SortKey() string {
	if p.SortTitle != "" {
		return p.SortTitle
	}
	return p.SortName
}

// parentPath is the collection of top-level items.
func (c *Client) parentPath() string {
	switch c.kind {
	case Radarr:
		return "movie"
	case Lidarr:
		return "artist"
	default:
		return "series"
	}
}

// Parents lists every series, movie or artist the instance tracks.
func (c *Client) Parents(ctx context.Context) ([]Parent, error) {
	var out []Parent
	if err := c.http.Do(ctx, "GET", c.path(c.parentPath()), nil, &out); err != nil {
		return nil, fmt.Errorf("arr %s: parents: %w", c.kind, err)
	}
	return out, nil
}

// Version reports the instance's build.
//
// /system/status is the only endpoint all three serve identically, which makes
// it both the version source and the reachability check.
func (c *Client) Version(ctx context.Context) (string, error) {
	var out struct {
		Version string `json:"version"`
	}
	if err := c.http.Do(ctx, "GET", c.path("system/status"), nil, &out); err != nil {
		return "", fmt.Errorf("arr %s: status: %w", c.kind, err)
	}
	return out.Version, nil
}

// RootFolder is a library root and its free space.
type RootFolder struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
	// Accessible is the *arr's own answer to whether it can write there —
	// a mount that vanished reads as accessible:false rather than an error.
	Accessible bool  `json:"accessible"`
	FreeSpace  int64 `json:"freeSpace"`
	TotalSpace int64 `json:"totalSpace"`
}

// RootFolders lists the configured library roots.
func (c *Client) RootFolders(ctx context.Context) ([]RootFolder, error) {
	var out []RootFolder
	if err := c.http.Do(ctx, "GET", c.path("rootfolder"), nil, &out); err != nil {
		return nil, fmt.Errorf("arr %s: root folders: %w", c.kind, err)
	}
	return out, nil
}

// HealthCheck is one active complaint.
//
// Only current problems appear: a check that has cleared is absent rather than
// resolved, so the record of when it stopped comes from it ceasing to be
// returned.
type HealthCheck struct {
	Source  string `json:"source"`
	Type    string `json:"type"`
	Message string `json:"message"`
	WikiURL string `json:"wikiUrl"`
}

// Health lists the instance's active health checks.
func (c *Client) Health(ctx context.Context) ([]HealthCheck, error) {
	var out []HealthCheck
	if err := c.http.Do(ctx, "GET", c.path("health"), nil, &out); err != nil {
		return nil, fmt.Errorf("arr %s: health: %w", c.kind, err)
	}
	return out, nil
}

// unknownItemsParam is the queue flag that includes items the *arr cannot match
// to anything it tracks.
//
// Named per entity, and worth setting: an unmatched item is exactly the one
// that is stuck, so the default of false hides the failures.
func (k Kind) unknownItemsParam() string {
	switch k {
	case Radarr:
		return "includeUnknownMovieItems"
	case Lidarr:
		return "includeUnknownArtistItems"
	default:
		return "includeUnknownSeriesItems"
	}
}

// QueueItem is one download the *arr is currently tracking.
type QueueItem struct {
	ID       int64  `json:"id"`
	SeriesID int64  `json:"seriesId"`
	MovieID  int64  `json:"movieId"`
	AlbumID  int64  `json:"albumId"`
	ArtistID int64  `json:"artistId"`
	Title    string `json:"title"`
	// DownloadID is the infohash, uppercase.
	DownloadID            string     `json:"downloadId"`
	Status                string     `json:"status"`
	TrackedDownloadStatus string     `json:"trackedDownloadStatus"`
	TrackedDownloadState  string     `json:"trackedDownloadState"`
	ErrorMessage          string     `json:"errorMessage"`
	Size                  float64    `json:"size"`
	Sizeleft              float64    `json:"sizeleft"`
	EstimatedCompletion   *time.Time `json:"estimatedCompletionTime"`
	Indexer               string     `json:"indexer"`
	DownloadClient        string     `json:"downloadClient"`
	Protocol              string     `json:"protocol"`
	Quality               Quality    `json:"quality"`
	StatusMessages        []struct {
		Title    string   `json:"title"`
		Messages []string `json:"messages"`
	} `json:"statusMessages"`
}

// queuePage is the paged envelope the queue is served in.
type queuePage struct {
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

// maxQueuePages bounds a walk. A queue is small by nature; a walk that needs
// more pages than this is a paging bug, not a big queue.
const maxQueuePages = 50

// Queue lists everything the instance is currently waiting on.
func (c *Client) Queue(ctx context.Context, pageSize int) ([]QueueItem, error) {
	if pageSize <= 0 {
		pageSize = 200
	}
	var all []QueueItem
	for page := 1; page <= maxQueuePages; page++ {
		q := url.Values{
			"page":                     {strconv.Itoa(page)},
			"pageSize":                 {strconv.Itoa(pageSize)},
			c.kind.unknownItemsParam(): {"true"},
		}
		var out queuePage
		if err := c.http.Do(ctx, "GET", c.path("queue")+"?"+q.Encode(), nil, &out); err != nil {
			return all, fmt.Errorf("arr %s: queue page %d: %w", c.kind, page, err)
		}
		all = append(all, out.Records...)
		if len(out.Records) < pageSize || len(all) >= out.TotalRecords {
			return all, nil
		}
	}
	return all, fmt.Errorf("arr %s: queue exceeded %d pages", c.kind, maxQueuePages)
}

// BlocklistItem is a release the *arr has refused to take again.
//
// The record of every failed grab that was actively rejected afterwards, which
// history alone does not distinguish from one that simply never imported.
type BlocklistItem struct {
	ID          int64      `json:"id"`
	SeriesID    int64      `json:"seriesId"`
	MovieID     int64      `json:"movieId"`
	ArtistID    int64      `json:"artistId"`
	AlbumID     int64      `json:"albumId"`
	SourceTitle string     `json:"sourceTitle"`
	Date        time.Time  `json:"date"`
	Protocol    string     `json:"protocol"`
	Indexer     string     `json:"indexer"`
	Message     string     `json:"message"`
	DownloadID  string     `json:"downloadId"`
	Quality     Quality    `json:"quality"`
	Languages   []Language `json:"languages"`
}

// blocklistPage is the paged envelope the blocklist is served in.
type blocklistPage struct {
	TotalRecords int             `json:"totalRecords"`
	Records      []BlocklistItem `json:"records"`
}

// Blocklist walks every blocklisted release.
func (c *Client) Blocklist(ctx context.Context, pageSize int) ([]BlocklistItem, error) {
	if pageSize <= 0 {
		pageSize = 500
	}
	var all []BlocklistItem
	for page := 1; page <= maxHistoryPages; page++ {
		q := url.Values{
			"page":     {strconv.Itoa(page)},
			"pageSize": {strconv.Itoa(pageSize)},
		}
		var out blocklistPage
		if err := c.http.Do(ctx, "GET", c.path("blocklist")+"?"+q.Encode(), nil, &out); err != nil {
			return all, fmt.Errorf("arr %s: blocklist page %d: %w", c.kind, page, err)
		}
		all = append(all, out.Records...)
		if len(out.Records) < pageSize || len(all) >= out.TotalRecords {
			return all, nil
		}
	}
	return all, fmt.Errorf("arr %s: blocklist exceeded %d pages", c.kind, maxHistoryPages)
}

// Season is one season of a series, as Sonarr embeds it in the series resource.
//
// There is no /season endpoint: a season exists only as a number inside its
// series, which is why it is read from Parent rather than fetched.
type Season struct {
	SeasonNumber int  `json:"seasonNumber"`
	Monitored    bool `json:"monitored"`
}

// Episode is one episode Sonarr tracks.
//
// EpisodeFileID is 0 when nothing has been imported — that zero is what
// distinguishes a wanted episode from a held one.
type Episode struct {
	ID            int64      `json:"id"`
	SeriesID      int64      `json:"seriesId"`
	TvdbID        int64      `json:"tvdbId"`
	EpisodeFileID int64      `json:"episodeFileId"`
	SeasonNumber  int        `json:"seasonNumber"`
	EpisodeNumber int        `json:"episodeNumber"`
	Title         string     `json:"title"`
	Overview      string     `json:"overview"`
	AirDateUtc    *time.Time `json:"airDateUtc"`
	Runtime       int        `json:"runtime"`
	Monitored     bool       `json:"monitored"`
	HasFile       bool       `json:"hasFile"`
}

// Episodes lists a series' episodes. Sonarr only.
func (c *Client) Episodes(ctx context.Context, seriesID int64) ([]Episode, error) {
	q := url.Values{"seriesId": {strconv.FormatInt(seriesID, 10)}}
	var out []Episode
	if err := c.http.Do(ctx, "GET", c.path("episode")+"?"+q.Encode(), nil, &out); err != nil {
		return nil, fmt.Errorf("arr %s: episodes for series %d: %w", c.kind, seriesID, err)
	}
	return out, nil
}

// Album is one album Lidarr tracks.
type Album struct {
	ID             int64      `json:"id"`
	ArtistID       int64      `json:"artistId"`
	ForeignAlbumID string     `json:"foreignAlbumId"`
	Title          string     `json:"title"`
	Overview       string     `json:"overview"`
	AlbumType      string     `json:"albumType"`
	ReleaseDate    *time.Time `json:"releaseDate"`
	Monitored      bool       `json:"monitored"`
	Added          *time.Time `json:"added"`
	Genres         []string   `json:"genres"`
}

// Albums lists an artist's albums. Lidarr only.
func (c *Client) Albums(ctx context.Context, artistID int64) ([]Album, error) {
	q := url.Values{"artistId": {strconv.FormatInt(artistID, 10)}}
	var out []Album
	if err := c.http.Do(ctx, "GET", c.path("album")+"?"+q.Encode(), nil, &out); err != nil {
		return nil, fmt.Errorf("arr %s: albums for artist %d: %w", c.kind, artistID, err)
	}
	return out, nil
}

// Track is one track Lidarr tracks.
//
// Duration is milliseconds — unlike every runtime elsewhere in the family,
// which are minutes.
type Track struct {
	ID             int64  `json:"id"`
	ArtistID       int64  `json:"artistId"`
	AlbumID        int64  `json:"albumId"`
	TrackFileID    int64  `json:"trackFileId"`
	ForeignTrackID string `json:"foreignTrackId"`
	Title          string `json:"title"`
	TrackNumber    string `json:"trackNumber"`
	MediumNumber   int    `json:"mediumNumber"`
	Duration       int64  `json:"duration"`
	Monitored      bool   `json:"monitored"`
	HasFile        bool   `json:"hasFile"`
}

// Tracks lists an artist's tracks. Lidarr only.
func (c *Client) Tracks(ctx context.Context, artistID int64) ([]Track, error) {
	q := url.Values{"artistId": {strconv.FormatInt(artistID, 10)}}
	var out []Track
	if err := c.http.Do(ctx, "GET", c.path("track")+"?"+q.Encode(), nil, &out); err != nil {
		return nil, fmt.Errorf("arr %s: tracks for artist %d: %w", c.kind, artistID, err)
	}
	return out, nil
}
