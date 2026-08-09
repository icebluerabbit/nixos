package mam

import (
	"context"
	"encoding/json"
	"net/http"
)

// IPInfo is the jsonIp.php response: the current IP, its ASN, and AS organization.
type IPInfo struct {
	IP  string `json:"ip"`
	ASN int    `json:"ASN"`
	AS  string `json:"AS"` // AS organization name
}

// IP fetches the current IP / ASN / AS org (jsonIp.php). Available on both
// www.myanonamouse.net and t.myanonamouse.net.
func (c *Client) IP(ctx context.Context) (*IPInfo, error) {
	data, err := c.do(ctx, http.MethodGet, "/json/jsonIp.php", nil, nil)
	if err != nil {
		return nil, err
	}
	var info IPInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, jsonErr("ip", err, data)
	}
	return &info, nil
}
