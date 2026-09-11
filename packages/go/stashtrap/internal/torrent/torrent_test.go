package torrent_test

import (
	"crypto/sha1" //nolint:gosec // matching the infohash the code under test computes
	"encoding/hex"
	"testing"

	"github.com/zeebo/bencode"

	"homelab/stashtrap/internal/torrent"
)

// build encodes a metainfo dictionary the way a tracker would, so the tests
// exercise real bencode rather than a fixture nobody can read.
func build(t *testing.T, info, extra map[string]any) (raw []byte, infoHash string) {
	t.Helper()

	infoRaw, err := bencode.EncodeBytes(info)
	if err != nil {
		t.Fatalf("encode info: %v", err)
	}
	top := map[string]any{"info": bencode.RawMessage(infoRaw)}
	for k, v := range extra {
		top[k] = v
	}
	raw, err = bencode.EncodeBytes(top)
	if err != nil {
		t.Fatalf("encode metainfo: %v", err)
	}
	sum := sha1.Sum(infoRaw) //nolint:gosec // see import
	return raw, hex.EncodeToString(sum[:])
}

func TestParseSingleFile(t *testing.T) {
	t.Parallel()

	raw, wantHash := build(t, map[string]any{
		"name":         "Scene.2026.1080p.mp4",
		"length":       int64(1234567),
		"piece length": int64(262144),
		"private":      int64(1),
		"source":       "BitPorn",
	}, map[string]any{
		"comment":    "https://bitporn.invalid/torrents/1",
		"created by": "mktorrent 1.1",
	})

	m, err := torrent.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.InfoHash != wantHash {
		t.Fatalf("InfoHash = %s, want %s", m.InfoHash, wantHash)
	}
	// A single-file torrent must still present a file table, so callers
	// never branch on the layout.
	if len(m.Files) != 1 || m.Files[0].Path != "Scene.2026.1080p.mp4" || m.Files[0].Length != 1234567 {
		t.Fatalf("Files = %+v", m.Files)
	}
	if m.TotalSize != 1234567 {
		t.Fatalf("TotalSize = %d", m.TotalSize)
	}
	if !m.Private || m.Source != "BitPorn" {
		t.Fatalf("private/source wrong: %+v", m)
	}
	if m.Comment == "" || m.CreatedBy == "" {
		t.Fatalf("comment/created by lost: %+v", m)
	}
	if got := m.Files[0].Ext(); got != "mp4" {
		t.Fatalf("Ext() = %q, want mp4", got)
	}
}

// A pack is the case that justifies parsing the torrent at all: one announce,
// many files, each mapping to a different scene upstream.
func TestParseMultiFile(t *testing.T) {
	t.Parallel()

	raw, _ := build(t, map[string]any{
		"name":         "Actress Pack",
		"piece length": int64(262144),
		"files": []any{
			map[string]any{"path": []any{"scene1.mp4"}, "length": int64(100)},
			map[string]any{"path": []any{"extras", "scene2.mp4"}, "length": int64(250)},
		},
	}, nil)

	m, err := torrent.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Files) != 2 {
		t.Fatalf("Files = %+v", m.Files)
	}
	if m.Files[1].Path != "extras/scene2.mp4" {
		t.Fatalf("nested path = %q", m.Files[1].Path)
	}
	if m.TotalSize != 350 {
		t.Fatalf("TotalSize = %d, want 350", m.TotalSize)
	}
	if m.Private {
		t.Fatal("Private set without a private flag")
	}
}

// The infohash is SHA-1 over the ORIGINAL info bytes. A torrent whose info
// dictionary is not in canonical order still has to hash to what the tracker
// and qBittorrent will agree on, which is why the raw slice is kept rather
// than re-encoded.
func TestInfoHashUsesOriginalBytes(t *testing.T) {
	t.Parallel()

	// "z" sorts after "name", so a canonical encoder would place it last;
	// writing it first proves nothing re-encodes on the way through.
	infoRaw := []byte("d1:z1:x4:name3:abc6:lengthi7ee")
	top := []byte("d4:info" + string(infoRaw) + "e")

	m, err := torrent.Parse(top)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	sum := sha1.Sum(infoRaw) //nolint:gosec // see import
	if want := hex.EncodeToString(sum[:]); m.InfoHash != want {
		t.Fatalf("InfoHash = %s, want %s", m.InfoHash, want)
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	cases := map[string][]byte{
		"empty":       {},
		"not bencode": []byte("hello"),
		"no info":     []byte("d7:comment3:abce"),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := torrent.Parse(raw); err == nil {
				t.Fatal("Parse succeeded on malformed input")
			}
		})
	}
}
