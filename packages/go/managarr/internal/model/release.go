package model

// Release is a named packaging of content.
//
//	Inside.Out.2015.1080p.BluRay.DTS.x264-NTb
//
// The hinge of the model. Announcements, grabs, searches, torrents and pack
// attempts all reference a release, so they join by foreign key rather than by
// comparing strings — and several torrents pointing at one release is exactly
// what a cross-seed IS.
//
// UUID rather than the title as key: trackers rename releases, and a primary
// key that changes takes every referencing row with it. The title stays UNIQUE,
// so identity is still enforced.
//
// ContentID is null until something links the release to a work — usually an
// *arr grab, which knows the episode or movie it was fetching. Most announced
// releases are never grabbed, so null is the normal case for the bulk of the
// corpus rather than a defect.
type Release struct {
	Base
	Title     string `gorm:"not null;uniqueIndex"`
	ContentID *ID    `gorm:"type:uuid;index"`

	// Parsed attributes, as the announcing service understood the name. Stored
	// because they are what a filter matched on, not because they are truth.
	Resolution   *string
	Source       *string
	Codec        *string
	Container    *string
	HDR          *string `gorm:"column:hdr"`
	ReleaseGroup *string
	Edition      *string
	Proper       *bool
	Repack       *bool
	Year         *int
	Season       *int
	Episode      *int
	SizeBytes    *int64

	Content *Content `gorm:"foreignKey:ContentID"`
}

func (Release) TableName() string { return "releases" }
