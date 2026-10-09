// Package qbittorrent talks to a qBittorrent WebUI API.
//
// Only the read surface needed to reconcile client state is modelled: the
// torrent list, a torrent's files, and its trackers. Nothing here mutates the
// client — deletion and tagging are deliberately absent so that a bug in a
// reader cannot remove a torrent.
//
// Authentication is not modelled. This homelab puts the media VLAN in
// qBittorrent's authSubnetWhitelist with WebUILocalHostAuth disabled, so
// callers on that VLAN are already authorised. Add a cookie or header through
// httpx.Config if that stops being true.
//
// https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)
package qbittorrent

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"homelab/httpx"
)

const (
	pathTorrentsInfo     = "/api/v2/torrents/info"
	pathTorrentsFiles    = "/api/v2/torrents/files"
	pathTorrentsTrackers = "/api/v2/torrents/trackers"
)

// Client is a qBittorrent WebUI API client.
type Client struct {
	http *httpx.Client
}

// ErrNoHash reports a call made with an empty infohash.
var ErrNoHash = errors.New("qbittorrent: hash is required")

// New builds a client for the WebUI at baseURL.
func New(cfg httpx.Config) (*Client, error) {
	c, err := httpx.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{http: c}, nil
}

// TorrentState is the serialized form of BitTorrent::TorrentState, from
// src/webui/api/serialize/serialize_torrent.cpp.
//
// Note the vocabulary changed: what older builds and most documentation call
// "paused" is serialized "stopped*" here. A check for "pausedUP" matches
// nothing and reports no error.
type TorrentState = string

// Every state the WebUI can emit. "unknown" is the serializer's own default for
// a state it has no case for, so it arrives over the wire rather than being
// this package's invention.
const (
	StateError              TorrentState = "error"
	StateMissingFiles       TorrentState = "missingFiles"
	StateUploading          TorrentState = "uploading"
	StateStoppedUP          TorrentState = "stoppedUP"
	StateQueuedUP           TorrentState = "queuedUP"
	StateStalledUP          TorrentState = "stalledUP"
	StateCheckingUP         TorrentState = "checkingUP"
	StateForcedUP           TorrentState = "forcedUP"
	StateDownloading        TorrentState = "downloading"
	StateMetaDL             TorrentState = "metaDL"
	StateForcedMetaDL       TorrentState = "forcedMetaDL"
	StateStoppedDL          TorrentState = "stoppedDL"
	StateQueuedDL           TorrentState = "queuedDL"
	StateStalledDL          TorrentState = "stalledDL"
	StateCheckingDL         TorrentState = "checkingDL"
	StateForcedDL           TorrentState = "forcedDL"
	StateCheckingResumeData TorrentState = "checkingResumeData"
	StateMoving             TorrentState = "moving"
	StateUnknown            TorrentState = "unknown"
)

// knownStates is every value the serializer can produce.
var knownStates = map[TorrentState]struct{}{
	StateError: {}, StateMissingFiles: {}, StateUploading: {},
	StateStoppedUP: {}, StateQueuedUP: {}, StateStalledUP: {},
	StateCheckingUP: {}, StateForcedUP: {}, StateDownloading: {},
	StateMetaDL: {}, StateForcedMetaDL: {}, StateStoppedDL: {},
	StateQueuedDL: {}, StateStalledDL: {}, StateCheckingDL: {},
	StateForcedDL: {}, StateCheckingResumeData: {}, StateMoving: {},
	StateUnknown: {},
}

// IsKnownState reports whether s is a state this build knows about. A false
// means qBittorrent gained a state — worth surfacing rather than storing blind.
func IsKnownState(s TorrentState) bool {
	_, ok := knownStates[s]
	return ok
}

// IsSeeding reports whether the torrent is in one of the upload-side states.
// Derived from the enum rather than from a substring match on "UP", which would
// also catch nothing useful and miss "uploading".
func IsSeeding(s TorrentState) bool {
	switch s {
	case StateUploading, StateStalledUP, StateQueuedUP, StateForcedUP, StateCheckingUP:
		return true
	default:
		return false
	}
}

// Torrent is one entry of the torrent list.
//
// Field set is deliberately narrow. Rates, peer counts and progress are
// omitted: they are instantaneous and a reconcile that stored them would be
// recording a sample, not a fact.
type Torrent struct {
	// Hash is the torrent id. For a v1 torrent it equals InfoHashV1; for a
	// v2 or hybrid one it does not, so both are decoded rather than assuming
	// the id is an infohash. The *arr apps report the v1 hash as downloadId,
	// which is what the join across the two systems is built on.
	Hash       string       `json:"hash"`
	InfoHashV1 string       `json:"infohash_v1"`
	InfoHashV2 string       `json:"infohash_v2"`
	Name       string       `json:"name"`
	Category   string       `json:"category"`
	Tags       string       `json:"tags"`
	State      TorrentState `json:"state"`
	// Private reports whether the torrent is on a private tracker, which is
	// what decides whether hit-and-run rules apply at all. Null before
	// metadata arrives, hence the pointer.
	Private *bool `json:"private"`
	// SavePath is where the client writes; ContentPath is the torrent's own
	// root, which for a single-file torrent is the file itself.
	SavePath    string `json:"save_path"`
	ContentPath string `json:"content_path"`
	TotalSize   int64  `json:"total_size"`
	// AddedOn is a unix timestamp from the client. It survives restarts but
	// is reset by a re-add, so it is a hint rather than an origin.
	AddedOn int64 `json:"added_on"`
	// CompletionOn is unix, zero while incomplete.
	CompletionOn int64 `json:"completion_on"`
	// Tracker is the currently-working announce URL, empty when none is.
	// The full list, including dead ones, comes from Trackers.
	Tracker string `json:"tracker"`

	// Progress is how much of the torrent is present, 0 to 1.
	//
	// Decoded despite being a point-in-time value, unlike the transfer rates:
	// it is the only field that distinguishes a stalled partial download from
	// a complete one that is merely idle, and State does not — stalledDL
	// covers both a torrent at 3% and one at 99%.
	Progress float64 `json:"progress"`

	// UpSpeed is the instantaneous upload rate in bytes/s.
	//
	// The one rate field decoded, and only because it ranks: a sampler that
	// can afford to look at some of the active torrents wants the ones
	// carrying the traffic, and no cumulative counter answers that — a
	// torrent with a huge Uploaded may have gone quiet months ago. Nothing
	// should store this.
	UpSpeed int64 `json:"upspeed"`

	// Cumulative counters. Facts about the torrent's whole life, unlike the
	// rate fields, which are instantaneous and deliberately not decoded.
	Ratio      float64 `json:"ratio"`
	Uploaded   int64   `json:"uploaded"`
	Downloaded int64   `json:"downloaded"`
	// SeedingTime is finishedTime(): seconds since the torrent completed.
	// It RESETS on pause and recheck, so it is a cross-check rather than an
	// authority — 61 torrents here read as 7-14 days old when they were
	// 30-90 because of exactly this.
	SeedingTime  int64 `json:"seeding_time"`
	TimeActive   int64 `json:"time_active"`
	LastActivity int64 `json:"last_activity"`

	// Torrent metadata, fixed at creation.
	Comment      string `json:"comment"`
	CreatedBy    string `json:"created_by"`
	CreationDate int64  `json:"creation_date"`
	MagnetURI    string `json:"magnet_uri"`
}

// File is one entry of a torrent's content listing.
type File struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Priority int    `json:"priority"`
}

// PrioritySkip is the priority value for a file excluded from the download.
// Such files never exist on disk, so a reconcile must not stat them.
const PrioritySkip = 0

// Tracker is one announce endpoint of a torrent, including ones that are
// failing — the status and message are the point of reading this.
type Tracker struct {
	URL    string `json:"url"`
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}

// Tracker status values as reported by the WebUI API.
//
// The docs describe 0-4. Observed traffic also carries 5, on trackers answering
// "Torrent has been deleted." — the unregistered case. It is named here from
// what it was seen doing rather than from documentation, so treat the name as
// descriptive and the raw code as authoritative.
const (
	TrackerDisabled     = 0
	TrackerNotContacted = 1
	TrackerWorking      = 2
	TrackerUpdating     = 3
	TrackerNotWorking   = 4
	TrackerError        = 5
)

// unregisteredMarkers are the tracker messages that mean the torrent is no
// longer registered. Trackers word it differently and none of it is
// standardised, so this is a list of observed phrasings rather than a protocol.
var unregisteredMarkers = []string{
	"torrent has been deleted",
	"unregistered torrent",
	"torrent not registered",
	"not registered with this tracker",
	"torrent not found",
}

// IsUnregistered reports whether a tracker message says the torrent is gone
// from the tracker. Matching is case-insensitive and by substring because the
// messages carry punctuation and sometimes an id.
func IsUnregistered(msg string) bool {
	low := strings.ToLower(msg)
	for _, m := range unregisteredMarkers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// Filters for TorrentsFiltered, as /torrents/info accepts them. Only the ones
// used here are named; the endpoint takes more.
const (
	// FilterActive is the client's own notion of active: transferring in
	// either direction right now. It is the cheap way to bound work that
	// scales per torrent, because it tracks what the client is doing rather
	// than how much it holds.
	FilterActive = "active"
)

// Torrents lists every torrent the client holds.
func (c *Client) Torrents(ctx context.Context) ([]Torrent, error) {
	return c.TorrentsFiltered(ctx, "")
}

// TorrentsFiltered lists the torrents matching one of the endpoint's filters.
// An empty filter lists everything.
func (c *Client) TorrentsFiltered(ctx context.Context, filter string) ([]Torrent, error) {
	path := pathTorrentsInfo
	if filter != "" {
		path += "?" + url.Values{"filter": {filter}}.Encode()
	}
	var out []Torrent
	if err := c.http.Do(ctx, "GET", path, nil, &out); err != nil {
		return nil, fmt.Errorf("qbittorrent: list torrents: %w", err)
	}
	return out, nil
}

// Files lists the content of one torrent.
func (c *Client) Files(ctx context.Context, hash string) ([]File, error) {
	if hash == "" {
		return nil, ErrNoHash
	}
	var out []File
	path := pathTorrentsFiles + "?" + url.Values{"hash": {hash}}.Encode()
	if err := c.http.Do(ctx, "GET", path, nil, &out); err != nil {
		return nil, fmt.Errorf("qbittorrent: files for %s: %w", hash, err)
	}
	return out, nil
}

// Trackers lists the announce endpoints of one torrent.
//
// qBittorrent prepends pseudo-entries for DHT, PeX and LSD whose URLs are not
// URLs at all ("** [DHT] **"). They are returned as-is; callers that only want
// real trackers should skip Status == TrackerDisabled.
func (c *Client) Trackers(ctx context.Context, hash string) ([]Tracker, error) {
	if hash == "" {
		return nil, ErrNoHash
	}
	var out []Tracker
	path := pathTrackersQuery(hash)
	if err := c.http.Do(ctx, "GET", path, nil, &out); err != nil {
		return nil, fmt.Errorf("qbittorrent: trackers for %s: %w", hash, err)
	}
	return out, nil
}

func pathTrackersQuery(hash string) string {
	return pathTorrentsTrackers + "?" + url.Values{"hash": {hash}}.Encode()
}

// AnnounceHost extracts the host from an announce URL, which is how trackers
// are identified across the model — one tracker owns several announce domains
// (torrentleech.org and tleechreload.org, pretome.info and preto.me) and the
// host is the only stable key the client reports.
//
// Returns "" for qBittorrent's DHT/PeX/LSD pseudo-entries.
func AnnounceHost(announce string) string {
	u, err := url.Parse(announce)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Hostname()
}
