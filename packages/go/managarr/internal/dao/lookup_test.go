package dao

import (
	"strings"
	"testing"

	"homelab/managarr/internal/model"
)

// TestLookupsCreateOnFirstSight is the contract every pass relies on.
//
// A reconcile pass names an indexer, a tracker or a client before anything has
// declared one, and the foreign keys are enforced. These exist so the reference
// is always satisfiable at the moment it is made.
func TestLookupsCreateOnFirstSight(t *testing.T) {
	d, ctx := testDAO(t)
	l := NewLookups(d)

	name := uniqueName("ix")
	got, err := l.Indexer(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != name {
		t.Fatalf("got %v, want %q", got, name)
	}

	var n int64
	if err := d.DB().WithContext(ctx).Model(&model.Indexer{}).
		Where("name = ?", name).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("found %d indexer rows, want 1", n)
	}
}

// TestLookupsEmptyIsNil keeps an unset field from creating a blank row.
//
// Half the *arr history records carry no indexer at all. Creating an indexer
// named "" would give every one of them the same fabricated parent.
func TestLookupsEmptyIsNil(t *testing.T) {
	d, ctx := testDAO(t)
	l := NewLookups(d)

	if got, err := l.Indexer(ctx, ""); err != nil || got != nil {
		t.Errorf("Indexer(\"\") = %v, %v", got, err)
	}
	if got, err := l.Tracker(ctx, ""); err != nil || got != nil {
		t.Errorf("Tracker(\"\") = %v, %v", got, err)
	}
	if got, err := l.Quality(ctx, ""); err != nil || got != nil {
		t.Errorf("Quality(\"\") = %v, %v", got, err)
	}
	if got, err := l.Language(ctx, ""); err != nil || got != nil {
		t.Errorf("Language(\"\") = %v, %v", got, err)
	}
	if got, err := l.Torrent(ctx, ""); err != nil || got != nil {
		t.Errorf("Torrent(\"\") = %v, %v", got, err)
	}
	if got, err := l.Release(ctx, ""); err != nil || got != nil {
		t.Errorf("Release(\"\") = %v, %v", got, err)
	}
	if got, err := l.SoulseekPeer(ctx, ""); err != nil || got != nil {
		t.Errorf("SoulseekPeer(\"\") = %v, %v", got, err)
	}
}

// TestTorrentNormalisesTheHash is the join key's one rule.
//
// An *arr reports downloadId uppercase, qBittorrent reports it lowercase, and
// autobrr's webhook renders whatever the tracker sent. Storing both cases would
// give one torrent two rows and break every join through it.
func TestTorrentNormalisesTheHash(t *testing.T) {
	d, ctx := testDAO(t)
	l := NewLookups(d)

	hash := strings.ToUpper(model.NewID().String()[:8]) + "ABCDEF0123456789ABCDEF0123456789"
	upper, err := l.Torrent(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}
	if upper == nil || *upper != strings.ToLower(hash) {
		t.Fatalf("got %v, want lowercase", upper)
	}

	// The same hash in the other case, and with surrounding whitespace, must
	// resolve to the same row.
	l.Reset()
	lower, err := l.Torrent(ctx, "  "+strings.ToLower(hash)+"  ")
	if err != nil {
		t.Fatal(err)
	}
	if *lower != *upper {
		t.Fatalf("case produced two keys: %q and %q", *upper, *lower)
	}

	var n int64
	if err := d.DB().WithContext(ctx).Model(&model.Torrent{}).
		Where("infohash = ?", *upper).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("found %d torrents, want 1", n)
	}
}

// TestReleaseIsIdempotent is what lets an announce and a grab meet.
//
// The release is the hinge every other table joins through, created on first
// mention by whichever pass runs first. Two calls must give one id, or the
// announce and the torrent would hang off different releases.
func TestReleaseIsIdempotent(t *testing.T) {
	d, ctx := testDAO(t)
	l := NewLookups(d)

	title := uniqueName("Some.Release")
	first, err := l.Release(ctx, title)
	if err != nil {
		t.Fatal(err)
	}
	l.Reset()
	second, err := l.Release(ctx, title)
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || second == nil || *first != *second {
		t.Fatalf("got %v then %v", first, second)
	}

	var n int64
	if err := d.DB().WithContext(ctx).Model(&model.Release{}).
		Where("title = ?", title).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("found %d releases, want 1", n)
	}
}

// TestServiceAndClientAreSeparateNamespaces pins a deliberate split.
//
// A service is something with an API this reads; a client is something that
// holds downloads. qBittorrent is a client and Sonarr is a service, and giving
// them one table would make "which services answered" and "which clients hold
// this" the same question.
func TestServiceAndClientAreSeparateNamespaces(t *testing.T) {
	d, ctx := testDAO(t)
	l := NewLookups(d)

	name := uniqueName("both")
	if _, err := l.Service(ctx, name, "sonarr"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Client(ctx, name, model.ClientQBittorrent); err != nil {
		t.Fatal(err)
	}

	var svc, cli int64
	if err := d.DB().WithContext(ctx).Model(&model.Service{}).
		Where("name = ?", name).Count(&svc).Error; err != nil {
		t.Fatal(err)
	}
	if err := d.DB().WithContext(ctx).Model(&model.Client{}).
		Where("name = ?", name).Count(&cli).Error; err != nil {
		t.Fatal(err)
	}
	if svc != 1 || cli != 1 {
		t.Errorf("services=%d clients=%d, want one of each", svc, cli)
	}
}

// TestResetClearsTheDedupeCache is what makes the cache safe across passes.
//
// once() suppresses repeat work inside ONE pass. If it persisted, a row deleted
// out from under the process between passes would never be recreated and every
// reference to it would fail the foreign key.
func TestResetClearsTheDedupeCache(t *testing.T) {
	d, ctx := testDAO(t)
	l := NewLookups(d)

	name := uniqueName("ix")
	if _, err := l.Indexer(ctx, name); err != nil {
		t.Fatal(err)
	}
	// Delete behind its back; without Reset the lookup would trust its cache.
	if err := d.DB().WithContext(ctx).
		Where("name = ?", name).Delete(&model.Indexer{}).Error; err != nil {
		t.Fatal(err)
	}

	l.Reset()
	if _, err := l.Indexer(ctx, name); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := d.DB().WithContext(ctx).Model(&model.Indexer{}).
		Where("name = ?", name).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("found %d rows after Reset, want the row recreated", n)
	}
}

// TestContentByExternalIDResolvesAndMisses covers both outcomes.
//
// A miss is normal, not an error: searches and announces are recorded for
// content no *arr tracks, and returning nil is what lets those rows exist with
// a NULL content_id rather than failing the pass.
func TestContentByExternalIDResolvesAndMisses(t *testing.T) {
	d, ctx := testDAO(t)
	l := NewLookups(d)

	got, err := l.ContentByExternalID(ctx, map[string]string{model.ProviderTVDB: uniqueName("absent")})
	if err != nil {
		t.Fatalf("a miss must not error: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}

	// Zero and empty are "this record does not carry that id", not a lookup.
	if got, err := l.ContentByExternalID(ctx, map[string]string{
		model.ProviderTVDB: "0", model.ProviderIMDB: "",
	}); err != nil || got != nil {
		t.Errorf("got %v, %v; want nil, nil", got, err)
	}

	content := model.Content{Kind: model.KindSeries, Title: uniqueName("Show")}
	if err := d.DB().WithContext(ctx).Create(&content).Error; err != nil {
		t.Fatal(err)
	}
	value := uniqueName("tvdb")
	if err := d.DB().WithContext(ctx).Create(&model.ContentExternalID{
		ContentID: content.ID, Provider: model.ProviderTVDB, Value: value,
	}).Error; err != nil {
		t.Fatal(err)
	}

	l.Reset()
	got, err = l.ContentByExternalID(ctx, map[string]string{model.ProviderTVDB: value})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != content.ID {
		t.Fatalf("got %v, want %s", got, content.ID)
	}
}

// TestContentByExternalIDPrefersTheMostSpecific pins the provider order.
//
// A record carrying both a TVDB id and an IMDb id must resolve through TVDB:
// IMDb spans film and television and is the weakest of the four. Leaving this to
// map iteration would make the answer non-deterministic.
func TestContentByExternalIDPrefersTheMostSpecific(t *testing.T) {
	d, ctx := testDAO(t)
	l := NewLookups(d)

	tv := model.Content{Kind: model.KindSeries, Title: uniqueName("Show")}
	film := model.Content{Kind: model.KindMovie, Title: uniqueName("Film")}
	if err := d.DB().WithContext(ctx).Create(&tv).Error; err != nil {
		t.Fatal(err)
	}
	if err := d.DB().WithContext(ctx).Create(&film).Error; err != nil {
		t.Fatal(err)
	}

	tvdb := uniqueName("tvdb")
	imdb := uniqueName("tt")
	rows := []model.ContentExternalID{
		{ContentID: tv.ID, Provider: model.ProviderTVDB, Value: tvdb},
		{ContentID: film.ID, Provider: model.ProviderIMDB, Value: imdb},
	}
	if err := d.DB().WithContext(ctx).Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	got, err := l.ContentByExternalID(ctx, map[string]string{
		model.ProviderIMDB: imdb,
		model.ProviderTVDB: tvdb,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || *got != tv.ID {
		t.Fatalf("got %v, want the TVDB match %s", got, tv.ID)
	}
}
