package store

import (
	"context"
	"database/sql"

	"homelab/mousetrap/internal/types"
)

// Bonus is one row mirrored from MAM userBonusHistory (income and MAM's own spend
// records). Amount is signed (negative = spent). Feeds received/history metrics.
type Bonus struct {
	ID          int64
	TS          float64 // MAM timestamp is fractional unix
	Type        types.BonusType
	Amount      int64
	TID         *int64
	OtherUserID *int64
	OtherName   *string
}

// InsertBonus records a mirrored bonus row and returns its row id.
func (s *Store) InsertBonus(ctx context.Context, b Bonus) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO bonuses (ts, type, amount, tid, other_userid, other_name) VALUES (?, ?, ?, ?, ?, ?)`,
		b.TS, string(b.Type), b.Amount, b.TID, b.OtherUserID, b.OtherName,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// BonusCursor returns MAX(ts) over the ledger — the newest row already mirrored, so
// a MAM poll inserts only rows newer than this (dedup without a unique constraint).
// Returns 0 when the ledger is empty.
func (s *Store) BonusCursor(ctx context.Context) (float64, error) {
	var ts sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(ts) FROM bonuses`).Scan(&ts); err != nil {
		return 0, err
	}
	return ts.Float64, nil
}
