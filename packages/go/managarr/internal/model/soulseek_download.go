package model

import "time"

// SoulseekDownload is one file fetched from a peer.
//
// A transfer, not a holding: one-shot and terminal, where a torrent keeps
// seeding for months. slskd expires it from its API once finished, so anything
// completing between two passes is visible only through the webhook — which is
// also the only place the failure reason ever exists.
//
// Points at Content rather than Release: a Soulseek fetch is a loose file out
// of somebody's share, not a scene-named packaging, so there is no release for
// it to reference.
type SoulseekDownload struct {
	Base
	ClientName   string `gorm:"not null;index"`
	PeerUsername string `gorm:"not null;uniqueIndex:idx_slsk_dl,priority:1"`
	RemotePath   string `gorm:"not null;uniqueIndex:idx_slsk_dl,priority:2"`

	ContentID *ID `gorm:"type:uuid;index"`

	// LocalPath is where slskd WROTE the file, as opposed to RemotePath, which
	// is where the peer kept it. Stated by slskd's event log and by nothing
	// else — the transfer listing does not carry it.
	LocalPath *string `gorm:"index"`
	// The identity of those bytes on disk.
	//
	// This is what ties a Soulseek download to the library file it became.
	// Music is imported by MOVE rather than by hardlink here, and a move within
	// a filesystem preserves the inode — so the library file and the download
	// are literally the same inode, which is an exact match rather than a
	// comparison of names.
	FileDataInode     *int64 `gorm:"index:idx_slsk_dl_filedata,priority:1"`
	FileDataSizeBytes *int64 `gorm:"index:idx_slsk_dl_filedata,priority:2"`
	State             *string
	Bytes             *int64
	SizeBytes         *int64
	Error             *string
	RequestedAt       *time.Time
	StartedAt         *time.Time
	FinishedAt        *time.Time

	FileData *FileData     `gorm:"foreignKey:FileDataInode,FileDataSizeBytes;references:Inode,SizeBytes"`
	Client   *Client       `gorm:"foreignKey:ClientName;references:Name"`
	Peer     *SoulseekPeer `gorm:"foreignKey:PeerUsername;references:Username"`
	Content  *Content      `gorm:"foreignKey:ContentID"`
}

func (SoulseekDownload) TableName() string { return "soulseek_downloads" }
