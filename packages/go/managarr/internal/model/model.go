// Package model is the schema.
//
// These structs are the single definition of the database: GORM derives the
// DDL, the indexes and the foreign keys from the tags, and AutoMigrate
// reconciles a live database against them.
//
// # Scope
//
// TV, film and music. Books, comics, audiobooks and adult media are out of
// scope today and extend this without reshaping it: add a kind to Content and a
// per-domain detail table. The spine does not change.
//
// # Purpose
//
// Total capture and correlation. Every service in the stack writes what it
// observes, and every fact is reachable from every other — a library file to
// the torrent that produced it to the release it came from to the announcement
// that offered it to the search that found it.
//
// # Layout
//
// One table per file, one file per table, named for the table. Everything a
// table owns lives in its file: the struct, its TableName, the constants that
// bound its enumerated columns, and its GORM hooks. Nothing about a table lives
// anywhere else, so "where is X defined" is answered by the filename and never
// by a search.
//
// This file is the exception and declares no table. It holds what is common to
// all of them — the id type, the embedded Base, and the redaction helpers the
// hooks share.
//
// # GORM relations
//
// A belongs-to field must NOT share its name with the column it references.
// GORM infers direction from the names, so a child column called Infohash
// pointing at a parent column called Infohash is read as has-one — the parent
// ends up constrained to the child, against half of a composite key, and
// migration fails with "no unique constraint matching given keys". Child
// columns are therefore named for what they reference: TorrentInfohash,
// TrackerAnnounceHost.
//
// # Keys
//
// A natural key is the primary key when it is compact and stable. A UUID is the
// primary key only when the natural key is long text, mutable, or absent — and
// then the natural key is still declared UNIQUE, so identity is enforced rather
// than implied. No sequences: a surrogate that carries no meaning should at
// least not carry an ordering that invites people to read one into it.
//
// # Correlation
//
// Correlation is by SHARED IDENTIFIER, never by title comparison. Every service
// here carries somebody else's id — Bazarr carries Sonarr's, beets carries
// MusicBrainz's, Prowlarr carries IMDb's — so Content is resolved through
// ContentExternalID and everything else hangs off that. Titles are recorded
// because they are useful to read, not because anything joins on them.
package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"homelab/managarr/internal/redact"
)

// ID is a v7 UUID: time-ordered, so index locality follows insert order without
// a sequence and without leaking a row count.
type ID = uuid.UUID

// NewID returns a fresh v7 UUID, falling back to v4 if the clock source fails.
//
// Exported because a writer that must link rows together before inserting them
// — a season to its series, a file to its media info — needs the key in hand
// rather than discovered afterwards, and re-reading every row back to learn the
// id GORM assigned costs a query per row.
func NewID() ID {
	if id, err := uuid.NewV7(); err == nil {
		return id
	}
	return uuid.New()
}

// Base carries the surrogate key for tables that need one.
//
// Embedded rather than repeated so the BeforeCreate hook that fills it lives in
// exactly one place. A caller may set ID explicitly — the hook only fills a
// zero value — which is what makes a re-import reproducible.
type Base struct {
	ID ID `gorm:"type:uuid;primaryKey"`
}

// BeforeCreate assigns an id when the caller did not.
func (b *Base) BeforeCreate(*gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = NewID()
	}
	return nil
}

// Credential stripping happens in BeforeSave hooks on the models that carry a
// URL, rather than in the writers.
//
// Every URL this stack copies out of a private tracker carries a per-account
// passkey, so a URL column is a credential store unless something stops it.
// Putting that something in the reconcile passes was tried and failed: a column
// got added, the writer that filled it did not call the redactor, and 117,220
// passkeys went to disk. A hook cannot be forgotten — it fires for the reconcile
// passes, the webhook handlers and a future writer that does not exist yet,
// because it belongs to the model rather than to any caller.
//
// BeforeSave rather than BeforeCreate: an upsert that updates an existing row
// must be redacted too, and BeforeCreate does not run on the update path.

// redactURL strips credentials from an optional URL column in place.
func redactURL(s *string) {
	if s != nil && *s != "" {
		*s = redact.URL(*s)
	}
}

// redactText strips credentials from an optional free-text column in place.
//
// Separate from redactURL because a message is not a URL: it is prose that may
// contain one, so the path-shaped rules would eat legitimate words.
func redactText(s *string) {
	if s != nil && *s != "" {
		*s = redact.Text(*s)
	}
}
