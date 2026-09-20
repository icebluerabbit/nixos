package model

import "time"

// ClientTorrent is a torrent as held by one client.
//
// Everything here is per-client and mutable: the same torrent in two clients
// has two progress figures, two ratios and two save paths. Everything that is
// true of the torrent regardless of who holds it lives on Torrent.
type ClientTorrent struct {
	ClientName      string `gorm:"primaryKey"`
	TorrentInfohash string `gorm:"primaryKey"`

	// State is the client's own enum, verbatim.
	State    *string
	Progress *float64

	Ratio              *float64
	UploadedBytes      *int64
	DownloadedBytes    *int64
	SeedingTimeSeconds *int64
	TimeActiveSeconds  *int64

	// Name is the client's own name for the torrent.
	//
	// Recorded because it is the identifier OTHER TOOLS QUOTE. seasonpackarr
	// asks qBittorrent for the torrent list and then writes names back out in
	// its log — "client(Show.S01E03.1080p...mkv => 1080p)" — and that string is
	// this column verbatim. Without it there is no way to turn what
	// seasonpackarr says into an infohash, and the pack-to-episode association
	// cannot be recorded at all.
	//
	// Indexed for that lookup, which is the only reason it exists.
	Name *string `gorm:"index"`

	SavePath    *string
	ContentPath *string
	Category    *string `gorm:"index"`
	Tags        *string

	AddedAt        *time.Time
	CompletedAt    *time.Time
	LastActivityAt *time.Time
	// RemovedAt records that this client no longer holds the torrent. The row
	// stays: the library file it produced still points at it, and that
	// provenance is why the row exists.
	RemovedAt *time.Time `gorm:"index"`
	// RemovalReason is why it went, when managarr is the one that removed it.
	//
	// Written at the moment of the deletion and by nothing else. A reconcile
	// pass can only observe that a torrent is absent, which is the same
	// observation whether the tracker unregistered it, a *arr cleaned it up or
	// somebody removed it by hand — and those justify very different things
	// afterwards.
	//
	// It matters for deciding whether a library file is orphaned. Dropping a
	// library entry is defensible when every torrent that provided it died
	// BECAUSE the tracker dropped it; it is not defensible because the torrent
	// merely happens to be missing. Without this column that distinction is
	// gone the moment the row is marked removed, which is how this model ended
	// up with 39 imports nothing can explain.
	//
	// NULL means managarr did not remove it — the overwhelming majority.
	RemovalReason *string

	Client  *Client  `gorm:"foreignKey:ClientName;references:Name"`
	Torrent *Torrent `gorm:"foreignKey:TorrentInfohash;references:Infohash"`
}

func (ClientTorrent) TableName() string { return "client_torrents" }
