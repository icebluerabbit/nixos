package model

// Indexer is a search source: what a query goes through.
//
// Distinct from Tracker, which is what a torrent announces to. They frequently
// correspond and are still not the same thing — Prowlarr and autobrr name an
// indexer, qBittorrent only ever sees an announce host.
//
// Keyed on name because that is the one identifier all three writers share.
// Each service owns different columns, so none of them fight: autobrr owns
// identifier, implementation and enabled; Prowlarr owns protocol, privacy and
// priority. Writing only what you own is what keeps the last pass to run from
// deciding the value.
type Indexer struct {
	Name string `gorm:"primaryKey"`

	Protocol       *string
	Privacy        *string
	Priority       *int
	Identifier     *string
	Implementation *string
	Enabled        *bool
	// AnnounceHost links this indexer to the tracker it fronts, where that is
	// known. Null for a torznab indexer whose announce URL is never seen.
	TrackerAnnounceHost *string `gorm:"index"`

	Tracker *Tracker `gorm:"foreignKey:TrackerAnnounceHost;references:AnnounceHost"`
}

func (Indexer) TableName() string { return "indexers" }
