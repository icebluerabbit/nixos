package model

import "gorm.io/gorm"

import "time"

// Search is one indexer query.
//
// Prowlarr proxies every query in this stack, which makes this the only record
// of what was looked for as opposed to what was taken. A search that returned
// nothing is recorded exactly like one that returned a hundred results — and
// the empty ones are the interesting half.
//
// The external ids are carried verbatim because they are how ContentID gets
// resolved afterwards: Prowlarr sends whichever id the caller had.
type Search struct {
	Base
	ServiceName string    `gorm:"not null;uniqueIndex:idx_search_source,priority:1"`
	SourceRef   string    `gorm:"not null;uniqueIndex:idx_search_source,priority:2"`
	StartedAt   time.Time `gorm:"not null;index:,sort:desc"`

	IndexerName *string `gorm:"index"`
	ContentID   *ID     `gorm:"type:uuid;index"`
	// Initiator is the asking service as it identified itself: CrossSeed,
	// Sonarr, or Prowlarr's own UI.
	Initiator   *string
	Query       *string
	QueryType   *string
	ResultCount *int
	ElapsedMs   *int
	// Cached means Prowlarr answered without touching the tracker, so the
	// timing says nothing about tracker health.
	Cached     *bool
	URL        *string `gorm:"column:url"`
	ImdbID     *string `gorm:"column:imdb_id"`
	TmdbID     *string `gorm:"column:tmdb_id"`
	TvdbID     *string `gorm:"column:tvdb_id"`
	Season     *int
	Episode    *int
	Categories *string

	Indexer *Indexer `gorm:"foreignKey:IndexerName;references:Name"`
	Content *Content `gorm:"foreignKey:ContentID"`
	Service *Service `gorm:"foreignKey:ServiceName;references:Name"`
}

func (Search) TableName() string { return "searches" }

// BeforeSave strips the passkey Prowlarr records on every query it proxies.
func (s *Search) BeforeSave(*gorm.DB) error {
	redactURL(s.URL)
	return nil
}
