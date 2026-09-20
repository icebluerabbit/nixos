package model

// Language is an *arr language.
type Language struct {
	Name string `gorm:"primaryKey"`
	// Code is the ISO 639 short form where known; Bazarr works in these while
	// the *arr apps work in names.
	Code *string `gorm:"index"`
}

func (Language) TableName() string { return "languages" }
