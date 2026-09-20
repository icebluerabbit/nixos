package model

// Providers that identify content. These are the join keys the whole model
// relies on; every service in the stack carries at least one of them.
const (
	ProviderTVDB = "tvdb"
	// ProviderTVDBEpisode is kept apart from ProviderTVDB because TheTVDB
	// numbers episodes in their own sequence: series 81189 and episode 81189 are
	// both real and unrelated, so one provider would collapse them into one row.
	ProviderTVDBEpisode = "tvdb_episode"
	ProviderTMDB        = "tmdb"
	ProviderIMDB        = "imdb"
	ProviderMusicBrainz = "musicbrainz"
	// The *arr apps' own ids. Namespaced by provider name rather than shared,
	// because two Sonarr instances number their series independently.
	ProviderSonarr = "sonarr"
	ProviderRadarr = "radarr"
	ProviderLidarr = "lidarr"
)

// ContentExternalID maps a content to one provider's identifier.
//
// This is the correlation spine. UNIQUE(provider, value) is what guarantees
// that two services reporting the same TVDB id resolve to the same Content —
// and what makes a wrong merge impossible rather than merely unlikely.
//
// It is also why nothing in this model joins on a title: Bazarr carries
// Sonarr's ids, beets carries MusicBrainz's, Prowlarr carries IMDb's. Every
// service already knows somebody else's identifier.
type ContentExternalID struct {
	ContentID ID     `gorm:"type:uuid;primaryKey"`
	Provider  string `gorm:"primaryKey;uniqueIndex:idx_ceid_provider_value,priority:1"`
	Value     string `gorm:"not null;uniqueIndex:idx_ceid_provider_value,priority:2"`

	Content *Content `gorm:"foreignKey:ContentID;constraint:OnDelete:CASCADE"`
}

func (ContentExternalID) TableName() string { return "content_external_ids" }
