package store

import (
	"context"
	"testing"

	"homelab/mousetrap/internal/types"
)

// seedLedgers writes one realistic slice of every ledger: two announces (one
// accepted and grabbed, one rejected), two grabs (a plain add and a wedged
// one), two spends (a wedge on a grab and a shop purchase) and two mirrored
// bonus rows, one of them negative.
//
// The aggregates are what the Prometheus collector scrapes, so a wrong GROUP BY
// or a renamed column shows up as a silently missing or mislabelled series
// rather than as an error. One fixture asserted across every aggregate is the
// cheapest way to notice.
func seedLedgers(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()

	accepted, err := s.InsertAnnounce(ctx, Announce{
		TID: 1, Name: "A Book", MediaType: "Ebook",
		Categories: []string{"Ebooks - Fantasy", "Ebooks - SF"},
		Filetypes:  []string{"epub"},
		Languages:  []string{"english"},
		Fiction:    "Fiction", Size: 1000, Freeleech: true,
		AcceptDecision: "accept", AcceptReason: "ok",
	})
	if err != nil {
		t.Fatalf("InsertAnnounce accepted: %v", err)
	}
	if err := s.UpdateAnnounceGrab(ctx, accepted, "grab", "ok"); err != nil {
		t.Fatalf("UpdateAnnounceGrab: %v", err)
	}

	if _, err := s.InsertAnnounce(ctx, Announce{
		TID: 2, Name: "A Tape", MediaType: "Audiobook",
		Categories: []string{"Audiobooks - Horror"},
		Filetypes:  []string{"m4b"},
		Languages:  []string{"english"},
		Fiction:    "Fiction", Size: 3000,
		AcceptDecision: "reject", AcceptReason: "size_min",
	}); err != nil {
		t.Fatalf("InsertAnnounce rejected: %v", err)
	}

	// A skipped announce is the only source of SkippedWedgeBytes: no grab row
	// is written when the strict wedge rule declines.
	skipped, err := s.InsertAnnounce(ctx, Announce{
		TID: 3, Name: "A Tome", MediaType: "Ebook", Size: 7000,
		AcceptDecision: "accept", AcceptReason: "ok",
	})
	if err != nil {
		t.Fatalf("InsertAnnounce skipped: %v", err)
	}
	if err := s.UpdateAnnounceGrab(ctx, skipped, "skip", "wedge_unavailable"); err != nil {
		t.Fatalf("UpdateAnnounceGrab skip: %v", err)
	}

	wedgeSpend, err := s.InsertSpend(ctx, Spend{
		Type: types.SpendWedge, Amount: 1, TID: &[]int64{1}[0],
		Reason: types.SpendReasonGrab,
	})
	if err != nil {
		t.Fatalf("InsertSpend wedge: %v", err)
	}
	if _, err := s.InsertSpend(ctx, Spend{
		Type: types.SpendPoints, Amount: 5000,
		Reason: types.SpendReasonShopVIP, Response: "ok",
	}); err != nil {
		t.Fatalf("InsertSpend points: %v", err)
	}

	if _, err := s.InsertGrab(ctx, Grab{
		AnnounceID: &accepted, Source: types.SourceAnnounce, TID: 1, Size: 1000,
		CostsRatio: false, Result: string(types.ResultAdded),
	}); err != nil {
		t.Fatalf("InsertGrab added: %v", err)
	}
	if _, err := s.InsertGrab(ctx, Grab{
		SpendID: &wedgeSpend, Source: types.SourceTorznab, TID: 9, Size: 2000,
		CostsRatio: true, Result: string(types.ResultAddedWedge),
	}); err != nil {
		t.Fatalf("InsertGrab wedge: %v", err)
	}

	if _, err := s.InsertBonus(ctx, Bonus{
		TS: 1700000000, Type: types.BonusGiftPoints, Amount: 100,
	}); err != nil {
		t.Fatalf("InsertBonus gift: %v", err)
	}
	if _, err := s.InsertBonus(ctx, Bonus{
		TS: 1700000100, Type: types.BonusWedgePF, Amount: -50,
	}); err != nil {
		t.Fatalf("InsertBonus wedge: %v", err)
	}
}

// counter returns a flattener for grouped aggregates. It takes the aggregate's
// two return values directly, so a call reads
// `c1(s.AnnounceCategories(ctx))` — which Go only allows when the call is the
// sole argument, hence the closure over t rather than a t-first helper.
func counter(t *testing.T) func([]Count1, error) map[string]int64 {
	t.Helper()
	return func(rows []Count1, err error) map[string]int64 {
		if err != nil {
			t.Fatalf("aggregate: %v", err)
		}
		out := make(map[string]int64, len(rows))
		for _, r := range rows {
			out[r.Key] = r.N
		}
		return out
	}
}

func wantCounts(t *testing.T, name string, got, want map[string]int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %v, want %v", name, got, want)
		return
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s[%s]: got %d, want %d", name, k, got[k], v)
		}
	}
}

func TestAnnounceAggregates(t *testing.T) {
	s := openTest(t)
	seedLedgers(t, s)
	ctx := context.Background()

	c1 := counter(t)

	total, err := s.AnnouncesTotal(ctx)
	if err != nil {
		t.Fatalf("AnnouncesTotal: %v", err)
	}
	if total != 3 {
		t.Errorf("AnnouncesTotal = %d, want 3", total)
	}

	wantCounts(t, "AnnouncesBy(media_type)",
		c1(s.AnnouncesBy(ctx, "media_type")),
		map[string]int64{"Ebook": 2, "Audiobook": 1})
	wantCounts(t, "AnnounceCategories",
		c1(s.AnnounceCategories(ctx)),
		map[string]int64{"Ebooks - Fantasy": 1, "Ebooks - SF": 1, "Audiobooks - Horror": 1})
	wantCounts(t, "AnnounceFiletypes",
		c1(s.AnnounceFiletypes(ctx)),
		map[string]int64{"epub": 1, "m4b": 1})
	wantCounts(t, "AnnounceLanguages",
		c1(s.AnnounceLanguages(ctx)),
		map[string]int64{"english": 2})
	wantCounts(t, "AnnounceBytesByMediaType",
		c1(s.AnnounceBytesByMediaType(ctx)),
		map[string]int64{"Ebook": 8000, "Audiobook": 3000})
}

// The decision aggregates are two-key: they carry the reason alongside the
// verdict, which is what makes a rejection legible on the dashboard.
func TestAnnounceDecisionAggregates(t *testing.T) {
	s := openTest(t)
	seedLedgers(t, s)
	ctx := context.Background()

	accept, err := s.AnnouncesByAccept(ctx)
	if err != nil {
		t.Fatalf("AnnouncesByAccept: %v", err)
	}
	gotAccept := map[string]int64{}
	for _, r := range accept {
		gotAccept[r.A+"/"+r.B] = r.N
	}
	wantCounts(t, "AnnouncesByAccept", gotAccept,
		map[string]int64{"accept/ok": 2, "reject/size_min": 1})

	grab, err := s.AnnouncesByGrab(ctx)
	if err != nil {
		t.Fatalf("AnnouncesByGrab: %v", err)
	}
	gotGrab := map[string]int64{}
	for _, r := range grab {
		gotGrab[r.A+"/"+r.B] = r.N
	}
	// The rejected announce was never gated, so it carries no grab decision
	// and must not appear.
	wantCounts(t, "AnnouncesByGrab", gotGrab,
		map[string]int64{"grab/ok": 1, "skip/wedge_unavailable": 1})
}

// A column outside the whitelist is refused rather than interpolated, because
// the value reaches the SQL text directly.
func TestAnnouncesByRejectsUnknownColumn(t *testing.T) {
	s := openTest(t)

	if _, err := s.AnnouncesBy(context.Background(), "name; DROP TABLE announces"); err == nil {
		t.Fatal("AnnouncesBy accepted a column outside the whitelist")
	}
}

// The histogram is a Prometheus ConstHistogram source: cumulative counts per
// bucket, plus the count and byte sum per media type.
func TestAnnounceSizeHistogram(t *testing.T) {
	s := openTest(t)
	seedLedgers(t, s)

	hist, err := s.AnnounceSizeHistogram(context.Background(), []float64{500, 2000, 8000})
	if err != nil {
		t.Fatalf("AnnounceSizeHistogram: %v", err)
	}

	byType := map[string]SizeHist{}
	for _, h := range hist {
		byType[h.MediaType] = h
	}
	ebook, ok := byType["Ebook"]
	if !ok {
		t.Fatalf("no Ebook histogram in %v", byType)
	}
	if ebook.Count != 2 || ebook.Sum != 8000 {
		t.Errorf("Ebook count/sum = %d/%v, want 2/8000", ebook.Count, ebook.Sum)
	}
	// 1000 and 7000: nothing at or below 500, one at or below 2000, both at
	// or below 8000.
	want := map[float64]uint64{500: 0, 2000: 1, 8000: 2}
	for le, n := range want {
		if ebook.Buckets[le] != n {
			t.Errorf("Ebook bucket le=%v = %d, want %d", le, ebook.Buckets[le], n)
		}
	}
}

func TestGrabAggregates(t *testing.T) {
	s := openTest(t)
	seedLedgers(t, s)
	ctx := context.Background()

	c1 := counter(t)

	wantCounts(t, "GrabsByResult",
		c1(s.GrabsByResult(ctx)),
		map[string]int64{"added": 1, "added_wedge": 1})
	wantCounts(t, "GrabsBySource",
		c1(s.GrabsBySource(ctx)),
		map[string]int64{"announce": 1, "torznab": 1})

	bytes, err := s.GrabBytesTotal(ctx)
	if err != nil {
		t.Fatalf("GrabBytesTotal: %v", err)
	}
	if bytes != 3000 {
		t.Errorf("GrabBytesTotal = %d, want 3000", bytes)
	}

	saved, err := s.WedgeSavedBytes(ctx)
	if err != nil {
		t.Fatalf("WedgeSavedBytes: %v", err)
	}
	if saved != 2000 {
		t.Errorf("WedgeSavedBytes = %d, want 2000", saved)
	}
}

func TestSpendAndBonusAggregates(t *testing.T) {
	s := openTest(t)
	seedLedgers(t, s)
	ctx := context.Background()

	c1 := counter(t)

	wantCounts(t, "SpendsByType",
		c1(s.SpendsByType(ctx)),
		map[string]int64{"wedge": 1, "points": 1})
	wantCounts(t, "SpendSumByType",
		c1(s.SpendSumByType(ctx)),
		map[string]int64{"wedge": 1, "points": 5000})
	wantCounts(t, "SpendsByReason",
		c1(s.SpendsByReason(ctx)),
		map[string]int64{"grab": 1, "shop_vip": 1})
	wantCounts(t, "BonusesByType",
		c1(s.BonusesByType(ctx)),
		map[string]int64{"giftPoints": 1, "wedgePF": 1})
	// Signed: a MAM-side spend subtracts.
	wantCounts(t, "BonusSumByType",
		c1(s.BonusSumByType(ctx)),
		map[string]int64{"giftPoints": 100, "wedgePF": -50})

	total, err := s.BonusTotal(ctx)
	if err != nil {
		t.Fatalf("BonusTotal: %v", err)
	}
	if total != 2 {
		t.Errorf("BonusTotal = %d, want 2", total)
	}
}

// Query is the ad-hoc read surface behind POST /query: arbitrary SQL in, rows
// as maps out.
func TestQueryReturnsRows(t *testing.T) {
	s := openTest(t)
	seedLedgers(t, s)

	rows, err := s.Query(context.Background(),
		`SELECT tid, media_type FROM announces ORDER BY tid`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if got := rows[0]["media_type"]; got != "Ebook" {
		t.Errorf("first media_type = %v, want Ebook", got)
	}
}

func TestQueryRejectsBadSQL(t *testing.T) {
	s := openTest(t)

	if _, err := s.Query(context.Background(), "SELECT * FROM nosuchtable"); err == nil {
		t.Fatal("Query accepted a statement against a missing table")
	}
}
