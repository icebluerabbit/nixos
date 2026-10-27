package model

// FilterIndexer is the filter-to-indexer relation.
//
// Many to many: a filter spans trackers and a tracker is used by many filters.
// No column on either side can hold this, which is why an earlier version's
// filters.indexer_id was both wrong and unenforceable.
type FilterIndexer struct {
	FilterName  string `gorm:"primaryKey"`
	IndexerName string `gorm:"primaryKey"`

	Filter  *Filter  `gorm:"foreignKey:FilterName;references:Name;constraint:OnDelete:CASCADE"`
	Indexer *Indexer `gorm:"foreignKey:IndexerName;references:Name;constraint:OnDelete:CASCADE"`
}

func (FilterIndexer) TableName() string { return "filter_indexers" }
