package model

import "time"

// LibraryFile is a file an *arr imported into the library.
//
// This is the authoritative statement that an acquisition reached the library:
// the import record names the download AND the file it produced. Counting
// hardlinks could only guess at it, and guessed wrong for anything extracted
// from an archive or linked by cross-seed — which is the defect this whole
// service exists to correct.
//
// Provenance is three nullable foreign keys with a CHECK that at most one is
// set. Not a polymorphic parent table: a torrent, a Soulseek fetch and a rip
// share almost no columns, and inventing a common supertype for them produced a
// table with a nullable primary key.
//
// At most one, not exactly one: a file imported before this model existed has
// no known provenance, and recording the gap is better than forcing a lie.
type LibraryFile struct {
	Base
	Path string `gorm:"not null;uniqueIndex"`

	ContentID         *ID `gorm:"type:uuid;index"`
	ServiceName       *string
	FileDataInode     *int64 `gorm:"index:idx_lf_filedata,priority:1"`
	FileDataSizeBytes *int64 `gorm:"index:idx_lf_filedata,priority:2"`

	TorrentInfohash    *string `gorm:"index"`
	SoulseekDownloadID *ID     `gorm:"type:uuid;index"`
	RipID              *ID     `gorm:"type:uuid;index"`

	ImportedAt *time.Time
	// RemovedAt is when the *arr stopped tracking this file, and
	// RemovalReason is why, in the *arr's own vocabulary: Upgrade,
	// MissingFromDisk or Manual.
	//
	// Cleared on every pass for a file the *arr still lists, so it means "gone
	// as of the last look" rather than "was gone once". The bytes may well
	// still exist — an imported file is a hardlink to the still-seeding
	// torrent copy, and the *arr only drops its own link — so this records the
	// library's view, not the disk's.
	RemovedAt     *time.Time `gorm:"index:,where:removed_at IS NOT NULL"`
	RemovalReason *string
	// SupersededAt is the stronger statement: removed BECAUSE a better release
	// replaced it, rather than removed for any reason.
	SupersededAt *time.Time
	// SupersededBy is left NULL by the history pass. A delete record names the
	// file that went and never the one that replaced it, so filling this would
	// mean guessing from import ordering rather than reading an identifier.
	SupersededBy *ID `gorm:"type:uuid"`

	QualityName       *string
	QualityRevision   *int
	CustomFormatScore *int
	ReleaseGroup      *string
	SceneName         *string
	RelativePath      *string
	SeasonNumber      *int
	ArrFileID         *int

	Content          *Content          `gorm:"foreignKey:ContentID"`
	Torrent          *Torrent          `gorm:"foreignKey:TorrentInfohash;references:Infohash"`
	Soulseek         *SoulseekDownload `gorm:"foreignKey:SoulseekDownloadID"`
	Rip              *Rip              `gorm:"foreignKey:RipID"`
	Service          *Service          `gorm:"foreignKey:ServiceName;references:Name"`
	Quality          *Quality          `gorm:"foreignKey:QualityName;references:Name"`
	FileData         *FileData         `gorm:"foreignKey:FileDataInode,FileDataSizeBytes;references:Inode,SizeBytes"`
	SupersededByFile *LibraryFile      `gorm:"foreignKey:SupersededBy;references:ID"`
}

func (LibraryFile) TableName() string { return "library_files" }

// LibraryFileRemoval is one file the *arr reported deleting, with when and why.
//
// Input to a bulk update rather than a table of its own — the columns it names
// live on LibraryFile. It is declared here because the generated query API has
// to import it, and generated code cannot reach into a package's internals or
// into the ignored directory the generator itself lives in.
//
// Reason is the *arr's own vocabulary, lowercased: upgrade, missingfromdisk or
// manual. Only the first also sets superseded_at, because only the first says
// something arrived to take the file's place.
type LibraryFileRemoval struct {
	Path   string
	At     time.Time
	Reason string
}
