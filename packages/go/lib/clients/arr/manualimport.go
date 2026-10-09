package arr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

// nameManualImport is the command that imports an explicit list of files.
//
// Unlike DownloadedAlbumsScan it does not score the files against the
// releases the app holds, so an identification the app itself produced is
// acted on rather than re-checked against a threshold it cannot pass.
const nameManualImport = "ManualImport"

// Import modes the command accepts.
const (
	// ImportCopy leaves the source where it is. With the app's
	// copyUsingHardlinks on the same filesystem it links rather than
	// duplicates, so the source survives as its own tree for free.
	ImportCopy = "Copy"
	ImportMove = "Move"
	ImportAuto = "Auto"
)

// ErrNotLidarr reports a Lidarr-shaped call made on another app.
var ErrNotLidarr = errors.New("arr: this endpoint is modelled for Lidarr only")

// Ref is an id-bearing object in a manual import listing.
//
// Null and {} both decode to the zero value, which is what the app sends for
// something it could not resolve — so an unresolved reference is an id of 0
// rather than an absence to test for.
type Ref struct {
	ID int64 `json:"id"`
}

// Rejection is one reason the app would refuse a file.
type Rejection struct {
	Reason string `json:"reason"`
	Type   string `json:"type"`
}

// ManualImportItem is one file of a manual import listing, in Lidarr's shape.
//
// Sonarr and Radarr serve the same endpoint with series/episodes and movie
// fields instead, which is why ManualImport refuses to run against them: the
// ids would all decode as zero and the import would look merely unresolved.
type ManualImportItem struct {
	Path           string `json:"path"`
	Artist         Ref    `json:"artist"`
	Album          Ref    `json:"album"`
	AlbumReleaseID int64  `json:"albumReleaseId"`
	Tracks         []Ref  `json:"tracks"`
	// Quality is kept as raw JSON and handed back verbatim. It is the app's
	// own quality model, which this package has no reason to understand and
	// every reason not to re-encode: a field dropped in the round trip is a
	// different quality than the one identified.
	Quality    json.RawMessage `json:"quality"`
	Rejections []Rejection     `json:"rejections"`
}

// Placed reports whether the app resolved everything needed to import the
// file. Anything short of all four leaves no track to import it as.
func (i ManualImportItem) Placed() bool {
	return i.Artist.ID != 0 && i.Album.ID != 0 && i.AlbumReleaseID != 0 && len(i.Tracks) > 0
}

// Reasons lists why the app would refuse this file.
func (i ManualImportItem) Reasons() []string {
	out := make([]string, 0, len(i.Rejections))
	for _, r := range i.Rejections {
		reason := r.Reason
		if reason == "" {
			reason = "no reason given"
		}
		out = append(out, reason)
	}
	if len(out) == 0 {
		return []string{"no reason given"}
	}
	return out
}

// ManualImport asks the app what it makes of a folder.
//
// This is the same identification an unattended scan performs: artist, album,
// release and tracks per file, plus any reason it would have refused to act
// on them.
//
// filterExistingFiles=false is passed so the listing describes the folder as
// it is rather than as a diff against the library.
func (c *Client) ManualImport(ctx context.Context, folder string) ([]ManualImportItem, error) {
	if c.kind != Lidarr {
		return nil, fmt.Errorf("%w: client is %s", ErrNotLidarr, c.kind)
	}
	query := url.Values{"folder": {folder}, "filterExistingFiles": {"false"}}

	var out []ManualImportItem
	if err := c.http.Do(ctx, "GET", c.path("manualimport")+"?"+query.Encode(), nil, &out); err != nil {
		return nil, fmt.Errorf("arr: manual import listing for %s: %w", folder, err)
	}
	return out, nil
}

// ManualImportFile is one file of the ManualImport command.
type ManualImportFile struct {
	Path           string          `json:"path"`
	ArtistID       int64           `json:"artistId"`
	AlbumID        int64           `json:"albumId"`
	AlbumReleaseID int64           `json:"albumReleaseId"`
	TrackIDs       []int64         `json:"trackIds"`
	Quality        json.RawMessage `json:"quality"`
	// DisableReleaseSwitching pins the files to the release the app already
	// holds. Left false so it may still adopt the release the files
	// actually are, which is the point of tagging them from the disc.
	DisableReleaseSwitching bool `json:"disableReleaseSwitching"`
}

// ManualImportCommand is the command body.
type ManualImportCommand struct {
	Name                 string             `json:"name"`
	ImportMode           string             `json:"importMode"`
	ReplaceExistingFiles bool               `json:"replaceExistingFiles"`
	Files                []ManualImportFile `json:"files"`
}

// NewManualImport builds the command for exactly what the app identified.
func NewManualImport(items []ManualImportItem, mode string) ManualImportCommand {
	files := make([]ManualImportFile, 0, len(items))
	for _, item := range items {
		trackIDs := make([]int64, 0, len(item.Tracks))
		for _, track := range item.Tracks {
			trackIDs = append(trackIDs, track.ID)
		}
		files = append(files, ManualImportFile{
			Path:                    item.Path,
			ArtistID:                item.Artist.ID,
			AlbumID:                 item.Album.ID,
			AlbumReleaseID:          item.AlbumReleaseID,
			TrackIDs:                trackIDs,
			Quality:                 item.Quality,
			DisableReleaseSwitching: false,
		})
	}
	return ManualImportCommand{
		Name:                 nameManualImport,
		ImportMode:           mode,
		ReplaceExistingFiles: false,
		Files:                files,
	}
}
