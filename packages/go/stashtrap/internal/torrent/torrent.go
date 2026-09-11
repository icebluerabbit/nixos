// Package torrent reads the parts of a .torrent that stashtrap keeps.
//
// The announce name says what a release is called; the torrent says what is
// actually in it. That matters here because a "pack" — an actress compilation,
// a multi-scene rip — is one announce covering many files, each of which maps
// to a different scene upstream. Nothing but the file table can tell them apart.
//
// The raw bytes are never persisted. Parse yields the derived facts and the
// caller drops the input: a corpus of announces is meant to stay cheap to
// query, and a torrent file is hundreds of kilobytes of piece hashes that
// answer no question anyone will ask of it later.
package torrent

import (
	"crypto/sha1" //nolint:gosec // BitTorrent v1 infohashes are SHA-1 by specification.
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/zeebo/bencode"
)

// ErrNoInfo reports a torrent with no info dictionary — not a torrent.
var ErrNoInfo = errors.New("torrent: no info dictionary")

// File is one entry from the torrent's file table.
type File struct {
	// Path is the file's path inside the torrent, slash-joined.
	Path string `json:"path"`
	// Length is its size in bytes.
	Length int64 `json:"length"`
}

// Ext returns the lower-cased extension without the dot, or "" if there is
// none. Phase two sorts video from sample from artwork on it.
func (f File) Ext() string {
	return strings.TrimPrefix(strings.ToLower(path.Ext(f.Path)), ".")
}

// Meta is what stashtrap keeps from a .torrent.
//
// Deliberately absent: announce and announce-list. They hold the passkey, and
// a field that does not exist cannot be logged by accident.
type Meta struct {
	// InfoHash is the v1 infohash, hex, lower case.
	InfoHash string `json:"info_hash"`
	// Name is the torrent's name — the folder for a multi-file torrent, the
	// filename for a single-file one.
	Name string `json:"name"`
	// Files is the file table. A single-file torrent yields one entry named
	// after the torrent, so callers never special-case the two layouts.
	Files []File `json:"files"`
	// TotalSize is the sum of file lengths: exact, unlike the announced size.
	TotalSize int64 `json:"total_size"`
	// Private reports the BEP 27 private flag.
	Private bool `json:"private"`
	// Source is the tracker's source tag, set by trackers that use it to
	// keep otherwise-identical torrents distinct.
	Source string `json:"source"`
	// Comment is the torrent's comment, usually its page URL.
	Comment string `json:"comment"`
	// CreatedBy names the tool that made it.
	CreatedBy string `json:"created_by"`
}

// metainfo is the top level. Info stays raw: the infohash is SHA-1 over the
// original bytes of the info dictionary, and re-encoding a decoded dictionary
// would change them whenever the source was not in canonical form.
type metainfo struct {
	Info      bencode.RawMessage `bencode:"info"`
	Comment   string             `bencode:"comment"`
	CreatedBy string             `bencode:"created by"`
}

// info is the subset of the info dictionary worth keeping. `pieces` is
// deliberately not decoded — it is the bulk of the file and says nothing about
// content.
type info struct {
	Name    string `bencode:"name"`
	Length  int64  `bencode:"length"`
	Private int    `bencode:"private"`
	Source  string `bencode:"source"`
	Files   []struct {
		Path   []string `bencode:"path"`
		Length int64    `bencode:"length"`
	} `bencode:"files"`
}

// Parse decodes a .torrent.
func Parse(raw []byte) (*Meta, error) {
	var mi metainfo
	if err := bencode.DecodeBytes(raw, &mi); err != nil {
		return nil, fmt.Errorf("torrent: decode: %w", err)
	}
	if len(mi.Info) == 0 {
		return nil, ErrNoInfo
	}

	var in info
	if err := bencode.DecodeBytes(mi.Info, &in); err != nil {
		return nil, fmt.Errorf("torrent: decode info: %w", err)
	}

	sum := sha1.Sum(mi.Info) //nolint:gosec // see import
	m := &Meta{
		InfoHash:  hex.EncodeToString(sum[:]),
		Name:      in.Name,
		Private:   in.Private == 1,
		Source:    in.Source,
		Comment:   mi.Comment,
		CreatedBy: mi.CreatedBy,
	}

	// Single-file torrents carry `length` and no `files`. Normalising them
	// into a one-entry table means callers never branch on the layout.
	if len(in.Files) == 0 {
		m.Files = []File{{Path: in.Name, Length: in.Length}}
		m.TotalSize = in.Length
		return m, nil
	}

	m.Files = make([]File, 0, len(in.Files))
	for _, f := range in.Files {
		m.Files = append(m.Files, File{
			Path:   path.Join(f.Path...),
			Length: f.Length,
		})
		m.TotalSize += f.Length
	}
	return m, nil
}
