package model

import "time"

// SoulseekSearch is a query slskd ran.
//
// slskd has no retention setting for searches, so its history is effectively
// complete and unbounded — which is why this homelab runs a separate pruner
// against it. Keyed on (query, started_at) because slskd's own search ids are
// not carried here and a pass re-reads the whole history.
type SoulseekSearch struct {
	Base
	Query     string    `gorm:"not null;uniqueIndex:idx_slsk_search,priority:1"`
	StartedAt time.Time `gorm:"not null;uniqueIndex:idx_slsk_search,priority:2"`

	EndedAt   *time.Time
	FileCount *int
	PeerCount *int
	State     *string

	// ResultsFetchedAt is when this search's responses were read.
	//
	// Needed because the results table cannot answer the question. Only files
	// that were actually downloaded are kept, so a search whose thousand
	// answers were all passed over writes no rows at all — and "no rows" is
	// indistinguishable from "never looked". Without this column every pass
	// would re-fetch the entire history to rediscover that it had nothing to
	// store, at a couple of hundred kilobytes per search.
	ResultsFetchedAt *time.Time `gorm:"index:,where:results_fetched_at IS NULL"`
}

func (SoulseekSearch) TableName() string { return "soulseek_searches" }
