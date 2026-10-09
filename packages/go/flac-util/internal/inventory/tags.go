package inventory

import (
	"strings"

	"homelab/flac-util/internal/report"
)

// Keys a Vorbis comment block may use for the same thing. Taggers disagree,
// and a file that has been through more than one of them carries several at
// once, so the first non-empty match wins rather than the first key present.
var (
	albumArtistKeys = []string{"ALBUMARTIST", "ALBUM ARTIST", "ALBUM_ARTIST", "ARTIST"}
	albumKeys       = []string{"ALBUM"}
)

// Comment returns the first non-empty value stored under any of the given
// keys, compared without regard to case.
func Comment(tags report.Tags, keys ...string) string {
	for _, key := range keys {
		for _, comment := range tags.Comments {
			if !strings.EqualFold(comment[0], key) {
				continue
			}
			if value := strings.TrimSpace(comment[1]); value != "" {
				return value
			}
		}
	}
	return ""
}

// AlbumArtist returns the artist a report's track is filed under.
func AlbumArtist(r *report.Report) string {
	if r == nil {
		return ""
	}
	return Comment(r.Tags, albumArtistKeys...)
}

// Album returns the album a report's track belongs to.
func Album(r *report.Report) string {
	if r == nil {
		return ""
	}
	return Comment(r.Tags, albumKeys...)
}

// describe fills in a group's name from the tags of its tracks, and records
// where they disagreed.
func (g *Group) describe() {
	for _, entry := range g.Entries {
		artist, album := AlbumArtist(entry.Report), Album(entry.Report)

		if artist != "" {
			switch {
			case g.Artist == "":
				g.Artist = artist
			case g.Artist != artist:
				g.MixedArtist = true
			}
		}
		if album != "" {
			switch {
			case g.Album == "":
				g.Album = album
			case g.Album != album:
				g.MixedAlbum = true
			}
		}
	}
}
