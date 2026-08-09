package store

import (
	"database/sql"
	"errors"
	"time"
)

// Session is a handle to one named session row. Its Load/Save satisfy
// mam.SessionStore (structurally — store does not import mam).
type Session struct {
	db   *sql.DB
	name string
}

// Session returns a handle to the named session (e.g. "default").
func (s *Store) Session(name string) *Session {
	return &Session{db: s.db, name: name}
}

// Load returns the stored mam_id, or "" if the session has none yet.
func (se *Session) Load() (string, error) {
	var id string
	err := se.db.QueryRow(`SELECT mam_id FROM sessions WHERE name = ?`, se.name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// Save upserts the rotated mam_id for this session.
func (se *Session) Save(mamID string) error {
	_, err := se.db.Exec(
		`INSERT INTO sessions (name, mam_id, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET mam_id = excluded.mam_id, updated_at = excluded.updated_at`,
		se.name, mamID, time.Now().Unix(),
	)
	return err
}
