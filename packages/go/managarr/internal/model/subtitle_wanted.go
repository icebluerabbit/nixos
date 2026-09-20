package model

import "time"

// SubtitleWanted is a subtitle Bazarr is looking for and has not found.
//
// The subtitle equivalent of an *arr's wanted list: the gap, recorded while it
// is still a gap.
type SubtitleWanted struct {
	ContentID    ID     `gorm:"type:uuid;primaryKey"`
	LanguageName string `gorm:"primaryKey"`

	LanguageCode    *string
	Forced          *bool
	HearingImpaired *bool
	ObservedAt      time.Time `gorm:"not null"`

	Content  *Content  `gorm:"foreignKey:ContentID;constraint:OnDelete:CASCADE"`
	Language *Language `gorm:"foreignKey:LanguageName;references:Name"`
}

func (SubtitleWanted) TableName() string { return "subtitle_wanted" }
