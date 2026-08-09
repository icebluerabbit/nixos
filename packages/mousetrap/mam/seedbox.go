package mam

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// SeedboxMsg is the outcome string dynamicSeedbox.php returns in msg. Values match
// the API's "Example output" casing (the authoritative literal responses); note the
// doc's status table used slightly different casing ("No Change", "Last Change too
// recent"), so compare tolerantly if MAM proves inconsistent.
type SeedboxMsg string

const (
	SeedboxNoChange       SeedboxMsg = "No change"
	SeedboxCompleted      SeedboxMsg = "Completed"
	SeedboxTooRecent      SeedboxMsg = "Last change too recent"
	SeedboxNoCookie       SeedboxMsg = "No Session Cookie"
	SeedboxInvalidSession SeedboxMsg = "Invalid session"
	SeedboxIPMismatch     SeedboxMsg = "Invalid session - IP mismatch"
	SeedboxASNMismatch    SeedboxMsg = "Invalid session - ASN mismatch"
	SeedboxInvalidCookie  SeedboxMsg = "Invalid session - Invalid Cookie"
	SeedboxInvalidOther   SeedboxMsg = "Invalid session - Other"
	SeedboxNotAllowed     SeedboxMsg = "Incorrect session type - not allowed this function"
	SeedboxNonAPISession  SeedboxMsg = "Incorrect session type - non-API session"
)

// SeedboxOutcome is the HTTP status and human meaning the doc pairs with a message.
type SeedboxOutcome struct {
	Code    int
	Meaning string
}

// SeedboxOutcomes maps each dynamicSeedbox.php message to its HTTP status code and
// documented meaning (per the endpoint's status table). SeedboxInvalidOther is not
// in the table but appears in the example outputs, mapped here as a 403.
var SeedboxOutcomes = map[SeedboxMsg]SeedboxOutcome{
	SeedboxNoChange:       {http.StatusOK, "The IP you accessed from is already the one set"},
	SeedboxCompleted:      {http.StatusOK, "The system has finished the update"},
	SeedboxTooRecent:      {http.StatusTooManyRequests, "The update was quashed because you changed too recently"},
	SeedboxNoCookie:       {http.StatusForbidden, "The mam_id session cookie was not properly provided"},
	SeedboxInvalidSession: {http.StatusForbidden, "Session invalid (bad mam_id value, or moved off the locked IP/ASN)"},
	SeedboxIPMismatch:     {http.StatusForbidden, "Session is locked to a single IP and you are not accessing from it"},
	SeedboxASNMismatch:    {http.StatusForbidden, "Session is locked to an ASN list and you are accessing from a different one"},
	SeedboxInvalidCookie:  {http.StatusForbidden, "The cookie could not be decoded (bad/corrupted value)"},
	SeedboxInvalidOther:   {http.StatusForbidden, "Other invalid-session reason"},
	SeedboxNotAllowed:     {http.StatusForbidden, "The session is not allowed to set the dynamic seedbox"},
	SeedboxNonAPISession:  {http.StatusForbidden, "The session is a full normal web session, not allowed to set the dynamic seedbox"},
}

// SeedboxResult is the dynamicSeedbox.php response. Msg is the typed outcome and
// Success reflects it; look Msg up in SeedboxOutcomes for the status code + meaning.
// IP/ASN/AS describe the address the call was made from.
type SeedboxResult struct {
	Success bool       `json:"Success"`
	Msg     SeedboxMsg `json:"msg"`
	IP      string     `json:"ip"`
	ASN     int        `json:"ASN"`
	AS      string     `json:"AS"`
}

// SetDynamicSeedbox sets the dynamic seedbox IP (dynamicSeedbox.php). It requires an
// IP/ASN-locked session that is allowed to set the dynamic seedbox (distinct from a
// normal web/API session), is served from t.myanonamouse.net (not www), and is
// rate-limited to once per hour (rolling).
func (c *Client) SetDynamicSeedbox(ctx context.Context) (*SeedboxResult, error) {
	// dynamicSeedbox lives on the tracker host, not the main site.
	data, err := c.do(ctx, http.MethodGet, c.trackerUrl+"/json/dynamicSeedbox.php", nil, nil)
	// 403/429 are normal outcomes that still carry a JSON body (Success/msg), so
	// parse the body and treat an HTTPError as a result, not a transport failure.
	var res SeedboxResult
	if len(data) > 0 {
		_ = json.Unmarshal(data, &res)
	}
	if err != nil {
		var he *HTTPError
		if errors.As(err, &he) {
			return &res, nil
		}
		return nil, err
	}
	return &res, nil
}
