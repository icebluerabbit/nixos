package state

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"homelab/mousetrap/internal/mam"
	"homelab/mousetrap/internal/types"
)

type fakeSession struct{}

func (fakeSession) Load() (string, error) { return "mam-id", nil }
func (fakeSession) Save(string) error     { return nil }

// stateOver points a state service at a MAM mock that answers every path with
// body.
func stateOver(t *testing.T, body string, status int) *State {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(mam.New(srv.URL, "", fakeSession{}))
}

// The snapshot the gate reads is derived, not copied: seeding and hnr are sums
// of MAM's separate counters, and the display-formatted sizes and ratio are
// parsed into numbers.
func TestRefreshAccountDerivesSnapshot(t *testing.T) {
	const user = `{
		"classname":"VIP","ratio":"2.50","uploaded":"2.00 TiB","downloaded":"1.00 TiB",
		"seedbonus":1500,"wedges":4,
		"unsat":{"count":5,"limit":20},
		"sSat":{"count":100},"seedUnsat":{"count":2},"seedHnr":{"count":3},
		"inactHnr":{"count":1},"leeching":{"count":7}
	}`
	s := stateOver(t, user, http.StatusOK)

	before := time.Now()
	if err := s.RefreshAccount(context.Background()); err != nil {
		t.Fatalf("RefreshAccount: %v", err)
	}

	a := s.Account()
	if a.ClassName != "VIP" || a.Wedges != 4 || a.SeedBonus != 1500 {
		t.Errorf("identity = %+v", a)
	}
	if a.Ratio != 2.5 {
		t.Errorf("ratio = %v, want 2.5", a.Ratio)
	}
	if a.UploadedBytes != 2*(1<<40) || a.DownloadedBytes != 1<<40 {
		t.Errorf("transfer = %d/%d", a.UploadedBytes, a.DownloadedBytes)
	}
	if a.UnsatCount != 5 || a.UnsatLimit != 20 {
		t.Errorf("unsat = %d/%d", a.UnsatCount, a.UnsatLimit)
	}
	// 100 satisfied + 3 seeding-but-hnr + 2 seeding-but-unsatisfied.
	if a.SeedingCount != 105 {
		t.Errorf("seeding = %d, want 105", a.SeedingCount)
	}
	// 3 seeding-hnr + 1 inactive-hnr.
	if a.HnrCount != 4 {
		t.Errorf("hnr = %d, want 4", a.HnrCount)
	}
	if a.LeechingCount != 7 {
		t.Errorf("leeching = %d, want 7", a.LeechingCount)
	}

	if s.LastRefresh().Before(before) {
		t.Errorf("LastRefresh = %v, want at or after %v", s.LastRefresh(), before)
	}
}

// Absent counters are zero rather than a nil dereference: MAM omits the
// snatch summary unless it is asked for, and an older account may lack fields.
func TestRefreshAccountToleratesMissingCounters(t *testing.T) {
	s := stateOver(t, `{"classname":"User","ratio":"∞","uploaded":"500 B"}`, http.StatusOK)

	if err := s.RefreshAccount(context.Background()); err != nil {
		t.Fatalf("RefreshAccount: %v", err)
	}

	a := s.Account()
	if !math.IsInf(a.Ratio, 1) {
		t.Errorf("ratio = %v, want +Inf", a.Ratio)
	}
	if a.UnsatLimit != 0 || a.SeedingCount != 0 || a.HnrCount != 0 {
		t.Errorf("absent counters produced %+v", a)
	}
}

// A failed poll leaves the previous snapshot in place: a stale account is a
// better gate input than an empty one, which would read as a zero unsat limit.
func TestRefreshAccountKeepsSnapshotOnError(t *testing.T) {
	s := stateOver(t, "nope", http.StatusInternalServerError)

	if err := s.RefreshAccount(context.Background()); err == nil {
		t.Fatal("RefreshAccount succeeded against a failing MAM")
	}
	var zero types.AccountState
	if got := s.Account(); got != zero {
		t.Errorf("account = %+v, want the zero snapshot", got)
	}
	if !s.LastRefresh().IsZero() {
		t.Errorf("LastRefresh = %v, want zero", s.LastRefresh())
	}
}
