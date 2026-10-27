package ingest

import (
	"slices"
	"testing"
)

// TestArrWebhookFilesFlattensEveryCollection is the reason one struct covers
// three apps.
//
// Sonarr sends episodeFile (singular) on Download and episodeFiles (plural) on
// ImportComplete; Radarr sends movieFile; Lidarr sends trackFiles. Missing one
// name means the provenance for that app's imports is never written, and
// nothing fails — the table is just quietly short.
func TestArrWebhookFilesFlattensEveryCollection(t *testing.T) {
	h := arrWebhook{
		EpisodeFile:  &arrFile{Path: "/tv/a.mkv"},
		EpisodeFiles: []arrFile{{Path: "/tv/b.mkv"}, {Path: "/tv/c.mkv"}},
		MovieFile:    &arrFile{Path: "/films/d.mkv"},
		TrackFiles:   []arrFile{{Path: "/music/e.flac"}},
	}
	var paths []string
	for _, f := range h.files() {
		paths = append(paths, f.Path)
	}
	slices.Sort(paths)
	want := []string{"/films/d.mkv", "/music/e.flac", "/tv/a.mkv", "/tv/b.mkv", "/tv/c.mkv"}
	if !slices.Equal(paths, want) {
		t.Errorf("got %q\nwant %q", paths, want)
	}
}

// TestArrWebhookFilesEmpty covers a grab or health event, which carry none.
func TestArrWebhookFilesEmpty(t *testing.T) {
	var h arrWebhook
	if got := h.files(); len(got) != 0 {
		t.Errorf("got %d files, want 0", len(got))
	}
}

// TestArrWebhookDecodesSonarrDownload pins the JSON tags against a real body.
func TestArrWebhookDecodesSonarrDownload(t *testing.T) {
	const body = `{
	  "eventType": "Download",
	  "downloadId": "0123456789ABCDEF0123456789ABCDEF01234567",
	  "series": {"id": 12, "title": "Some Show"},
	  "episodeFile": {
	    "id": 900,
	    "relativePath": "Season 01/S01E01.mkv",
	    "path": "/media/tv/Some Show/Season 01/S01E01.mkv",
	    "size": 123456789,
	    "sceneName": "Some.Show.S01E01.1080p"
	  },
	  "release": {
	    "releaseTitle": "Some.Show.S01E01.1080p-GRP",
	    "indexer": "TorrentLeech",
	    "size": 123456789
	  },
	  "instanceName": "Sonarr"
	}`
	var h arrWebhook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if h.EventType != "Download" {
		t.Errorf("event = %q", h.EventType)
	}
	if h.Release.Indexer != "TorrentLeech" {
		t.Errorf("indexer = %q", h.Release.Indexer)
	}
	files := h.files()
	if len(files) != 1 || files[0].Path != "/media/tv/Some Show/Season 01/S01E01.mkv" {
		t.Fatalf("files = %+v", files)
	}
}

// TestArrEventNamesAreLowercase is what the router compares against.
//
// The family does not agree on case — Sonarr sends "Grab", Lidarr has sent
// "grab" — so the constants must be the lowercased form and the router must
// fold. A capitalised constant here would compile, run, and route nothing.
func TestArrEventNamesAreLowercase(t *testing.T) {
	for _, c := range []string{
		arrEventGrab, arrEventDownload, arrEventImportComplete,
		arrEventHealthIssue, arrEventHealthRestored,
	} {
		for _, r := range c {
			if r >= 'A' && r <= 'Z' {
				t.Errorf("event constant %q is not lowercase", c)
				break
			}
		}
	}
}

// TestArrWebhookDecodesHealth covers the other half of the router.
func TestArrWebhookDecodesHealth(t *testing.T) {
	const body = `{
	  "eventType": "Health",
	  "level": "warning",
	  "message": "Indexer unavailable due to failures: TorrentLeech",
	  "type": "IndexerStatusCheck",
	  "wikiUrl": "https://wiki.servarr.com/"
	}`
	var h arrWebhook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if h.Type != "IndexerStatusCheck" || h.Level != "warning" {
		t.Errorf("type=%q level=%q", h.Type, h.Level)
	}
	if h.Message == "" {
		t.Error("message did not decode")
	}
}
