package store

import (
	"context"

	"mousetrap/types"
)

// Spend is one of our own outgoing spends — a wedge on a grab or points in the shop.
// Recorded when we spend, before MAM's userBonusHistory reflects it, so it drives
// the optimistic in-flight wedge count and the spent metrics. Response is set only
// for shop spends (nil for a wedge-on-grab).
type Spend struct {
	ID       int64
	TS       int64
	Type     types.SpendType   // wedge|points
	Amount   int64             // points spent; wedges are 1 each
	TID      *int64            // torrent for a wedge-on-grab; nil for shop
	Reason   types.SpendReason // grab|shop_vip|shop_upload
	Response string
}

// InsertSpend records a spend and returns its row id (linked from grabs.spend_id
// for a wedge-on-grab).
func (s *Store) InsertSpend(ctx context.Context, sp Spend) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO spends (ts, type, amount, tid, reason, response) VALUES (?, ?, ?, ?, ?, ?)`,
		tsOrNow(sp.TS), string(sp.Type), sp.Amount, sp.TID, string(sp.Reason), nullStr(sp.Response),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
