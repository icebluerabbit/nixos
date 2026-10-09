package inventory

import "testing"

func TestEntryTrack(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"ordinary track", "/music/Album/01 - Klaxon.flac", "01 - Klaxon"},
		{"dots in the name", "/music/Album/01. Feat. Someone.flac", "01. Feat. Someone"},
		{"no extension", "/music/Album/track", "track"},
		{"no directory", "track.flac", "track"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Entry{AudioPath: tt.path}).Track(); got != tt.want {
				t.Errorf("Track() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGroupLabel(t *testing.T) {
	tests := []struct {
		name  string
		group Group
		want  string
	}{
		{
			name:  "artist and album",
			group: Group{Dir: "/music/i-dle/I SWAY (2024)", Artist: "(G)I-DLE", Album: "I SWAY"},
			want:  "(G)I-DLE — I SWAY",
		},
		{
			name:  "untagged falls back to the directory",
			group: Group{Dir: "/music/i-dle/I SWAY (2024)"},
			want:  "unknown artist — I SWAY (2024)",
		},
		{
			name:  "disagreeing artists are reported",
			group: Group{Dir: "/d", Artist: "A", Album: "B", MixedArtist: true},
			want:  "A — B (tracks disagree on artist)",
		},
		{
			name:  "disagreeing albums are reported",
			group: Group{Dir: "/d", Artist: "A", Album: "B", MixedAlbum: true},
			want:  "A — B (tracks disagree on album)",
		},
		{
			name:  "disagreeing on both",
			group: Group{Dir: "/d", Artist: "A", Album: "B", MixedArtist: true, MixedAlbum: true},
			want:  "A — B (tracks disagree on artist and album)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.Label(); got != tt.want {
				t.Errorf("Label() = %q, want %q", got, tt.want)
			}
		})
	}
}
