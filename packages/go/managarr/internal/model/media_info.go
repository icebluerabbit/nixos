package model

// MediaInfo is the technical detail of one library file.
//
// One to one, so the library file's id is both primary and foreign key — there
// is no separate identity to assign.
//
// The bitrate fields are text rather than numeric: Lidarr sends audioBitRate as
// a string and Sonarr sends it as a number, and a numeric column fails the
// whole decode on the first Lidarr record.
type MediaInfo struct {
	LibraryFileID ID `gorm:"type:uuid;primaryKey"`

	VideoCodec            *string
	VideoBitrate          *string
	VideoBitDepth         *int
	VideoFps              *float64
	VideoDynamicRange     *string
	VideoDynamicRangeType *string
	Resolution            *string
	RunTime               *string
	ScanType              *string
	AudioCodec            *string
	AudioBitrate          *string
	AudioChannels         *float64
	AudioLanguages        *string
	AudioStreamCount      *int
	AudioBits             *string
	AudioSampleRate       *string
	Subtitles             *string

	LibraryFile *LibraryFile `gorm:"foreignKey:LibraryFileID;constraint:OnDelete:CASCADE"`
}

func (MediaInfo) TableName() string { return "media_info" }
