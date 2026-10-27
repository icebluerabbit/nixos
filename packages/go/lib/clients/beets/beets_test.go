package beets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"homelab/httpx"
)

// newClient points a client at a handler standing in for beets' web plugin.
func newClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := New(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestItems(t *testing.T) {
	var gotPath string
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[
			{"id":1,"path":"/music/Artist/Album/01 Track.flac",
			 "mb_trackid":"t-1","mb_albumid":"a-1","mb_releasegroupid":"rg-1",
			 "mb_artistid":"ar-1","acoustid_id":"ac-1","isrc":"USXX11700001",
			 "album":"Album","albumartist":"Artist","artist":"Artist",
			 "title":"Track","track":1,"disc":1,"year":2019,
			 "label":"Label","country":"US","media":"CD",
			 "format":"FLAC","bitrate":971000,"bitrate_mode":"","bitdepth":16,
			 "samplerate":44100,"channels":2,"length":213.5,
			 "added":1690000000.5},
			{"id":2,"path":"/music/Other/Untagged/02.mp3"}
		]}`))
	})

	items, err := c.Items(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/item/" {
		t.Errorf("path = %q, want /item/", gotPath)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	// Every field this package names, checked against the wire shape. beets'
	// items table has around ninety columns; a rename upstream has to fail
	// here rather than silently zero a field the reconcile correlates on.
	got := items[0]
	want := Item{
		ID:               1,
		Path:             "/music/Artist/Album/01 Track.flac",
		MbTrackID:        "t-1",
		MbAlbumID:        "a-1",
		MbReleaseGroupID: "rg-1",
		MbArtistID:       "ar-1",
		AcoustID:         "ac-1",
		ISRC:             "USXX11700001",
		Album:            "Album",
		AlbumArtist:      "Artist",
		Artist:           "Artist",
		Title:            "Track",
		Track:            1,
		Disc:             1,
		Year:             2019,
		Label:            "Label",
		Country:          "US",
		Media:            "CD",
		Format:           "FLAC",
		Bitrate:          971000,
		BitDepth:         16,
		SampleRate:       44100,
		Channels:         2,
		Length:           213.5,
		Added:            1690000000.5,
	}
	if got != want {
		t.Errorf("item =\n%+v\nwant\n%+v", got, want)
	}
}

// An import beets never matched is a real item with nothing on it — not an
// error, and not a row to skip. The reconcile still records the file.
func TestItemsUnmatchedImport(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":2,"path":"/music/x.mp3"}]}`))
	})

	items, err := c.Items(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].MbReleaseGroupID != "" || items[0].Album != "" {
		t.Errorf("expected empty tags, got %+v", items[0])
	}
}

// include_paths defaults to off, and with it off beets sends "path": null for
// every item. Decoding must not fail — the pass detects the condition itself
// and warns, because an error here would look like beets being unreachable.
func TestItemsNullPath(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":1,"path":null,"album":"Album"}]}`))
	})

	items, err := c.Items(context.Background())
	if err != nil {
		t.Fatalf("null path must decode, got %v", err)
	}
	if items[0].Path != "" {
		t.Errorf("path = %q, want empty", items[0].Path)
	}
}

func TestItemsEmpty(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	})

	items, err := c.Items(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestItemsError(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := c.Items(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}

func TestStats(t *testing.T) {
	var gotPath string
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"items":412,"albums":37}`))
	})

	st, err := c.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/stats" {
		t.Errorf("path = %q, want /stats", gotPath)
	}
	if st.Items != 412 || st.Albums != 37 {
		t.Errorf("stats = %+v", st)
	}
}

func TestStatsError(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	if _, err := c.Stats(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}

func TestAddedAt(t *testing.T) {
	// beets stores the import time as a float of epoch seconds, so the
	// fractional part is real sub-second precision and must survive.
	tests := []struct {
		name  string
		added float64
		want  time.Time
	}{
		{"zero is no time", 0, time.Time{}},
		{"negative is no time", -1, time.Time{}},
		{"whole seconds", 1690000000, time.Unix(1690000000, 0).UTC()},
		{"fractional seconds", 1690000000.5, time.UnixMilli(1690000000500).UTC()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := (Item{Added: tc.added}).AddedAt(); !got.Equal(tc.want) {
				t.Errorf("AddedAt() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	if _, err := New(httpx.Config{BaseURL: ""}); err == nil {
		t.Fatal("expected an error for an empty base URL")
	}
}
