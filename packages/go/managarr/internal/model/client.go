package model

import "time"

// Client kinds.
const (
	ClientQBittorrent = "qbittorrent"
	ClientSlskd       = "slskd"
)

// Client is a download client.
//
// Separate from the torrent itself: a torrent is a thing that exists in the
// world — announced, proposed by cross-seed, grabbed by an *arr — and a client
// holding it is a different fact. Keeping them apart is what lets a torrent be
// recorded without inventing a row in a client that never had it.
type Client struct {
	Name string `gorm:"primaryKey"`

	Kind       string `gorm:"not null;index"`
	Version    *string
	ObservedAt *time.Time
}

func (Client) TableName() string { return "clients" }
