package model

import "time"

// SoulseekUpload is one file a peer took from this share.
//
// The only record in the whole model of demand rather than supply. Everything
// else here says what was wanted, found and obtained; this says what somebody
// ELSE wanted from what is already held — which file, how often, and by how
// many different people.
//
// One row per completed transfer, not per file. The aggregate is the
// interesting part, but it is a question to ask of the rows rather than a
// number to keep: "uploaded 86 times" and "uploaded 86 times to one peer over
// an hour" are different facts, and only the raw rows can tell them apart.
type SoulseekUpload struct {
	// TransferID is slskd's own id for the transfer, unique across every
	// upload it has ever recorded. A natural key, so no surrogate is needed.
	TransferID string `gorm:"primaryKey"`

	ClientName   string `gorm:"not null;index"`
	PeerUsername string `gorm:"not null;index"`

	// LocalPath is the file of yours that was taken; RemotePath is how it
	// appeared to the peer, backslash-separated in Soulseek's own style.
	LocalPath  *string `gorm:"index"`
	RemotePath string  `gorm:"not null"`

	// LibraryFileID is which piece of the library this was, resolved through
	// the inode rather than through the path: a rename by an *arr changes the
	// path and leaves the inode alone.
	LibraryFileID     *ID    `gorm:"type:uuid;index"`
	FileDataInode     *int64 `gorm:"index:idx_slsk_up_filedata,priority:1"`
	FileDataSizeBytes *int64 `gorm:"index:idx_slsk_up_filedata,priority:2"`

	SizeBytes *int64
	Bytes     *int64
	State     *string

	RequestedAt *time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time `gorm:"index:,sort:desc"`

	Client      *Client       `gorm:"foreignKey:ClientName;references:Name"`
	Peer        *SoulseekPeer `gorm:"foreignKey:PeerUsername;references:Username"`
	LibraryFile *LibraryFile  `gorm:"foreignKey:LibraryFileID"`
	FileData    *FileData     `gorm:"foreignKey:FileDataInode,FileDataSizeBytes;references:Inode,SizeBytes"`
}

func (SoulseekUpload) TableName() string { return "soulseek_uploads" }
