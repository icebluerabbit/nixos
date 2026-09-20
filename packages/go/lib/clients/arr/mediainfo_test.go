package arr_test

import (
	"encoding/json"
	"testing"

	"homelab/clients/arr"
)

// The three apps do not share a MediaInfo type.
//
// Sonarr's MediaInfoResource types bitrates as int64. Lidarr's is a different
// class entirely — five audio fields — whose AudioBitRate is a preformatted
// string like "1411 kbps". Go matches JSON keys case-insensitively, so Lidarr's
// audioBitRate binds to an audioBitrate field; typed as int64 that fails the
// decode and takes the whole reconcile pass with it.
func TestMediaInfoAcceptsBothShapes(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body         string
		wantAudio    string
		wantVideo    string
		wantCodec    string
		wantBits     string
		wantChannels float64
	}{
		"sonarr numeric": {
			body: `{"videoCodec":"x265","videoBitrate":8000000,
			        "audioCodec":"EAC3","audioBitrate":768000,
			        "audioChannels":5.1,"resolution":"1920x1080"}`,
			wantAudio: "768000", wantVideo: "8000000",
			wantCodec: "EAC3", wantChannels: 5.1,
		},
		"lidarr formatted string": {
			body: `{"audioChannels":2.0,"audioBitRate":"1411 kbps",
			        "audioCodec":"FLAC","audioBits":"16",
			        "audioSampleRate":"44.1 kHz"}`,
			wantAudio: "1411 kbps", wantVideo: "",
			wantCodec: "FLAC", wantBits: "16", wantChannels: 2.0,
		},
		"nulls": {
			body:      `{"audioBitrate":null,"videoBitrate":null}`,
			wantAudio: "", wantVideo: "",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var mi arr.MediaInfo
			if err := json.Unmarshal([]byte(tc.body), &mi); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got := mi.AudioBitrate.String(); got != tc.wantAudio {
				t.Errorf("audio bitrate = %q, want %q", got, tc.wantAudio)
			}
			if got := mi.VideoBitrate.String(); got != tc.wantVideo {
				t.Errorf("video bitrate = %q, want %q", got, tc.wantVideo)
			}
			if tc.wantCodec != "" && mi.AudioCodec != tc.wantCodec {
				t.Errorf("audio codec = %q, want %q", mi.AudioCodec, tc.wantCodec)
			}
			if tc.wantBits != "" && mi.AudioBits.String() != tc.wantBits {
				t.Errorf("audio bits = %q, want %q", mi.AudioBits, tc.wantBits)
			}
			if tc.wantChannels != 0 && mi.AudioChannels != tc.wantChannels {
				t.Errorf("audio channels = %v, want %v", mi.AudioChannels, tc.wantChannels)
			}
		})
	}
}

// A whole file record has to decode for every app, since one failure aborts the
// pass rather than skipping the file.
func TestMediaFileDecodesPerApp(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"sonarr": `{"id":1,"path":"/x.mkv","size":100,"seriesId":5,"seasonNumber":2,
		            "quality":{"quality":{"id":9,"name":"WEBDL-1080p","source":"web","resolution":1080},
		                       "revision":{"version":2,"real":0,"isRepack":true}},
		            "customFormatScore":100,"qualityCutoffNotMet":false,
		            "mediaInfo":{"videoBitrate":8000000,"audioBitrate":768000}}`,
		"lidarr": `{"id":2,"path":"/y.flac","size":50,"artistId":7,"albumId":3,
		            "quality":{"quality":{"id":6,"name":"FLAC"},"revision":{"version":1}},
		            "customFormatScore":0,
		            "mediaInfo":{"audioChannels":2.0,"audioBitRate":"1411 kbps","audioCodec":"FLAC"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var f arr.MediaFile
			if err := json.Unmarshal([]byte(body), &f); err != nil {
				t.Fatalf("decode %s file: %v", name, err)
			}
			if f.Quality.Quality.Name == "" {
				t.Errorf("%s: quality name empty", name)
			}
			if f.MediaInfo == nil {
				t.Errorf("%s: media info missing", name)
			}
		})
	}
}
