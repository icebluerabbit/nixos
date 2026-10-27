package model

// LibraryFileLanguage links a library file to an audio language it carries.
type LibraryFileLanguage struct {
	LibraryFileID ID     `gorm:"type:uuid;primaryKey"`
	LanguageName  string `gorm:"primaryKey"`

	LibraryFile *LibraryFile `gorm:"foreignKey:LibraryFileID;constraint:OnDelete:CASCADE"`
	Language    *Language    `gorm:"foreignKey:LanguageName;references:Name"`
}

func (LibraryFileLanguage) TableName() string { return "library_file_languages" }
