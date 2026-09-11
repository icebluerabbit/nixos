package types_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"homelab/stashtrap/internal/types"
)

func TestUnmarshalTags(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body string
		want []string
	}{
		// autobrr's own .Tags is a []string.
		"array":            {`{"tags":["anal","teen"]}`, []string{"anal", "teen"}},
		"comma string":     {`{"tags":"anal, teen"}`, []string{"anal", "teen"}},
		"pipe string":      {`{"tags":"anal|teen"}`, []string{"anal", "teen"}},
		"trailing comma":   {`{"tags":"anal,"}`, []string{"anal"}},
		"absent":           {`{}`, nil},
		"empty string":     {`{"tags":""}`, nil},
		"empty array":      {`{"tags":[]}`, nil},
		"unexpected shape": {`{"tags":42}`, nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var a types.Announce
			if err := json.Unmarshal([]byte(tc.body), &a); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(a.Tags) != len(tc.want) {
				t.Fatalf("tags = %q, want %q", a.Tags, tc.want)
			}
			for i := range tc.want {
				if a.Tags[i] != tc.want[i] {
					t.Fatalf("tags = %q, want %q", a.Tags, tc.want)
				}
			}
		})
	}
}

func TestUnmarshalRestOfPayload(t *testing.T) {
	t.Parallel()

	const body = `{
		"name":"[Site.com] Performer - Title",
		"indexer":"bitporn",
		"info_hash":"abc123",
		"torrent_id":"139346",
		"category":"XXX",
		"release_tags":"1080p",
		"size":19354756074,
		"freeleech":true,
		"freeleech_percent":100,
		"uploader":"someone",
		"info_url":"https://example.invalid/torrents/1",
		"download_url":"https://example.invalid/download/1.torrent"
	}`

	var a types.Announce
	if err := json.Unmarshal([]byte(body), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.Name != "[Site.com] Performer - Title" || a.Indexer != "bitporn" {
		t.Fatalf("name/indexer wrong: %+v", a)
	}
	if a.Size != 19354756074 || !a.Freeleech || a.FreeleechPercent != 100 {
		t.Fatalf("numeric fields wrong: %+v", a)
	}
	if a.TorrentID != "139346" || a.InfoHash != "abc123" {
		t.Fatalf("identifiers wrong: %+v", a)
	}
}

func TestTorrent(t *testing.T) {
	t.Parallel()

	t.Run("absent", func(t *testing.T) {
		t.Parallel()
		raw, err := types.Announce{}.Torrent()
		if err != nil || raw != nil {
			t.Fatalf("Torrent() = %v, %v; want nil, nil", raw, err)
		}
	})

	t.Run("decoded", func(t *testing.T) {
		t.Parallel()
		want := []byte("d4:infod4:name3:abcee")
		a := types.Announce{TorrentData: base64.StdEncoding.EncodeToString(want)}
		got, err := a.Torrent()
		if err != nil {
			t.Fatalf("Torrent(): %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("Torrent() = %q, want %q", got, want)
		}
	})

	t.Run("not base64", func(t *testing.T) {
		t.Parallel()
		if _, err := (types.Announce{TorrentData: "!!!"}).Torrent(); err == nil {
			t.Fatal("Torrent() succeeded on invalid base64")
		}
	})
}

// The download URL carries the tracker passkey and the torrent payload is a
// megabyte of binary: neither may reach a log line, and this is the test that
// says so.
func TestLogValueOmitsSecrets(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	log.Info("announce", "announce", types.Announce{
		Name:        "release",
		DownloadURL: "https://tracker.invalid/download?passkey=SUPERSECRET",
		TorrentData: "QUJD",
	})

	out := buf.String()
	for _, forbidden := range []string{"SUPERSECRET", "passkey", "QUJD"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("log record leaked %q: %s", forbidden, out)
		}
	}
	if !strings.Contains(out, "release") {
		t.Fatalf("log record lost the release name: %s", out)
	}
}
