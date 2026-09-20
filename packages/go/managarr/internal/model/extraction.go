package model

import "time"

// Extraction is unpackerr unpacking an archive.
//
// The extracted file is a NEW inode. When unpackerr extracts in place the *arr
// may then hardlink it, in which case a library file shares that inode — and
// the archive it came from does not. That is precisely the case hardlink
// counting got wrong, and the reason this is recorded explicitly.
type Extraction struct {
	TorrentInfohash string `gorm:"primaryKey"`
	ArchivePath     string `gorm:"primaryKey"`

	ExtractedPath     *string
	FileDataInode     *int64
	FileDataSizeBytes *int64
	Status            *string
	StartedAt         *time.Time
	FinishedAt        *time.Time

	Torrent  *Torrent  `gorm:"foreignKey:TorrentInfohash;references:Infohash;constraint:OnDelete:CASCADE"`
	FileData *FileData `gorm:"foreignKey:FileDataInode,FileDataSizeBytes;references:Inode,SizeBytes"`
}

func (Extraction) TableName() string { return "extractions" }
