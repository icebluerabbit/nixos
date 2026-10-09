// Package inventory pairs tracks with the reports written beside them and
// arranges them for reading.
//
// It groups tracks by the directory they sit in, because that is what actually
// guarantees they belong together, and marks the values in a group that differ
// from their neighbours. That marking is the whole point: a single track that
// was not encoded like its siblings is what a reader is looking for, and it is
// invisible when each report is read on its own.
//
// Nothing here judges a value. A number is marked because it is unlike the
// others in its group, never because it is good or bad.
package inventory

import (
	"path/filepath"
	"strings"

	"homelab/flac-util/internal/report"
)

// Entry is one track and whatever is known about it.
type Entry struct {
	// AudioPath is the track this entry describes, whether or not it exists.
	AudioPath string
	// State is how the report and the track relate.
	State report.State
	// Report is the report read from disk, absent when there is none.
	Report *report.Report
	// Err is why this entry could not be read, if it could not be.
	Err error
}

// Track returns the track's name without its directory or extension.
func (e Entry) Track() string {
	base := filepath.Base(e.AudioPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Group is the set of tracks sharing a directory.
type Group struct {
	// Dir is the directory the tracks were found in.
	Dir string
	// Artist and Album name the group, taken from the tags of the tracks in
	// it. Either may be empty when nothing was tagged.
	Artist string
	Album  string
	// MixedArtist and MixedAlbum record that the tracks disagreed. The group
	// is still one group, since they share a directory; the disagreement is
	// reported rather than acted on.
	MixedArtist bool
	MixedAlbum  bool
	// Entries are the tracks, in the order they should be shown.
	Entries []Entry
}

// Label names the group for a header.
func (g Group) Label() string {
	artist, album := g.Artist, g.Album
	if artist == "" {
		artist = "unknown artist"
	}
	if album == "" {
		album = filepath.Base(g.Dir)
	}

	label := artist + " — " + album
	switch {
	case g.MixedArtist && g.MixedAlbum:
		label += " (tracks disagree on artist and album)"
	case g.MixedArtist:
		label += " (tracks disagree on artist)"
	case g.MixedAlbum:
		label += " (tracks disagree on album)"
	}
	return label
}
