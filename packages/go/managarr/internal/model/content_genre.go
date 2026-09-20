package model

// ContentGenre is a genre tag on a work, as the *arr reported it.
type ContentGenre struct {
	ContentID ID     `gorm:"type:uuid;primaryKey"`
	Genre     string `gorm:"primaryKey;index"`

	Content *Content `gorm:"foreignKey:ContentID;constraint:OnDelete:CASCADE"`
}

func (ContentGenre) TableName() string { return "content_genres" }
