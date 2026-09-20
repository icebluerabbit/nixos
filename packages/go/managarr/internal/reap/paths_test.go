package reap

import "testing"

// The defect this package was written around.
//
// qBittorrent reports content_path as the FILE when a torrent holds one file
// inside a folder. Comparing that against directory entries on disk makes every
// such torrent look unclaimed while it is actively seeding — which, acted on,
// deletes live data. A hand-rolled version of this comparison reported 2.8TB of
// orphans against a real answer of 104GB for exactly this reason.
func TestFileInsideFolderClaimsItsFolder(t *testing.T) {
	const root = "/mnt/media/complete"
	// What qBittorrent reports for a single-file-in-folder torrent.
	reported := "/mnt/media/complete/books/Some Author - A Book/book.epub"

	got, ok := entryUnder(root, reported)
	if !ok {
		t.Fatalf("entryUnder(%q) not under root", reported)
	}
	if want := "/mnt/media/complete/books/Some Author - A Book"; got != want {
		t.Errorf("got %q, want %q — the FOLDER must be claimed, not the file", got, want)
	}
}

func TestEntryUnder(t *testing.T) {
	const root = "/mnt/media/complete"
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{
			// The simple case: a single-file torrent stored directly in its
			// category. The entry is the file itself.
			name: "file directly in category",
			in:   "/mnt/media/complete/tv/Show.S01E01.mkv",
			want: "/mnt/media/complete/tv/Show.S01E01.mkv",
			ok:   true,
		},
		{
			name: "multi-file torrent folder",
			in:   "/mnt/media/complete/movies/Some.Movie.2024",
			want: "/mnt/media/complete/movies/Some.Movie.2024",
			ok:   true,
		},
		{
			name: "file nested two deep still folds to the entry",
			in:   "/mnt/media/complete/movies/Some.Movie.2024/Subs/eng.srt",
			want: "/mnt/media/complete/movies/Some.Movie.2024",
			ok:   true,
		},
		{
			// The cross-seed tree is <tracker>/<entry> rather than
			// <category>/<entry>, but the shape is the same two levels.
			name: "cross-seed link tree",
			in:   "/mnt/media/cross-seed/links/YU-Scene/Show.S01E08.mkv",
			want: "",
			ok:   false, // not under this root; see the links root test below
		},
		{
			name: "the category directory itself is not an entry",
			in:   "/mnt/media/complete/tv",
			want: "",
			ok:   false,
		},
		{
			name: "the root itself is not an entry",
			in:   "/mnt/media/complete",
			want: "",
			ok:   false,
		},
		{
			// Must not match: a sibling directory whose name merely starts with
			// the root. Prefix comparison without the separator would.
			name: "sibling root with shared prefix",
			in:   "/mnt/media/complete-old/tv/Show.mkv",
			want: "",
			ok:   false,
		},
		{
			name: "unrelated path",
			in:   "/var/tmp/qbittorrent/tv/Show.mkv",
			want: "",
			ok:   false,
		},
		{
			name: "trailing slash is tolerated",
			in:   "/mnt/media/complete/tv/Show.S01E01.mkv/",
			want: "/mnt/media/complete/tv/Show.S01E01.mkv",
			ok:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := entryUnder(root, c.in)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// The cross-seed tree sits one level deeper in absolute terms, which is why the
// root is passed in rather than assumed.
func TestEntryUnderLinksRoot(t *testing.T) {
	const root = "/mnt/media/cross-seed/links"
	got, ok := entryUnder(root, "/mnt/media/cross-seed/links/YU-Scene/Show.S01E08.mkv")
	if !ok {
		t.Fatal("not under links root")
	}
	if want := "/mnt/media/cross-seed/links/YU-Scene/Show.S01E08.mkv"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Several reported paths inside one entry collapse to a single claim.
func TestClaimedEntriesFolds(t *testing.T) {
	const root = "/mnt/media/complete"
	claimed := claimedEntries(root, []string{
		"/mnt/media/complete/movies/Film.2024/Film.mkv",
		"/mnt/media/complete/movies/Film.2024/Subs/eng.srt",
		"/mnt/media/complete/tv/Show.S01E01.mkv",
		"/var/tmp/elsewhere/thing",
	})

	if len(claimed) != 2 {
		t.Fatalf("claimed %d entries, want 2: %v", len(claimed), claimed)
	}
	for _, want := range []string{
		"/mnt/media/complete/movies/Film.2024",
		"/mnt/media/complete/tv/Show.S01E01.mkv",
	} {
		if _, ok := claimed[want]; !ok {
			t.Errorf("%q not claimed", want)
		}
	}
}

func TestCategoryOf(t *testing.T) {
	const root = "/mnt/media/complete"
	if got := categoryOf(root, "/mnt/media/complete/books/A Book"); got != "books" {
		t.Errorf("got %q, want books", got)
	}
	if got := categoryOf(root, "/mnt/media/complete/tv/Show.mkv"); got != "tv" {
		t.Errorf("got %q, want tv", got)
	}
}
