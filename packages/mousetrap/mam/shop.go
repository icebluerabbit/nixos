package mam

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// SpendType selects what bonusBuy.php purchases.
type SpendType string

const (
	SpendVIP    SpendType = "VIP"
	SpendUpload SpendType = "upload"
	SpendWedges SpendType = "wedges"
)

// BuyParams drives bonusBuy.php. Amount is required with SpendUpload (an int >= 50
// GiB total, or the literal string "Max Affordable " including the trailing space).
// Duration is required with SpendVIP (the API only accepts "max", which fills to the
// 90-day limit, minimum 7 days). Both are int|string on the wire, carried as strings.
type BuyParams struct {
	SpendType SpendType
	Amount    string // upload only
	Duration  string // vip only ("max")
}

// BuyResult is the bonusBuy.php response. MAM does not document the output body; the
// known field is a success count.
type BuyResult struct {
	SuccessCount int `json:"success_count"`
}

// BonusBuy spends bonus points (VIP, upload credit, or wedges).
func (c *Client) BonusBuy(ctx context.Context, p BuyParams) (*BuyResult, error) {
	q := url.Values{}
	q.Set("spendtype", string(p.SpendType))
	if p.Amount != "" {
		q.Set("amount", p.Amount)
	}
	if p.Duration != "" {
		q.Set("duration", p.Duration)
	}
	data, err := c.do(ctx, http.MethodGet, "/json/bonusBuy.php", q, nil)
	if err != nil {
		return nil, err
	}
	var res BuyResult
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, jsonErr("buy", err, data)
	}
	return &res, nil
}
