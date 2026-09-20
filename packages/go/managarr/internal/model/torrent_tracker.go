package model

import "gorm.io/gorm"

// TorrentTracker is one tracker entry on one torrent, with its last status.
//
// StatusCode is qBittorrent's raw enum — 0 disabled, 1 not contacted, 2
// working, 3 updating, 4 not working, 5 tracker error — kept alongside the
// decoded name so a code this build does not recognise is still recorded.
//
// The partial index covers the failing states, which is what anything asking
// "is this torrent still announcing" actually wants.
type TorrentTracker struct {
	TorrentInfohash     string `gorm:"primaryKey"`
	TrackerAnnounceHost string `gorm:"primaryKey"`

	Status     *string
	Message    *string
	StatusCode *int    `gorm:"index:,where:status_code >= 4"`
	URL        *string `gorm:"column:url"`

	Torrent *Torrent `gorm:"foreignKey:TorrentInfohash;references:Infohash;constraint:OnDelete:CASCADE"`
	Tracker *Tracker `gorm:"foreignKey:TrackerAnnounceHost;references:AnnounceHost"`
}

func (TorrentTracker) TableName() string { return "torrent_trackers" }

// BeforeSave strips the announce key, which for most private trackers IS the
// whole credential — the announce URL authenticates by being known.
func (t *TorrentTracker) BeforeSave(*gorm.DB) error {
	redactURL(t.URL)
	redactText(t.Message)
	return nil
}
