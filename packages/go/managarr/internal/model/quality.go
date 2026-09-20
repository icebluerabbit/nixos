package model

// Quality is an *arr quality definition.
type Quality struct {
	Name string `gorm:"primaryKey"`

	Source     *string
	Resolution *int
}

func (Quality) TableName() string { return "qualities" }
