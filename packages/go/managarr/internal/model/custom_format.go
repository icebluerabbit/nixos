package model

// CustomFormat is an *arr custom format definition.
//
// Scoped to the service that defines it: two instances can use the same name
// for different rules, and a score only means something next to the instance
// that assigned it.
type CustomFormat struct {
	ServiceName string `gorm:"primaryKey"`
	Name        string `gorm:"primaryKey"`

	Score *int

	Service *Service `gorm:"foreignKey:ServiceName;references:Name"`
}

func (CustomFormat) TableName() string { return "custom_formats" }
