package model

import (
	"time"

	"gorm.io/gorm"
)

// Offer kinds: how a release came to be on the table.
const (
	// OfferAnnounce is an IRC or RSS announce, seen by autobrr.
	OfferAnnounce = "announce"
	// OfferCrossSeedCandidate is a torrent cross-seed proposed as matching
	// something already held.
	OfferCrossSeedCandidate = "cross_seed_candidate"
)

// Offer is a release made available through an indexer at a point in time.
//
// An announce and a cross-seed candidate are the same fact — here is a release,
// from this indexer, now — differing only in how it surfaced. Modelling them as
// separate tables meant a rejection had two different homes and nothing could
// answer "everything that was ever offered, and what I did about it".
//
// A search RESULT is deliberately not a third kind. Nothing in this stack
// retains one: Prowlarr keeps the query and a result count, an *arr keeps only
// what it grabbed, and the *arr's release endpoint runs a fresh search against
// the trackers rather than replaying a past one. The candidate that was taken
// is already recorded, as a grab.
//
// The kind-specific columns are null for the kinds that do not carry them: an
// infohash comes from an announce or a cross-seed match, a source infohash only
// from cross-seed. Splitting into subtype tables to avoid those nulls would
// rebuild the fragmentation this replaces.
type Offer struct {
	Base
	Kind         string    `gorm:"not null;index;uniqueIndex:idx_offer_source,priority:2"`
	IndexerName  string    `gorm:"not null;uniqueIndex:idx_offer_source,priority:1"`
	SourceRef    string    `gorm:"not null;uniqueIndex:idx_offer_source,priority:3"`
	DiscoveredAt time.Time `gorm:"not null;index:,sort:desc"`

	ReleaseID *ID `gorm:"type:uuid;index"`

	// Infohash of the offered torrent, where the source states it.
	//
	// Deliberately NOT a foreign key to torrents: a release can be announced
	// or proposed and never obtained, so the hash is real while the torrent is
	// not one this stack holds. Constraining it would mean discarding the hash
	// — and for an autobrr announce it is the only place the hash ever appears,
	// because autobrr's API does not serialise it.
	Infohash *string `gorm:"index:,where:infohash IS NOT NULL"`

	// TrackerTorrentID is the TRACKER's own id for the offered torrent.
	//
	// The bridge between autobrr's API and its webhook. autobrr keys a release
	// by an id its templates cannot emit, and emits an infohash its API never
	// serialises, so neither side alone can identify an announce to the other.
	// This is the single field present in both, which makes
	// (indexer_name, tracker_torrent_id) the key the webhook updates through.
	//
	// Unique per indexer, and only where the indexer's definition parses one —
	// hence the partial index rather than a constraint.
	TrackerTorrentID *string `gorm:"index:,where:tracker_torrent_id IS NOT NULL"`

	// Seeders and leechers as the source reported them AT THE MOMENT of the
	// offer. A point-in-time count, not a current one: it is what the decision
	// to take or refuse the release was made against.
	Seeders     *int
	Leechers    *int
	SizeBytes   *int64
	PublishedAt *time.Time

	// SourceInfohash is the torrent cross-seed searched FROM. Set for
	// OfferCrossSeedCandidate: the candidate is what it found, this is what it
	// was matching against.
	SourceInfohash *string `gorm:"index"`

	// The IRC channel an announce was seen on is deliberately absent.
	//
	// autobrr does not record it: its release table has no channel or network
	// column, it keeps no announce log, and it exposes no channel macro. That
	// is not an oversight upstream — an IRC indexer maps to exactly one network
	// and announce channel by definition, which is what autobrr keys its
	// announce handler by, so the channel is already determined by IndexerName.
	// Storing it here would repeat nine distinct strings across every announce
	// ever seen. irc_networks and irc_channels hold the topology.

	// AnnounceType and the parse-time fields autobrr reports.
	AnnounceType *string
	Protocol     *string
	Categories   *string
	Uploader     *string
	PreTime      *string
	InfoURL      *string `gorm:"column:info_url"`
	DownloadURL  *string `gorm:"column:download_url"`
	// NormalizedHash is autobrr's dedupe digest over the parsed name. Not an
	// infohash and must never be read as one.
	NormalizedHash *string

	Release       *Release `gorm:"foreignKey:ReleaseID"`
	Indexer       *Indexer `gorm:"foreignKey:IndexerName;references:Name"`
	SourceTorrent *Torrent `gorm:"foreignKey:SourceInfohash;references:Infohash"`
}

func (Offer) TableName() string { return "offers" }

// BeforeSave strips the tracker keys carried by an offer's links.
func (o *Offer) BeforeSave(*gorm.DB) error {
	redactURL(o.InfoURL)
	redactURL(o.DownloadURL)
	return nil
}
