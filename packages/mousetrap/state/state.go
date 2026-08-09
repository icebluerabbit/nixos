// Package state is the in-memory account snapshot service — the current-view cache
// in front of MAM (which remains the source of truth). Timers and actions refresh
// it via the MAM client; the grab gate reads it; the metrics Collector exports it.
// It is a stateful service composing the mam client, distinct from the pure policy
// engine.
package state

import (
	"context"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"mousetrap/mam"
	"mousetrap/types"
)

// State caches the latest account snapshot.
type State struct {
	mu        sync.RWMutex
	mam       *mam.Client
	account   types.AccountState
	accountAt time.Time
}

// New constructs the state service over a MAM client.
func New(m *mam.Client) *State { return &State{mam: m} }

// RefreshAccount polls MAM (jsonLoad?snatch_summary) and updates the account snapshot.
func (s *State) RefreshAccount(ctx context.Context) error {
	u, err := s.mam.LoadUser(ctx, mam.UserParams{SnatchSummary: true, Notif: true})
	if err != nil {
		return err
	}
	acct := userToAccount(u)
	s.mu.Lock()
	s.account = acct
	s.accountAt = time.Now()
	s.mu.Unlock()
	return nil
}

// Account returns the latest account snapshot (zero value until first refresh).
func (s *State) Account() types.AccountState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.account
}

// LastRefresh is when the account snapshot was last successfully refreshed (zero
// until the first refresh). Exported for the mam_refresh_timestamp metric.
func (s *State) LastRefresh() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accountAt
}

// userToAccount maps a MAM user payload onto the gate's AccountState, parsing the
// display-formatted ratio and byte counts MAM returns. Uploaded/Downloaded are kept
// separate (buffer is derived) so each exports as its own metric.
func userToAccount(u *mam.User) types.AccountState {
	a := types.AccountState{
		ClassName:       u.ClassName,
		Wedges:          u.Wedges,
		SeedBonus:       int64(u.SeedBonus),
		UploadedBytes:   parseSize(string(u.Uploaded)),
		DownloadedBytes: parseSize(string(u.Downloaded)),
	}
	a.Ratio = parseRatio(string(u.Ratio))
	if u.Unsat != nil {
		a.UnsatCount = u.Unsat.Count
		a.UnsatLimit = u.Unsat.Limit
	}
	// seeding = satisfied + hnr-but-seeding + unsatisfied-but-seeding; hnr = seed + inactive.
	a.SatisfiedCount = cnt(u.SSat)
	a.SeedUnsatCount = cnt(u.SeedUnsat)
	a.SeedHnrCount = cnt(u.SeedHnr)
	a.InactHnrCount = cnt(u.InactHnr)
	a.SeedingCount = a.SatisfiedCount + a.SeedHnrCount + a.SeedUnsatCount
	a.LeechingCount = cnt(u.Leeching)
	a.HnrCount = a.SeedHnrCount + a.InactHnrCount
	return a
}

func cnt(s *mam.SnatchCount) int {
	if s == nil {
		return 0
	}
	return s.Count
}

// parseRatio parses MAM's ratio, mapping "∞"/"inf" (downloaded == 0) to +Inf.
func parseRatio(s string) float64 {
	t := strings.ToLower(strings.TrimSpace(s))
	switch t {
	case "∞", "inf", "infinity":
		return math.Inf(1)
	}
	f, _ := strconv.ParseFloat(t, 64)
	return f
}

// sizeUnits maps a (case-folded) size suffix to its byte factor. MAM formats sizes
// with IEC binary units; decimal units are accepted as a fallback.
var sizeUnits = map[string]float64{
	"B":   1,
	"KIB": 1 << 10, "MIB": 1 << 20, "GIB": 1 << 30, "TIB": 1 << 40, "PIB": 1 << 50,
	"KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12, "PB": 1e15,
}

// parseSize converts a MAM display size like "1.50 TiB" or "6.0 MiB" to bytes.
func parseSize(s string) int64 {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0
	}
	i := 0
	for i < len(s) && (s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	num, err := strconv.ParseFloat(strings.TrimSpace(s[:i]), 64)
	if err != nil {
		return 0
	}
	factor := 1.0
	if f, ok := sizeUnits[strings.ToUpper(strings.TrimSpace(s[i:]))]; ok {
		factor = f
	}
	return int64(num * factor)
}
