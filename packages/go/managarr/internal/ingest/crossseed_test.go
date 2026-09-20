package ingest

import (
	"testing"

	"homelab/managarr/internal/model"
)

// TestCrossSeedOutcome maps an injection RESULT, which is a different
// vocabulary from the SQLite decision log.
//
// FAILURE is errored rather than rejected: cross-seed wanted the candidate and
// could not act on it. Calling that a rejection would make a broken qBittorrent
// connection read as a judgement about the release.
func TestCrossSeedOutcome(t *testing.T) {
	cases := map[string]string{
		"INJECTED": model.OutcomeAccepted,
		"SAVED":    model.OutcomeAccepted,
		"injected": model.OutcomeAccepted,
		"FAILURE":  model.OutcomeErrored,
		"failure":  model.OutcomeErrored,
		// The webhook fires only on a match, so anything unrecognised is still
		// a match that was acted on.
		"":        model.OutcomeAccepted,
		"UNKNOWN": model.OutcomeAccepted,
	}
	for result, want := range cases {
		if got := crossSeedOutcome(result); got != want {
			t.Errorf("crossSeedOutcome(%q) = %q, want %q", result, got, want)
		}
	}
}

// TestCrossSeedHookDecodes pins the nested shape.
//
// Everything machine-readable lives under `extra`; title and body are prose for
// a chat sink. Decoding the top level only would give an empty handler that
// silently recorded nothing.
func TestCrossSeedHookDecodes(t *testing.T) {
	const body = `{
	  "title": "Injected",
	  "body": "Some.Release matched",
	  "extra": {
	    "event": "RESULTS",
	    "name": "Some.Release-GRP",
	    "infoHash": "0123456789ABCDEF0123456789ABCDEF01234567",
	    "result": "INJECTED",
	    "decision": "MATCH",
	    "trackers": ["torrentleech"],
	    "source": "Some.Other.Release",
	    "searchee": {"name": "Some.Other.Release"}
	  }
	}`
	var h crossSeedHook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if h.Extra.InfoHash != "0123456789ABCDEF0123456789ABCDEF01234567" {
		t.Errorf("infohash = %q", h.Extra.InfoHash)
	}
	if h.Extra.Searchee.Name != "Some.Other.Release" {
		t.Errorf("searchee = %q", h.Extra.Searchee.Name)
	}
	if h.Extra.Result != "INJECTED" {
		t.Errorf("result = %q", h.Extra.Result)
	}
}

// TestCrossSeedHookFallsBackToSource covers older builds.
//
// The searchee is half the offer's key. Without it the candidate cannot be tied
// to what it was matched against, and the row would collide with every other
// candidate for the same hash.
func TestCrossSeedHookFallsBackToSource(t *testing.T) {
	const body = `{"extra":{"infoHash":"abc","source":"Some.Other.Release"}}`
	var h crossSeedHook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if h.Extra.Searchee.Name != "" {
		t.Fatal("this fixture should have no searchee object")
	}
	if h.Extra.Source != "Some.Other.Release" {
		t.Errorf("source = %q", h.Extra.Source)
	}
}

// The pair, from the payload cross-seed actually sends.
//
// Captured from a live notification rather than invented. extra.searchee
// carries the SOURCE infohash and extra.infoHashes the candidates, both full
// length — this is the only place in the whole stack that states which torrent
// went in and which came out.
//
// An earlier version of this struct read extra.searchee.name and nothing else,
// so the source hash arrived in every notification and was discarded. managarr
// then resolved the source by NAME, refused it whenever one name covered
// several torrents, and ended up recording a source for barely a quarter of
// cross-seed candidates.
func TestCrossSeedHookCarriesTheSourceInfohash(t *testing.T) {
	const body = `{
	  "title": "cross-seed",
	  "body": "announce: Injected its.always.sunny.s18e06.mkv on 1 tracker by MATCH from torrentClient: YU-Scene",
	  "extra": {
	    "event": "RESULTS",
	    "name": "its.always.sunny.s18e06.mkv",
	    "infoHashes": ["43bd09a62222222222222222222222222222bbbb"],
	    "trackers": ["YU-Scene"],
	    "source": "announce",
	    "result": "INJECTED",
	    "paused": false,
	    "decisions": ["MATCH"],
	    "searchee": {
	      "category": "cross-seed-link",
	      "tags": ["cross-seed"],
	      "trackers": ["darkpeers.org"],
	      "length": 1145793440,
	      "clientHost": "10.10.130.5:8080",
	      "infoHash": "478e234c1111111111111111111111111111aaaa",
	      "path": null,
	      "source": "torrentClient"
	    }
	  }
	}`

	var h crossSeedHook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}

	if h.Extra.Searchee.InfoHash != "478e234c1111111111111111111111111111aaaa" {
		t.Errorf("source infohash = %q; it is in the payload", h.Extra.Searchee.InfoHash)
	}
	if h.Extra.Searchee.Source != "torrentClient" {
		t.Errorf("searchee source = %q", h.Extra.Searchee.Source)
	}
	if got := h.candidates(); len(got) != 1 ||
		got[0] != "43bd09a62222222222222222222222222222bbbb" {
		t.Errorf("candidates = %v", got)
	}
	// A null path decodes rather than failing the whole notification.
	if h.Extra.Searchee.Path != nil {
		t.Errorf("path = %v, want nil", *h.Extra.Searchee.Path)
	}
}

// One announce matching on several trackers is one searchee and several
// candidates, and every one has to be recorded — they are different torrents
// on different trackers.
func TestCrossSeedCandidatesCoversEveryHash(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			"plural only",
			`{"extra":{"infoHashes":["aaaa","bbbb","cccc"]}}`,
			3,
		},
		{
			// Older builds send the singular field.
			"singular only",
			`{"extra":{"infoHash":"aaaa"}}`,
			1,
		},
		{
			// Both, with the singular repeated in the list: one candidate, not
			// two, or the offer is written twice.
			"both, overlapping",
			`{"extra":{"infoHash":"aaaa","infoHashes":["aaaa","bbbb"]}}`,
			2,
		},
		{"neither", `{"extra":{"name":"x"}}`, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var h crossSeedHook
			if err := decode([]byte(tc.body), &h); err != nil {
				t.Fatal(err)
			}
			if got := h.candidates(); len(got) != tc.want {
				t.Errorf("candidates = %v, want %d", got, tc.want)
			}
		})
	}
}

// Hashes are lowercased on the way in. cross-seed sends them lowercase and the
// *arrs send them upper, and both end up in the same column.
func TestCrossSeedCandidatesAreLowercased(t *testing.T) {
	var h crossSeedHook
	if err := decode([]byte(`{"extra":{"infoHashes":["ABCDEF12"]}}`), &h); err != nil {
		t.Fatal(err)
	}
	if got := h.candidates(); len(got) != 1 || got[0] != "abcdef12" {
		t.Errorf("candidates = %v", got)
	}
}

// A dataDir match ran against a path, not a torrent. There is genuinely no
// source infohash — that is an absence to record faithfully, not a lookup to
// retry by some other means.
func TestCrossSeedDataDirMatchHasNoSourceHash(t *testing.T) {
	const body = `{"extra":{
	  "name":"Some.Release-GRP",
	  "infoHashes":["6902156f0000000000000000000000000000dddd"],
	  "result":"INJECTED",
	  "searchee":{"name":"Some.Release-GRP","source":"dataDir",
	              "path":"/mnt/media/complete/tv/Some.Release-GRP"}}}`

	var h crossSeedHook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if h.Extra.Searchee.InfoHash != "" {
		t.Errorf("source = %q; a dataDir match has no source torrent", h.Extra.Searchee.InfoHash)
	}
	if h.Extra.Searchee.Path == nil || *h.Extra.Searchee.Path == "" {
		t.Error("the path it did match against was lost")
	}
}
