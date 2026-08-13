package store

import "context"

// Announce is one autobrr candidate and the decision made about it. Categories,
// Filetypes and Languages are multi-valued and persisted to their announce_* child
// tables. The deprecated announces.category column is never written (see 0004).
type Announce struct {
	ID             int64
	TS             int64
	TID            int64
	Name           string
	MediaType      string
	Categories     []string // → announce_categories child rows
	Filetypes      []string // → announce_filetypes child rows
	Fiction        string   // "Fiction"/"Non-Fiction"; "" (→NULL) when unknown
	Languages      []string // reduced language set → announce_languages child rows
	Size           int64
	VIP            bool
	Freeleech      bool
	AcceptDecision string // accept|reject
	AcceptReason   string
	GrabDecision   string // grab|hold|skip; "" (→NULL) if L1 rejected / not yet gated
	GrabReason     string
	Payload        string // raw announce JSON
}

// InsertAnnounce records an announce plus its categories, filetypes and languages
// (one child row each) in a single transaction, and returns the announce row id (for
// a grab FK).
func (s *Store) InsertAnnounce(ctx context.Context, a Announce) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO announces
		 (ts, tid, name, media_type, fiction, size, vip, freeleech,
		  accept_decision, accept_reason, grab_decision, grab_reason, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tsOrNow(a.TS), a.TID, a.Name, a.MediaType, nullStr(a.Fiction), a.Size,
		boolToInt(a.VIP), boolToInt(a.Freeleech),
		a.AcceptDecision, a.AcceptReason, nullStr(a.GrabDecision), nullStr(a.GrabReason), nullStr(a.Payload),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	children := []struct {
		query  string
		values []string
	}{
		{`INSERT INTO announce_categories (announce_id, category) VALUES (?, ?)`, a.Categories},
		{`INSERT INTO announce_filetypes (announce_id, filetype) VALUES (?, ?)`, a.Filetypes},
		{`INSERT INTO announce_languages (announce_id, language) VALUES (?, ?)`, a.Languages},
	}
	for _, c := range children {
		for _, v := range c.values {
			if _, err := tx.ExecContext(ctx, c.query, id, v); err != nil {
				return 0, err
			}
		}
	}
	return id, tx.Commit()
}

// UpdateAnnounceGrab sets the Layer-2 grab decision/reason on an announce row after
// the grab-gate runs.
func (s *Store) UpdateAnnounceGrab(ctx context.Context, id int64, decision, reason string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE announces SET grab_decision = ?, grab_reason = ? WHERE id = ?`,
		decision, reason, id,
	)
	return err
}
