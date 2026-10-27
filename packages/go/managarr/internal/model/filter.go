package model

import "time"

// Filter is an autobrr filter.
type Filter struct {
	Name string `gorm:"primaryKey"`

	Enabled      *bool
	Priority     *int
	ActionsCount *int
	CreatedAt    *time.Time
	UpdatedAt    *time.Time
}

func (Filter) TableName() string { return "filters" }
