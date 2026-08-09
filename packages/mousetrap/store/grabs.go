package store

import (
	"context"

	"mousetrap/types"
)

// Grab is one grab action and its outcome. AnnounceID is nil for torznab-originated
// grabs; SpendID links to the wedge spend when one was used (nil = no wedge).
type Grab struct {
	ID         int64
	TS         int64
	AnnounceID *int64
	SpendID    *int64
	Source     types.Source
	TID        int64
	Size       int64
	CostsRatio bool
	Result     string // GrabResult
	Response   string // success/error response
}

// InsertGrab records a grab and returns its row id.
func (s *Store) InsertGrab(ctx context.Context, g Grab) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO grabs (ts, announce_id, spend_id, source, tid, size, costs_ratio, result, response)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tsOrNow(g.TS), g.AnnounceID, g.SpendID, string(g.Source), g.TID, g.Size,
		boolToInt(g.CostsRatio), g.Result, nullStr(g.Response),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GrabSeen reports whether tid is something we hold (dedup) — only a successful add
// counts; a dry-run, fetch failure, or rejected add is not "have it".
func (s *Store) GrabSeen(ctx context.Context, tid int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM grabs WHERE tid = ? AND result IN ('added', 'added_wedge')`, tid,
	).Scan(&n)
	return n > 0, err
}

// InFlightSince aggregates the optimistic tally over the reconciliation window:
// count + ratio-costing bytes from grabs whose download actually happened (so it
// consumed a MAM unsat slot / ratio), plus wedges from spends. Results where no
// download occurred — dryrun, fetch_failed, no_url, none — are excluded.
func (s *Store) InFlightSince(ctx context.Context, since int64) (types.InFlight, error) {
	var f types.InFlight
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN costs_ratio = 1 THEN size ELSE 0 END), 0)
		 FROM grabs WHERE ts >= ? AND result NOT IN ('dryrun', 'fetch_failed', 'no_url', 'none')`, since,
	).Scan(&f.Count, &f.RatioBytes)
	if err != nil {
		return f, err
	}
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM spends WHERE type = 'wedge' AND ts >= ?`, since,
	).Scan(&f.Wedges)
	return f, err
}
