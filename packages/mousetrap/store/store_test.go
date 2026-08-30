package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"mousetrap/types"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestVersionOf(t *testing.T) {
	ok := map[string]int{"0001_init.sql": 1, "0042_add_x.sql": 42, "10_y.sql": 10}
	for name, want := range ok {
		got, err := versionOf(name)
		if err != nil || got != want {
			t.Errorf("versionOf(%q) = %d, %v; want %d", name, got, err, want)
		}
	}
	for _, bad := range []string{"init.sql", "abc_x.sql", "_x.sql"} {
		if _, err := versionOf(bad); err == nil {
			t.Errorf("versionOf(%q) expected error", bad)
		}
	}
}

func TestOpenMigrates(t *testing.T) {
	s := openTest(t)
	var applied int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	if applied != 6 {
		t.Errorf("migrations applied = %d, want 6", applied)
	}
	for _, tbl := range []string{
		"sessions", "announces", "spends", "grabs", "bonuses",
		"announce_categories", "announce_filetypes", "announce_languages",
	} {
		var name string
		if err := s.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&name); err != nil {
			t.Errorf("table %q missing: %v", tbl, err)
		}
	}
}

func TestOpenIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	_ = s1.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer s2.Close()
	var applied int
	if err := s2.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	if applied != 6 {
		t.Errorf("re-open applied = %d, want 6 (migrations must not re-run)", applied)
	}
}

func TestSessionRoundTrip(t *testing.T) {
	se := openTest(t).Session("default")
	if v, err := se.Load(); err != nil || v != "" {
		t.Errorf("initial Load = %q, %v; want empty", v, err)
	}
	if err := se.Save("abc"); err != nil {
		t.Fatal(err)
	}
	if v, _ := se.Load(); v != "abc" {
		t.Errorf("Load = %q, want abc", v)
	}
	if err := se.Save("def"); err != nil { // upsert
		t.Fatal(err)
	}
	if v, _ := se.Load(); v != "def" {
		t.Errorf("Load after upsert = %q, want def", v)
	}
}

func TestAnnounceRoundTrip(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	id, err := s.InsertAnnounce(ctx, Announce{
		TID: 5, MediaType: "Ebook",
		Categories:     []string{"Fantasy", "Romance"},
		Filetypes:      []string{"azw3", "epub"},
		Languages:      []string{"english"},
		Fiction:        "Fiction",
		AcceptDecision: "accept", AcceptReason: "ok",
	})
	if err != nil || id == 0 {
		t.Fatalf("InsertAnnounce = %d, %v", id, err)
	}
	// every multi-valued field fans out to its own child table
	for _, c := range []struct {
		table string
		col   string
		want  int
	}{
		{"announce_categories", "category", 2},
		{"announce_filetypes", "filetype", 2},
		{"announce_languages", "language", 1},
	} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM `+c.table+` WHERE announce_id=?`, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != c.want {
			t.Errorf("%s rows = %d, want %d", c.table, n, c.want)
		}
	}
	var fiction string
	if err := s.db.QueryRow(`SELECT fiction FROM announces WHERE id=?`, id).Scan(&fiction); err != nil {
		t.Fatal(err)
	}
	if fiction != "Fiction" {
		t.Errorf("fiction = %q, want Fiction", fiction)
	}
	if err := s.UpdateAnnounceGrab(ctx, id, "grab", "ok"); err != nil {
		t.Fatal(err)
	}
	var gd string
	if err := s.db.QueryRow(`SELECT grab_decision FROM announces WHERE id=?`, id).Scan(&gd); err != nil {
		t.Fatal(err)
	}
	if gd != "grab" {
		t.Errorf("grab_decision = %q, want grab", gd)
	}
}

func TestGrabDedupAndInFlight(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	now := time.Now().Unix()

	if _, err := s.InsertGrab(ctx, Grab{TS: now, Source: types.SourceAnnounce, TID: 1, Size: 1000, CostsRatio: true, Result: "added"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertGrab(ctx, Grab{TS: now, Source: types.SourceAnnounce, TID: 2, Size: 500, CostsRatio: true, Result: "dryrun"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertSpend(ctx, Spend{TS: now, Type: types.SpendWedge, Reason: types.SpendReasonGrab}); err != nil {
		t.Fatal(err)
	}
	// Point spends are summed by amount, not counted like wedges.
	if _, err := s.InsertSpend(ctx, Spend{TS: now, Type: types.SpendPoints, Amount: 16071, Reason: types.SpendReasonShopVIP}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertSpend(ctx, Spend{TS: now, Type: types.SpendPoints, Amount: 25000, Reason: types.SpendReasonShopUpload}); err != nil {
		t.Fatal(err)
	}
	// Outside the window: must not appear in any tally.
	if _, err := s.InsertSpend(ctx, Spend{TS: now - 5000, Type: types.SpendPoints, Amount: 999, Reason: types.SpendReasonShopVIP}); err != nil {
		t.Fatal(err)
	}

	if seen, _ := s.GrabSeen(ctx, 1); !seen {
		t.Error("real grab should be seen")
	}
	if seen, _ := s.GrabSeen(ctx, 2); seen {
		t.Error("dry-run grab must not be seen")
	}

	f, err := s.InFlightSince(ctx, now-1)
	if err != nil {
		t.Fatal(err)
	}
	if f.Count != 1 || f.RatioBytes != 1000 || f.Wedges != 1 || f.Points != 41071 {
		t.Errorf("in-flight = %+v, want {1,1000,1,41071} (dry-run and out-of-window excluded)", f)
	}
}

// TestSkippedWedgeBytes pins the wedge_unavailable byte tally. It reads announces
// rather than grabs because a skip never writes a grab row, and it must key on the
// exact reason string the policy emits — a drifted enum would silently report zero.
func TestSkippedWedgeBytes(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	add := func(tid, size int64, decision, reason string) {
		id, err := s.InsertAnnounce(ctx, Announce{TID: tid, Size: size, MediaType: "Audiobook",
			AcceptDecision: "accept", AcceptReason: "ok"})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.UpdateAnnounceGrab(ctx, id, decision, reason); err != nil {
			t.Fatal(err)
		}
	}
	add(1, 500_000_000, string(types.Skip), string(types.GrabWedgeUnavailable))
	add(2, 700_000_000, string(types.Skip), string(types.GrabWedgeUnavailable))
	add(3, 900_000_000, string(types.Skip), string(types.GrabVIPRequired)) // other skip reason
	add(4, 100_000_000, string(types.Grab), string(types.GrabOK))          // grabbed, not skipped

	n, err := s.SkippedWedgeBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1_200_000_000 {
		t.Errorf("SkippedWedgeBytes = %d, want 1200000000 (only wedge_unavailable skips)", n)
	}
}

func TestBonusCursor(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if c, _ := s.BonusCursor(ctx); c != 0 {
		t.Errorf("empty cursor = %v, want 0", c)
	}
	_, _ = s.InsertBonus(ctx, Bonus{TS: 1000.5, Type: types.BonusGiftPoints, Amount: 10})
	_, _ = s.InsertBonus(ctx, Bonus{TS: 2000.5, Type: types.BonusGiftWedge, Amount: 1})
	if c, _ := s.BonusCursor(ctx); c != 2000.5 {
		t.Errorf("cursor = %v, want 2000.5", c)
	}
}
