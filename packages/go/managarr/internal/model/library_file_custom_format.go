package model

// LibraryFileCustomFormat links a library file to a custom format that matched
// it, which is what produced its score.
type LibraryFileCustomFormat struct {
	LibraryFileID ID     `gorm:"type:uuid;primaryKey"`
	ServiceName   string `gorm:"primaryKey"`
	FormatName    string `gorm:"primaryKey"`

	LibraryFile  *LibraryFile  `gorm:"foreignKey:LibraryFileID;constraint:OnDelete:CASCADE"`
	CustomFormat *CustomFormat `gorm:"foreignKey:ServiceName,FormatName;references:ServiceName,Name"`
}

func (LibraryFileCustomFormat) TableName() string { return "library_file_custom_formats" }
