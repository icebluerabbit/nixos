package model

import (
	"time"

	"gorm.io/gorm"
)

// How a grab stopped short of becoming a library file.
//
// Three different outcomes that an *arr reports as three different events, kept
// apart because they mean different things about whose fault it was. Folding
// them together would make "the import broke" and "I decided not to bother"
// the same answer.
const (
	// ImportFailed is a download that could not be imported — unpack error,
	// missing file, wrong content.
	ImportFailed = "failed"
	// ImportIgnored is the *arr giving up on a download deliberately. Not a
	// malfunction: a decision, with a reason attached.
	ImportIgnored = "ignored"
	// ImportIncomplete is Lidarr: some tracks of an album imported and some
	// did not, so the album is present and wrong. Nothing else in the model
	// can express a partial success.
	ImportIncomplete = "incomplete"
)

// ImportFailure is an *arr import that did not complete.
//
// The counterpart to LibraryFile: together they say what a download became, and
// a download that appears in neither is still in flight or was lost silently.
type ImportFailure struct {
	Base
	ServiceName string    `gorm:"not null;uniqueIndex:idx_impfail,priority:1"`
	SourceRef   string    `gorm:"not null;uniqueIndex:idx_impfail,priority:2"`
	At          time.Time `gorm:"not null"`
	// Kind is which of the three outcomes above this was.
	Kind string `gorm:"not null;index"`

	ContentID       *ID     `gorm:"type:uuid;index"`
	TorrentInfohash *string `gorm:"index"`
	Reason          *string
	SourcePath      *string

	Service *Service `gorm:"foreignKey:ServiceName;references:Name"`
	Content *Content `gorm:"foreignKey:ContentID"`
	Torrent *Torrent `gorm:"foreignKey:TorrentInfohash;references:Infohash"`
}

func (ImportFailure) TableName() string { return "import_failures" }

// BeforeSave strips URLs quoted inside an import failure's explanation.
func (f *ImportFailure) BeforeSave(*gorm.DB) error {
	redactText(f.Reason)
	return nil
}
