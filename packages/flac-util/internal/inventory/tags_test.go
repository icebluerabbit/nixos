package inventory

import (
	"testing"

	"flac-util/internal/report"
)

// tagged returns a report carrying the given comments.
func tagged(comments ...[2]string) *report.Report {
	return &report.Report{Tags: report.Tags{HasVorbisComment: true, Comments: comments}}
}

func TestComment(t *testing.T) {
	tags := report.Tags{Comments: [][2]string{
		{"ALBUMARTIST", ""},
		{"album artist", "  (G)I-DLE  "},
		{"ALBUM", "I SWAY"},
	}}

	tests := []struct {
		name string
		keys []string
		want string
	}{
		{"empty values are passed over", []string{"ALBUMARTIST", "ALBUM ARTIST"}, "(G)I-DLE"},
		{"keys match without regard to case", []string{"AlBuM"}, "I SWAY"},
		{"absent key", []string{"COMPOSER"}, ""},
		{"no keys", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Comment(tags, tt.keys...); got != tt.want {
				t.Errorf("Comment() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAlbumArtistFallsBackToArtist(t *testing.T) {
	r := tagged([2]string{"ARTIST", "Parkway Drive"})
	if got := AlbumArtist(r); got != "Parkway Drive" {
		t.Errorf("AlbumArtist() = %q, want %q", got, "Parkway Drive")
	}
}

func TestAlbumArtistPrefersTheAlbumArtist(t *testing.T) {
	r := tagged(
		[2]string{"ARTIST", "Someone Featured"},
		[2]string{"ALBUMARTIST", "The Band"},
	)
	if got := AlbumArtist(r); got != "The Band" {
		t.Errorf("AlbumArtist() = %q, want %q", got, "The Band")
	}
}

func TestTagsOfNothing(t *testing.T) {
	if got := AlbumArtist(nil); got != "" {
		t.Errorf("AlbumArtist(nil) = %q, want empty", got)
	}
	if got := Album(nil); got != "" {
		t.Errorf("Album(nil) = %q, want empty", got)
	}
}

func TestDescribeNamesTheGroup(t *testing.T) {
	g := Group{Entries: []Entry{
		{Report: tagged([2]string{"ALBUMARTIST", "(G)I-DLE"}, [2]string{"ALBUM", "I SWAY"})},
		{Report: tagged([2]string{"ALBUMARTIST", "(G)I-DLE"}, [2]string{"ALBUM", "I SWAY"})},
	}}
	g.describe()

	if g.Artist != "(G)I-DLE" || g.Album != "I SWAY" {
		t.Errorf("described as %q / %q", g.Artist, g.Album)
	}
	if g.MixedArtist || g.MixedAlbum {
		t.Error("agreeing tracks were reported as disagreeing")
	}
}

func TestDescribeRecordsDisagreement(t *testing.T) {
	g := Group{Entries: []Entry{
		{Report: tagged([2]string{"ALBUMARTIST", "(G)I-DLE"}, [2]string{"ALBUM", "I SWAY"})},
		{Report: tagged([2]string{"ALBUMARTIST", "i-dle"}, [2]string{"ALBUM", "I SWAY"})},
	}}
	g.describe()

	if !g.MixedArtist {
		t.Error("MixedArtist = false, want true")
	}
	if g.MixedAlbum {
		t.Error("MixedAlbum = true, want false")
	}
	if g.Artist != "(G)I-DLE" {
		t.Errorf("Artist = %q, want the first one seen", g.Artist)
	}
}

func TestDescribeIgnoresUntaggedTracks(t *testing.T) {
	// A track with no report must not be read as a track that disagrees, or
	// every part-way-collected album would claim its tags were inconsistent.
	g := Group{Entries: []Entry{
		{Report: tagged([2]string{"ALBUMARTIST", "SUNMI"}, [2]string{"ALBUM", "꼬리"})},
		{Report: nil},
	}}
	g.describe()

	if g.MixedArtist || g.MixedAlbum {
		t.Error("a track with no report was counted as a disagreement")
	}
	if g.Artist != "SUNMI" {
		t.Errorf("Artist = %q, want SUNMI", g.Artist)
	}
}
