package reconcile

import (
	"testing"

	"homelab/managarr/internal/model"
)

// TestCountNewIsTheIncrementalStop covers the check that silently became a
// constant.
//
// The walk stops at the first page holding nothing new. Before this, searches
// and grabs stopped on `len(rows) > 0` — which only means "the page was not
// empty" — so every scheduled pass re-walked the entire history, tens of
// thousands of records, every hour. The condition has to be asked BEFORE the
// upsert, because afterwards every row exists and a page of new records is
// indistinguishable from one already stored.
func TestCountNewIsTheIncrementalStop(t *testing.T) {
	a, d, ctx := testArr(t)
	svc := uniqueService(t, d, ctx)
	p := &Prowlarr{dao: d, lookups: a.lookups, logger: a.logger, serviceName: svc}

	refs := []string{"1", "2", "3"}

	// Nothing stored yet: the whole page is new, so the walk continues.
	n, err := p.countNewSearches(ctx, refs)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("got %d new, want 3", n)
	}

	// Store two of them.
	for _, ref := range refs[:2] {
		if err := d.DB().WithContext(ctx).Create(&model.Search{
			ServiceName: svc, SourceRef: ref, StartedAt: nowUTC(),
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if n, err = p.countNewSearches(ctx, refs); err != nil {
		t.Fatal(err)
	} else if n != 1 {
		t.Fatalf("got %d new, want 1", n)
	}

	// All stored: zero new, which is what stops the walk.
	if err := d.DB().WithContext(ctx).Create(&model.Search{
		ServiceName: svc, SourceRef: "3", StartedAt: nowUTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if n, err = p.countNewSearches(ctx, refs); err != nil {
		t.Fatal(err)
	} else if n != 0 {
		t.Fatalf("got %d new, want 0 — the walk would never stop", n)
	}
}

// TestCountNewIsScopedToTheService keeps two instances from hiding each other's
// work.
//
// source_ref is Prowlarr's own row id and is unique only within one instance.
// Counting across instances would let a second Prowlarr's history stop the
// first one's walk on its very first page.
func TestCountNewIsScopedToTheService(t *testing.T) {
	a, d, ctx := testArr(t)
	other := uniqueService(t, d, ctx)
	mine := uniqueService(t, d, ctx)

	if err := d.DB().WithContext(ctx).Create(&model.Search{
		ServiceName: other, SourceRef: "1", StartedAt: nowUTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	p := &Prowlarr{dao: d, lookups: a.lookups, logger: a.logger, serviceName: mine}
	n, err := p.countNewSearches(ctx, []string{"1"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("got %d new; another instance's row was counted as mine", n)
	}
}

// TestCountNewEmptyPage is the end of the walk.
func TestCountNewEmptyPage(t *testing.T) {
	a, d, ctx := testArr(t)
	p := &Prowlarr{dao: d, lookups: a.lookups, logger: a.logger, serviceName: "x"}
	if n, err := p.countNewSearches(ctx, nil); err != nil || n != 0 {
		t.Errorf("got %d, %v", n, err)
	}
}

// TestCountNewGrabsQueriesItsTable exercises the grab side against the real
// schema.
//
// This used to also assert that a bogus table name produced an error, because
// the table was passed as a string and a typo was only discoverable on a
// production pass. It no longer can: the two variants are separate methods over
// generated accessors, so naming a table that does not exist fails to compile.
// What is left to check is that the statement this one builds runs.
func TestCountNewGrabsQueriesItsTable(t *testing.T) {
	a, d, ctx := testArr(t)
	p := &Prowlarr{dao: d, lookups: a.lookups, logger: a.logger, serviceName: "x"}
	if _, err := p.countNewGrabs(ctx, []string{"1"}); err != nil {
		t.Errorf("indexer_grabs: %v", err)
	}
}
