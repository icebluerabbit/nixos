package store

import "context"

// Announce is one autobrr candidate and the decision made about it. Languages is
// multi-valued and persisted to the announce_languages child table.
type Announce struct {
	ID             int64
	TS             int64
	TID            int64
	Name           string
	Category       string
	Kind           string
	Filetype       string
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

// InsertAnnounce records an announce plus its languages (one child row each) in a
// single transaction, and returns the announce row id (for a grab FK).
func (s *Store) InsertAnnounce(ctx context.Context, a Announce) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO announces
		 (ts, tid, name, category, kind, filetype, size, vip, freeleech,
		  accept_decision, accept_reason, grab_decision, grab_reason, payload)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tsOrNow(a.TS), a.TID, a.Name, a.Category, a.Kind, a.Filetype, a.Size,
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
	for _, lang := range a.Languages {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO announce_languages (announce_id, language) VALUES (?, ?)`, id, lang); err != nil {
			return 0, err
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
