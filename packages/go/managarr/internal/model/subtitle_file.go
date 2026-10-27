package model

import "time"

// SubtitleFile is a subtitle Bazarr fetched.
//
// Correlates three ways with no title matching: Bazarr carries the Sonarr and
// Radarr ids, and its subtitles_path sits in the same /mnt/media tree as the
// library file it belongs to.
//
// Score and the match lists are Bazarr's own assessment of how well the
// subtitle fits the release — the reason it chose this one over another.
type SubtitleFile struct {
	Base
	Path string `gorm:"not null;uniqueIndex"`

	LibraryFileID   *ID `gorm:"type:uuid;index"`
	ContentID       *ID `gorm:"type:uuid;index"`
	LanguageName    *string
	LanguageCode    *string
	Forced          *bool
	HearingImpaired *bool
	Provider        *string
	ProviderSubID   *string
	Score           *float64
	Matches         *string
	Mismatches      *string
	Blacklisted     *bool
	Upgradable      *bool
	DownloadedAt    *time.Time

	LibraryFile *LibraryFile `gorm:"foreignKey:LibraryFileID"`
	Content     *Content     `gorm:"foreignKey:ContentID"`
	Language    *Language    `gorm:"foreignKey:LanguageName;references:Name"`
}

func (SubtitleFile) TableName() string { return "subtitle_files" }
