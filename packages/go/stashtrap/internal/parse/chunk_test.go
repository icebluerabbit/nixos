package parse_test

import (
	"slices"
	"testing"

	"homelab/stashtrap/internal/parse"
)

// texts returns the chunk texts, which is what the catalogue is asked about.
func texts(chunks []parse.Chunk) []string {
	out := make([]string, 0, len(chunks))
	for _, c := range chunks {
		out = append(out, c.Text)
	}
	return out
}

func TestReadDetachesHeaderAndHoistsDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		body string
		date string
	}{
		{"bracketed header", "[FirstSite] Jane Doe - A Title 1080p", "jane doe a title", ""},
		{"parenthesised header", "(FirstSite.com) Jane Doe - A Title", "jane doe a title", ""},
		{"leading date", "[FirstSite] 26.09.09 Jane Doe [1080p]", "jane doe", "26.09.09"},
		{"trailing date", "[FirstSite] Jane Doe - A Title (2019-10-18) [2160p]", "jane doe a title", "2019-10-18"},
		{"month-name date", "[FirstSite] Jane Doe - A Title - Aug 14, 2026 mp4", "jane doe a title", "Aug 14, 2026"},
		{"no header", "Jane Doe - A Title", "jane doe a title", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := parse.Read(tc.in)
			if got.Date != tc.date {
				t.Fatalf("date = %q, want %q", got.Date, tc.date)
			}
			// The body chunk covering everything is the whole body.
			var whole string
			for _, c := range got.Chunks {
				if c.Position == parse.PositionBody && c.Text > whole {
					_ = c
				}
			}
			if !slices.Contains(texts(got.Chunks), tc.body) {
				t.Fatalf("chunks %q do not include the whole body %q", texts(got.Chunks), tc.body)
			}
		})
	}
}

// Chunks are offered both split and unsplit, because a delimiter is a real
// boundary often enough to use and unreliable often enough not to trust.
func TestReadOffersSplitAndUnsplitChunks(t *testing.T) {
	t.Parallel()

	got := texts(parse.Read("[FirstSite] Jane Doe, Second Performer - A Title").Chunks)
	for _, want := range []string{
		"jane doe second performer a title", // unsplit
		"jane doe",                          // split on the comma
		"second performer",
		"a title",
		"firstsite", // the header
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("chunks %q missing %q", got, want)
		}
	}
}

// A TLD ends a chunk and is dropped: the catalogue knows "adulttime", the
// release writes "AdultTime.com", and a dot cannot split in general because
// "jane.doe" is one name.
func TestReadSplitsHeaderOnTLDs(t *testing.T) {
	t.Parallel()

	got := texts(parse.Read("[AdultTime.com / Transfixed.com] Jane Doe").Chunks)
	for _, want := range []string{"adulttime", "transfixed"} {
		if !slices.Contains(got, want) {
			t.Fatalf("chunks %q missing %q", got, want)
		}
	}
	if slices.Contains(got, "com") {
		t.Fatalf("chunks %q kept a TLD as a name", got)
	}
}

// Joining words separate rather than name, and never form a chunk of their own.
func TestReadDropsJoiners(t *testing.T) {
	t.Parallel()

	got := texts(parse.Read("[FirstSite] Jane Doe and Second Performer").Chunks)
	for _, want := range []string{"jane doe", "second performer"} {
		if !slices.Contains(got, want) {
			t.Fatalf("chunks %q missing %q", got, want)
		}
	}
	for _, unwanted := range []string{"and", "jane doe and"} {
		if slices.Contains(got, unwanted) {
			t.Fatalf("chunks %q kept the joiner: %q", got, unwanted)
		}
	}
}

// Spans locate a chunk in the whole name, which is what lets two hits on the
// same words be recognised as one piece of evidence.
func TestReadSpansAreUniqueAcrossHeaderAndBody(t *testing.T) {
	t.Parallel()

	reading := parse.Read("[FirstSite] Jane Doe - A Title")
	seen := map[[2]int]string{}
	for _, c := range reading.Chunks {
		key := [2]int{c.Start, c.End}
		if prev, dup := seen[key]; dup {
			t.Fatalf("chunks %q and %q share span %v", prev, c.Text, key)
		}
		seen[key] = c.Text
		if c.Start >= c.End {
			t.Fatalf("chunk %q has empty span %v", c.Text, key)
		}
	}
}

func TestReadOfEmptyName(t *testing.T) {
	t.Parallel()

	if got := parse.Read("1080p").Chunks; len(got) != 0 {
		t.Fatalf("chunks = %q, want none for a name that is all technical tokens", texts(got))
	}
}
