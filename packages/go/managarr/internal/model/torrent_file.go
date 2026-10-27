package model

// TorrentFile is one file inside a torrent.
//
// (Inode, SizeBytes) is the FileData reference, carried as plain columns rather
// than a surrogate id because that pair IS the inode's identity. It is how a
// torrent file and a library file are known to be the same bytes — the fact
// that hardlink counting could only guess at.
type TorrentFile struct {
	TorrentInfohash string `gorm:"primaryKey"`
	Path            string `gorm:"primaryKey"`

	FileDataInode     *int64 `gorm:"index:idx_tf_filedata,priority:1"`
	FileDataSizeBytes *int64 `gorm:"index:idx_tf_filedata,priority:2"`
	IsArchive         *bool

	Torrent  *Torrent  `gorm:"foreignKey:TorrentInfohash;references:Infohash;constraint:OnDelete:CASCADE"`
	FileData *FileData `gorm:"foreignKey:FileDataInode,FileDataSizeBytes;references:Inode,SizeBytes"`
}

func (TorrentFile) TableName() string { return "torrent_files" }
