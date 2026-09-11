// Package types is the wire contract between autobrr and stashtrap.
//
// autobrr renders the body from a Go template in the filter's external webhook
// (modules/hosts/server/containers/autobrr/filters/), so the field names here
// are fixed by that template rather than by any autobrr API. Both halves change
// together.
package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
)

// Announce is one release offered by autobrr.
//
// Every field is optional on the wire: an indexer that does not report
// freeleech simply omits it, and the template can gain a field without
// breaking a running service. Absence is recorded as the zero value rather
// than rejected — the point of phase one is to learn what actually arrives.
type Announce struct {
	// Name is the release title, e.g. "[Site.com] Performer - Title".
	Name string `json:"name"`
	// Indexer is autobrr's identifier for the tracker.
	Indexer string `json:"indexer"`
	// InfoHash is the v1 infohash as autobrr reports it. It is the join key
	// to qBittorrent later, and is compared against the hash computed from
	// TorrentData whenever both are present.
	InfoHash string `json:"info_hash"`
	// TorrentID is the tracker's own id for the torrent.
	TorrentID string `json:"torrent_id"`
	// Category is the tracker's category string.
	Category string `json:"category"`
	// Tags are the tracker's tags.
	Tags []string `json:"tags"`
	// Categories is autobrr's Categories field, sent verbatim: each entry is
	// a [name, id] pair flattened in order, so ["XXX","6000","Fansite",
	// "100020"]. Only a feed release has it — an IRC one fills Category
	// instead. ParseCategories turns it back into pairs.
	Categories []string `json:"categories"`
	// ReleaseTags is autobrr's parse of quality/format tokens.
	ReleaseTags string `json:"release_tags"`
	// Size is the announced size in bytes. Trackers round it; the torrent's
	// own file table is authoritative when present.
	Size uint64 `json:"size"`
	// Freeleech reports a 100% freeleech release.
	//
	// Unreliable on some trackers: BitPorn's Prowlarr definition derives it
	// from a per-torrent field, so a site-wide freeleech event does not set
	// it. Recorded, never trusted as the only signal.
	Freeleech bool `json:"freeleech"`
	// FreeleechPercent is the discount, 0-100.
	FreeleechPercent int `json:"freeleech_percent"`
	// Uploader is the release's uploader where the tracker announces one.
	Uploader string `json:"uploader"`
	// InfoURL is the human-facing torrent page.
	InfoURL string `json:"info_url"`
	// DownloadURL fetches the .torrent. It carries the passkey, so it is
	// never logged and never stored.
	DownloadURL string `json:"download_url"`
	// TorrentData is the .torrent itself, base64 of the raw bencode.
	//
	// The template uses sprig's b64enc, not js: bencode is binary, JSON
	// strings must be valid UTF-8, and a js-escaped blob loses bytes to
	// U+FFFD on decode — silently, and only for some torrents.
	TorrentData string `json:"torrent_data"`
}

// Torrent decodes TorrentData. It returns nil, nil when the field is absent,
// which is normal for a template that does not send it.
func (a Announce) Torrent() ([]byte, error) {
	if a.TorrentData == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(a.TorrentData)
	if err != nil {
		return nil, fmt.Errorf("decode torrent_data: %w", err)
	}
	return raw, nil
}

// LogValue keeps DownloadURL and TorrentData out of every log record: the
// first carries a passkey, the second is a megabyte of binary. slog calls this
// for the whole struct, so a field added above is invisible until it is added
// here too — silence is the safe default.
func (a Announce) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", a.Name),
		slog.String("indexer", a.Indexer),
		slog.String("info_hash", a.InfoHash),
		slog.Uint64("size", a.Size),
		slog.Bool("freeleech", a.Freeleech),
		slog.Int("freeleech_percent", a.FreeleechPercent),
		slog.String("category", a.Category),
		slog.Any("tags", a.Tags),
	)
}

// UnmarshalJSON tolerates a tags field sent as a delimited string.
//
// autobrr's .Tags is a []string, but a template interpolating it directly
// yields "a, b", and a filter edited in the UI can produce either. Accepting
// both keeps a malformed template from dropping every announce.
func (a *Announce) UnmarshalJSON(data []byte) error {
	type alias Announce
	aux := struct {
		Tags json.RawMessage `json:"tags"`
		*alias
	}{alias: (*alias)(a)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	a.Tags = parseTags(aux.Tags)
	return nil
}
