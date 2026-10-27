package model

// Studio is a production company, broadcast network or record label.
//
// One dimension for three things because they occupy the same slot: Radarr
// reports a movie's studio, Sonarr a series' network, beets an album's label,
// and each gives exactly one per work. Kind records which sense it was.
//
// There is deliberately no `people` table. No service in scope exposes cast or
// crew — Sonarr's series object has no actor field, Radarr's movie object has
// no credits — and a music artist is already Content(KindArtist) rather than a
// credit on something else. A table nothing can fill is worse than none.
type Studio struct {
	Base
	Name string `gorm:"not null;uniqueIndex:idx_studio,priority:1"`
	Kind string `gorm:"not null;uniqueIndex:idx_studio,priority:2"`
}

func (Studio) TableName() string { return "studios" }

// Studio kinds.
const (
	StudioProduction = "studio"  // Radarr: movie.studio
	StudioNetwork    = "network" // Sonarr: series.network
	StudioLabel      = "label"   // beets: item.label
)
