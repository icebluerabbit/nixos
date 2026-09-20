package model

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

// Redaction is tested here, in one file, rather than beside each model.
//
// The property under test is not "Offer redacts its URL" — it is "no model can
// write a passkey to disk". That is a statement about the set, and checking it
// per file would let a newly added URL column pass by simply never getting a
// test of its own, which is exactly how 117,220 passkeys reached the database.

// passkeyURL is a download link in the shape private trackers actually use: the
// credential is a query parameter.
const passkeyURL = "https://tracker.example/download.php?id=12345&passkey=8f2a91c4de77b03e5a16"

// pathKeyURL is the other shape: the credential IS a path segment, which is how
// most of this homelab's trackers authenticate an RSS or announce URL.
const pathKeyURL = "https://tracker.example/rss/8f2a91c4de77b03e5a16c9d4/movies"

func ptr(s string) *string { return &s }

// leaks reports whether a redacted value still contains the secret.
func leaks(t *testing.T, got *string) {
	t.Helper()
	if got == nil {
		t.Fatal("value became nil")
	}
	if strings.Contains(*got, "8f2a91c4de77b03e5a16") {
		t.Fatalf("credential survived redaction: %s", *got)
	}
}

func TestOfferRedactsBothLinks(t *testing.T) {
	o := Offer{InfoURL: ptr(passkeyURL), DownloadURL: ptr(pathKeyURL)}
	if err := o.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	leaks(t, o.InfoURL)
	leaks(t, o.DownloadURL)
}

func TestSearchRedactsURL(t *testing.T) {
	s := Search{URL: ptr(passkeyURL)}
	if err := s.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	leaks(t, s.URL)
}

func TestIndexerGrabRedactsURL(t *testing.T) {
	g := IndexerGrab{URL: ptr(pathKeyURL)}
	if err := g.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	leaks(t, g.URL)
}

func TestTorrentTrackerRedactsAnnounceAndMessage(t *testing.T) {
	tr := TorrentTracker{
		URL:     ptr("https://tracker.example/announce/8f2a91c4de77b03e5a16c9d4"),
		Message: ptr("failed to reach " + passkeyURL),
	}
	if err := tr.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	leaks(t, tr.URL)
	leaks(t, tr.Message)
}

// TestTorrentKeepsInfohash is the one thing redaction must NOT eat.
//
// A v1 infohash is forty hex characters, which is exactly the shape of a
// path-embedded tracker key. It is the torrent's identity and the join key for
// half this schema, so losing it would break more than the leak it prevents.
func TestTorrentKeepsInfohash(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef01234567"
	magnet := "magnet:?xt=urn:btih:" + hash +
		"&tr=https://tracker.example/announce/8f2a91c4de77b03e5a16c9d4"

	tor := Torrent{MagnetURI: &magnet, Comment: ptr("from " + passkeyURL)}
	if err := tor.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	leaks(t, tor.MagnetURI)
	leaks(t, tor.Comment)
	if !strings.Contains(*tor.MagnetURI, hash) {
		t.Fatalf("infohash was redacted out of the magnet: %s", *tor.MagnetURI)
	}
}

func TestProseColumnsAreRedacted(t *testing.T) {
	reason := "rejected: could not fetch " + passkeyURL

	d := Decision{Reason: ptr(reason)}
	if err := d.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	leaks(t, d.Reason)

	q := QueueItem{ErrorMessage: ptr(reason)}
	if err := q.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	leaks(t, q.ErrorMessage)

	b := BlocklistEntry{Reason: ptr(reason)}
	if err := b.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	leaks(t, b.Reason)

	f := ImportFailure{Reason: ptr(reason)}
	if err := f.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	leaks(t, f.Reason)

	h := HealthEvent{Message: ptr("Indexer unavailable: " + passkeyURL)}
	if err := h.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	leaks(t, h.Message)
}

// TestRedactionToleratesNil covers the common case: the column is unset.
//
// Every redacted column is a pointer, and most rows leave most of them nil. A
// hook that dereferenced unconditionally would panic on the majority of writes.
func TestRedactionToleratesNil(t *testing.T) {
	hooks := []func(*gorm.DB) error{
		(&Offer{}).BeforeSave,
		(&Search{}).BeforeSave,
		(&IndexerGrab{}).BeforeSave,
		(&TorrentTracker{}).BeforeSave,
		(&Torrent{}).BeforeSave,
		(&Decision{}).BeforeSave,
		(&QueueItem{}).BeforeSave,
		(&BlocklistEntry{}).BeforeSave,
		(&ImportFailure{}).BeforeSave,
		(&HealthEvent{}).BeforeSave,
	}
	for i, hook := range hooks {
		if err := hook(nil); err != nil {
			t.Fatalf("hook %d on an empty model: %v", i, err)
		}
	}
}

// TestRedactionLeavesCleanValuesAlone guards against over-eager matching.
//
// An info URL with a numeric id is provenance, not a credential. Destroying it
// would remove the ability to say what a record refers to.
func TestRedactionLeavesCleanValuesAlone(t *testing.T) {
	const clean = "https://tracker.example/torrents.php?id=91234&torrent_id=55"
	o := Offer{InfoURL: ptr(clean)}
	if err := o.BeforeSave(nil); err != nil {
		t.Fatal(err)
	}
	if *o.InfoURL != clean {
		t.Fatalf("clean URL was modified:\n got %s\nwant %s", *o.InfoURL, clean)
	}
}
