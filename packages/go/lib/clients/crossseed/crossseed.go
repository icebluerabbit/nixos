// Package crossseed holds cross-seed's wire types.
//
// Types only, no client. managarr-crossseed reads these out of cross-seed's
// SQLite database and serves them as sidecar event payloads; managarr decodes
// the payloads back into them. Both halves need the same struct, and the only
// way to be sure they have it is for there to be one.
//
// cross-seed's own HTTP surface is /api/ping, /api/announce and /api/webhook,
// and none of it exposes decisions — the webhook fires only when a candidate is
// INJECTED, so cross-seed reports the matches and none of the rejections, which
// are four fifths of the record and the only answer to "why was this not
// cross-seeded". That is why there is a sidecar at all.
//
// https://github.com/cross-seed/cross-seed
package crossseed

import "time"

// Source is what a cross-seed event carries in sidecar.Event.Source.
const Source = "cross-seed"

// Event kinds this service emits.
const (
	// KindDecision is one verdict on one candidate.
	KindDecision = "decision"
)

// Decision is cross-seed's verdict on one candidate for one searchee.
//
// Values observed: MATCH, INFO_HASH_ALREADY_EXISTS, SIZE_MISMATCH,
// FILE_TREE_MISMATCH, SAME_INFO_HASH. Only the first is ever webhooked.
type Decision struct {
	// SearcheeName is the torrent the search ran FROM. cross-seed keys a
	// searchee by NAME, not by hash, and one name routinely covers several
	// infohashes — they are cross-seeds of each other — so this does not
	// uniquely identify a download.
	SearcheeName string `json:"searcheeName"`
	// InfoHash is the CANDIDATE's hash: what cross-seed proposed. Not the
	// searchee's.
	InfoHash string `json:"infoHash"`
	// GUID is the indexer's link for the candidate and carries that
	// indexer's RSS key. Redacted before storage.
	GUID      string    `json:"guid"`
	Decision  string    `json:"decision"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
}

// ClientTorrent is one torrent cross-seed knows the client holds.
//
// The bridge from a searchee to a real download. cross-seed keys a searchee by
// NAME and one name routinely covers several infohashes — they are cross-seeds
// of each other — so the name alone cannot say which torrent a decision was
// made about.
//
// Files is what resolves that. A searchee's file list identifies exactly one
// torrent even when several share a name, which makes the whole cross-seed
// graph expressible as infohash to infohash with no title matching anywhere in
// it.
//
// Not events — a lookup table — so the sidecar serves it on its own route
// rather than through /changes.
type ClientTorrent struct {
	InfoHash string `json:"infoHash"`
	Name     string `json:"name"`
	// Title is cross-seed's parsed title, distinct from the release name.
	Title string `json:"title,omitempty"`
	// Files is the torrent's content as cross-seed recorded it. The
	// identifying field: two torrents of the same release name hold the same
	// files, and two unrelated torrents do not.
	Files []ClientFile `json:"files,omitempty"`
	// Length is the total size, a cheap discriminator before comparing file
	// lists.
	Length   int64  `json:"length,omitempty"`
	SavePath string `json:"savePath,omitempty"`
	Category string `json:"category,omitempty"`
}

// ClientFile is one entry of a searchee's file list.
type ClientFile struct {
	Name   string `json:"name"`
	Length int64  `json:"length"`
	Path   string `json:"path,omitempty"`
}

// ClientTorrentsPath is the sidecar route serving that table.
const ClientTorrentsPath = "/client-torrents"

// EnsembleEntry maps an infohash to a path cross-seed linked for it.
//
// cross-seed's own record of which torrent produced which directory, by
// identifier rather than by name. The other half of making the graph
// hash-to-hash: an injected cross-seed gets its own linkDir, so it shares no
// path with its source, and this is where cross-seed writes down that they are
// the same content.
type EnsembleEntry struct {
	InfoHash string `json:"infoHash"`
	Path     string `json:"path"`
	Ensemble string `json:"ensemble"`
	Element  string `json:"element"`
}

// EnsemblePath is the sidecar route serving that table.
const EnsemblePath = "/ensemble"

// Stats is the sidecar's count of what the database holds.
type Stats struct {
	Decisions      int `json:"decisions"`
	Searchees      int `json:"searchees"`
	ClientTorrents int `json:"clientTorrents"`
}

// StatsPath is the sidecar route serving those counts.
const StatsPath = "/stats"
