package mam

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// BonusParams drives userBonusHistory.php.
type BonusParams struct {
	OtherUserID int      // other side of a gift transaction; 0 = any
	Type        []string // giftPoints|giftWedge|wedgePF|wedgeGFL|torrentThanks|millionaires
}

// BonusEntry is one row of the bonus/wedge ledger. tid, other_userid, and other_name
// are nullable; title arrives as either a string or an integer, so it is kept raw.
type BonusEntry struct {
	Timestamp   float64         `json:"timestamp"` // unix seconds, fractional
	Amount      int             `json:"amount"`    // signed; negative = spent
	Type        string          `json:"type"`
	TID         *int64          `json:"tid"`
	Title       json.RawMessage `json:"title"` // string or int, or null
	OtherUserID *int64          `json:"other_userid"`
	OtherName   *string         `json:"other_name"`
}

// BonusHistory fetches the bonus/wedge ledger (userBonusHistory.php).
func (c *Client) BonusHistory(ctx context.Context, p BonusParams) ([]BonusEntry, error) {
	q := url.Values{}
	if p.OtherUserID > 0 {
		q.Set("other_userid", strconv.Itoa(p.OtherUserID))
	}
	for _, t := range p.Type {
		q.Add("type[]", t)
	}
	data, err := c.do(ctx, http.MethodGet, "/json/userBonusHistory.php", q, nil)
	if err != nil {
		return nil, err
	}
	var out []BonusEntry
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, jsonErr("bonus", err, data)
	}
	return out, nil
}
