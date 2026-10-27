package reconcile

import (
	"testing"

	"homelab/clients/arr"
	"homelab/httpx"
	"homelab/managarr/internal/model"
)

// TestMediaInfoRowKeepsTextBitrates is why those columns are text.
//
// Sonarr types bitrate as an integer; Lidarr's is a different class whose
// audioBitRate is preformatted prose like "1411 kbps". Storing the string keeps
// both without inventing a unit neither stated — and Flex is what stops the
// decode failing outright, which once took the whole Lidarr pass down.
func TestMediaInfoRowKeepsTextBitrates(t *testing.T) {
	got := mediaInfoRow(model.NewID(), arr.MediaInfo{
		VideoBitrate:    arr.Flex("8000000"),
		AudioBitrate:    arr.Flex("1411 kbps"),
		AudioSampleRate: arr.Flex("44.1 kHz"),
		AudioBits:       arr.Flex("16"),
	})
	if deref(got.VideoBitrate) != "8000000" {
		t.Errorf("video bitrate = %q", deref(got.VideoBitrate))
	}
	if deref(got.AudioBitrate) != "1411 kbps" {
		t.Errorf("audio bitrate = %q", deref(got.AudioBitrate))
	}
	if deref(got.AudioSampleRate) != "44.1 kHz" {
		t.Errorf("sample rate = %q", deref(got.AudioSampleRate))
	}
}

// TestMediaInfoRowUnprobedFileIsAllNull covers the normal early state.
//
// A file the *arr has not analysed yet reports an empty MediaInfo. Every column
// must come out NULL rather than zero, or an unprobed 4K remux reads as a
// 0x0 file with no codec.
func TestMediaInfoRowUnprobedFileIsAllNull(t *testing.T) {
	id := model.NewID()
	got := mediaInfoRow(id, arr.MediaInfo{})

	if got.LibraryFileID != id {
		t.Error("library file id not carried through")
	}
	nulls := map[string]any{
		"VideoCodec":       got.VideoCodec,
		"VideoBitrate":     got.VideoBitrate,
		"VideoBitDepth":    got.VideoBitDepth,
		"VideoFps":         got.VideoFps,
		"Resolution":       got.Resolution,
		"AudioCodec":       got.AudioCodec,
		"AudioChannels":    got.AudioChannels,
		"AudioStreamCount": got.AudioStreamCount,
		"Subtitles":        got.Subtitles,
	}
	for name, v := range nulls {
		switch p := v.(type) {
		case *string:
			if p != nil {
				t.Errorf("%s = %q, want NULL", name, *p)
			}
		case *int:
			if p != nil {
				t.Errorf("%s = %d, want NULL", name, *p)
			}
		case *float64:
			if p != nil {
				t.Errorf("%s = %v, want NULL", name, *p)
			}
		}
	}
}

// TestMediaInfoRowCarriesTheVideoFields checks the non-empty path end to end.
func TestMediaInfoRowCarriesTheVideoFields(t *testing.T) {
	got := mediaInfoRow(model.NewID(), arr.MediaInfo{
		VideoCodec:            "x265",
		VideoBitDepth:         10,
		VideoFps:              23.976,
		VideoDynamicRangeType: "HDR10",
		Resolution:            "3840x2160",
		ScanType:              "Progressive",
		AudioCodec:            "TrueHD Atmos",
		AudioChannels:         7.1,
		AudioLanguages:        "eng",
		AudioStreamCount:      2,
		Subtitles:             "eng/spa",
	})
	if deref(got.VideoCodec) != "x265" {
		t.Errorf("codec = %q", deref(got.VideoCodec))
	}
	if got.VideoBitDepth == nil || *got.VideoBitDepth != 10 {
		t.Errorf("bit depth = %v", got.VideoBitDepth)
	}
	if got.VideoFps == nil || *got.VideoFps != 23.976 {
		t.Errorf("fps = %v", got.VideoFps)
	}
	if got.AudioChannels == nil || *got.AudioChannels != 7.1 {
		t.Errorf("channels = %v", got.AudioChannels)
	}
	if got.AudioStreamCount == nil || *got.AudioStreamCount != 2 {
		t.Errorf("stream count = %v", got.AudioStreamCount)
	}
}

// TestContentForFileRadarrUsesTheMovie covers the direction that differs.
//
// Radarr's file names its movie and there is nothing below a movie. Sonarr and
// Lidarr are the other way round: the file does not name the episode or track,
// the episode and track name the file.
func TestContentForFileRadarrUsesTheMovie(t *testing.T) {
	c, err := arr.New(arr.Radarr, httpConfig(), "key")
	if err != nil {
		t.Fatal(err)
	}
	a := &Arr{client: c}

	ix := newArrIndex()
	movie := model.NewID()
	ix.parent[42] = movie
	// A Radarr index never has file entries; if the resolver consulted it, a
	// collision on the numeric id would silently misattribute.
	ix.file[7] = model.NewID()

	got := a.contentForFile(ix, arr.MediaFile{ID: 7, MovieID: 42})
	if got == nil || *got != movie {
		t.Fatalf("got %v, want the movie %s", got, movie)
	}

	if got := a.contentForFile(ix, arr.MediaFile{ID: 7, MovieID: 999}); got != nil {
		t.Errorf("an untracked movie gave %v, want nil", got)
	}
}

// TestContentForFileSonarrUsesTheFileID is the leaf-resolution half.
func TestContentForFileSonarrUsesTheFileID(t *testing.T) {
	c, err := arr.New(arr.Sonarr, httpConfig(), "key")
	if err != nil {
		t.Fatal(err)
	}
	a := &Arr{client: c}

	ix := newArrIndex()
	episode := model.NewID()
	ix.file[7] = episode
	ix.parent[7] = model.NewID() // same number, different collection

	got := a.contentForFile(ix, arr.MediaFile{ID: 7, SeriesID: 7})
	if got == nil || *got != episode {
		t.Fatalf("got %v, want the episode %s", got, episode)
	}

	// A file no episode claims is an orphan the *arr has not reindexed yet.
	if got := a.contentForFile(ix, arr.MediaFile{ID: 8}); got != nil {
		t.Errorf("orphan file gave %v, want nil", got)
	}
}

// httpConfig is a minimal client config for tests that only exercise logic
// which never issues a request.
func httpConfig() httpx.Config {
	return httpx.Config{BaseURL: "http://127.0.0.1:1"}
}
