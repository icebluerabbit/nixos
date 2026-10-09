package mam

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// numish holds a MAM field that arrives as either a JSON string or a bare number.
// ratio is "10.00" for some accounts but a plain number (e.g. 4.71) for others; it
// is stored verbatim as its string form so the existing parsers handle both.
type numish string

func (n *numish) UnmarshalJSON(b []byte) error {
	if s := strings.TrimSpace(string(b)); len(s) >= 1 && s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		*n = numish(str)
		return nil
	} else { //nolint:revive // the else binds the s from the if's initializer
		*n = numish(s)
		return nil
	}
}

// UserParams drives jsonLoad.php. The bool fields are presence-only query flags and
// (per MAM) apply only to the self view; setting ID loads another user and disables
// the self-only additions.
type UserParams struct {
	ID            int  // userid to load; 0 = self
	SnatchSummary bool // add the torrent breakdown (self only)
	Notif         bool // add notifications (self only)
	ClientStats   bool // add client list + connectable status (self only, 30 min cache)
	Pretty        bool // JSON_PRETTY_PRINT (self only)
}

// SnatchCount is one bucket of the snatch-summary breakdown. MAM sends a bucket as
// either a bare integer or an object; the unsat bucket also carries a limit.
type SnatchCount struct {
	Count int `json:"count"`
	Limit int `json:"limit,omitempty"` // populated for the unsat bucket
}

// UnmarshalJSON accepts both forms MAM uses: a bare integer or a {count,limit} object.
func (s *SnatchCount) UnmarshalJSON(b []byte) error {
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		s.Count = n
		return nil
	}
	type alias SnatchCount
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*s = SnatchCount(a)
	return nil
}

// SnatchSummary is the snatch-summary block. MAM moved the breakdown under a
// "snatch_summary" key — adding the cache timestamps below — in September 2026;
// before that the buckets sat at the top level of the response. Both shapes are
// decoded and User.UnmarshalJSON promotes whichever arrived, so one reshuffle
// cannot silently empty the account snapshot again.
type SnatchSummary struct {
	// Created/CreatedAt report when MAM built the summary. It is cached, so these
	// say how stale the buckets are — a fresh HTTP response is not a fresh count.
	Created   int64  `json:"created"`
	CreatedAt string `json:"created_at"`

	Wedges    int          `json:"wedges,omitempty"`
	Unsat     *SnatchCount `json:"unsat,omitempty"`
	SSat      *SnatchCount `json:"sSat,omitempty"`
	SeedHnr   *SnatchCount `json:"seedHnr,omitempty"`
	SeedUnsat *SnatchCount `json:"seedUnsat,omitempty"`
	Leeching  *SnatchCount `json:"leeching,omitempty"`
	InactHnr  *SnatchCount `json:"inactHnr,omitempty"`
}

// User is the jsonLoad.php response. The base fields are always present; the rest
// appear only with the matching request flag. ratio/uploaded/downloaded are
// display-formatted strings.
type User struct {
	// base — always present. MAM is inconsistent about string vs bare-number JSON for
	// several of these (uid/ratio/uploaded/downloaded), so they decode via numish.
	UID        numish `json:"uid"`
	Username   string `json:"username"`
	ClassName  string `json:"classname"`
	Ratio      numish `json:"ratio"`      // formatted string or bare number
	Uploaded   numish `json:"uploaded"`   // formatted string or bare number
	Downloaded numish `json:"downloaded"` // formatted string or bare number
	SeedBonus  int    `json:"seedbonus"`

	// with Notif
	Notifs json.RawMessage `json:"notifs,omitempty"`

	// with SnatchSummary — the torrent breakdown, nested under "snatch_summary"
	// since September 2026 and at the top level before that. Each bucket carries a
	// count (unsat also a limit). MAM's breakdown includes further buckets beyond
	// those named here. Read these fields, not Summary: UnmarshalJSON fills them
	// from either shape.
	Summary   *SnatchSummary `json:"snatch_summary,omitempty"`
	Wedges    int            `json:"wedges,omitempty"`
	Unsat     *SnatchCount   `json:"unsat,omitempty"`
	SSat      *SnatchCount   `json:"sSat,omitempty"`
	SeedHnr   *SnatchCount   `json:"seedHnr,omitempty"`
	SeedUnsat *SnatchCount   `json:"seedUnsat,omitempty"`
	Leeching  *SnatchCount   `json:"leeching,omitempty"`
	InactHnr  *SnatchCount   `json:"inactHnr,omitempty"`
}

// UnmarshalJSON decodes the response and, when the breakdown arrived nested under
// "snatch_summary", promotes its buckets onto the top-level fields so callers read
// one shape either way. A top-level bucket wins, since only the older unnested
// form sets one.
func (u *User) UnmarshalJSON(b []byte) error {
	type alias User
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*u = User(a)

	s := u.Summary
	if s == nil {
		return nil
	}
	if u.Wedges == 0 {
		u.Wedges = s.Wedges
	}
	if u.Unsat == nil {
		u.Unsat = s.Unsat
	}
	if u.SSat == nil {
		u.SSat = s.SSat
	}
	if u.SeedHnr == nil {
		u.SeedHnr = s.SeedHnr
	}
	if u.SeedUnsat == nil {
		u.SeedUnsat = s.SeedUnsat
	}
	if u.Leeching == nil {
		u.Leeching = s.Leeching
	}
	if u.InactHnr == nil {
		u.InactHnr = s.InactHnr
	}
	return nil
}

// LoadUser fetches jsonLoad.php user data.
func (c *Client) LoadUser(ctx context.Context, p UserParams) (*User, error) {
	q := url.Values{}
	if p.ID > 0 {
		q.Set("id", strconv.Itoa(p.ID))
	}
	if p.SnatchSummary {
		q.Set("snatch_summary", "")
	}
	if p.Notif {
		q.Set("notif", "")
	}
	if p.ClientStats {
		q.Set("clientStats", "")
	}
	if p.Pretty {
		q.Set("pretty", "")
	}
	data, err := c.do(ctx, http.MethodGet, "/jsonLoad.php", q, nil)
	if err != nil {
		return nil, err
	}
	var u User
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, jsonErr("user", err, data)
	}
	return &u, nil
}
