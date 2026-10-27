package model

import "gorm.io/gorm"

import "time"

// Torrent is the torrent itself, independent of any client holding it.
//
// Infohash is the primary key: forty hex characters, globally unique,
// immutable, and the identifier every other component already carries — the
// *arr history, cross-seed's database and autobrr's webhook all name a torrent
// this way, so they join directly rather than through a surrogate.
//
// A row here does not mean a client has it. A torrent can be announced, or
// proposed by cross-seed as a candidate, and never downloaded. Whether anything
// holds it is ClientTorrent's question.
type Torrent struct {
	Infohash string `gorm:"primaryKey"`

	ReleaseID *ID `gorm:"type:uuid;index"`

	InfohashV1     *string
	InfohashV2     *string
	Private        *bool
	TotalSizeBytes *int64
	Comment        *string
	CreatedBy      *string
	CreationDate   *time.Time
	MagnetURI      *string `gorm:"column:magnet_uri"`

	Release *Release `gorm:"foreignKey:ReleaseID"`
}

func (Torrent) TableName() string { return "torrents" }

// BeforeSave strips the announce list embedded in a magnet, while leaving the
// info-hash alone: that is the torrent's identity, not a secret.
func (t *Torrent) BeforeSave(*gorm.DB) error {
	redactURL(t.MagnetURI)
	redactText(t.Comment)
	return nil
}
