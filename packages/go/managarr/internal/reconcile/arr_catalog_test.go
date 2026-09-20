package reconcile

import (
	"testing"
	"time"

	"homelab/managarr/internal/model"
)

func TestSeasonTitle(t *testing.T) {
	// Zero is Sonarr's specials bucket, not a season. Rendering it as
	// "Season 0" would put a row in the catalog that names nothing real.
	if got := seasonTitle(0); got != "Specials" {
		t.Errorf("seasonTitle(0) = %q", got)
	}
	if got := seasonTitle(3); got != "Season 3" {
		t.Errorf("seasonTitle(3) = %q", got)
	}
}

func TestMinutesToMs(t *testing.T) {
	// Servarr runtimes are minutes everywhere except Lidarr's track duration,
	// which is already milliseconds and does not go through here.
	got := minutesToMs(42)
	if got == nil || *got != 42*60*1000 {
		t.Fatalf("minutesToMs(42) = %v, want 2520000", got)
	}
	if minutesToMs(0) != nil {
		t.Error("an unreported runtime should be NULL, not 0")
	}
}

func TestAirYear(t *testing.T) {
	if airYear(nil) != nil {
		t.Error("nil date should give nil year")
	}
	var zero time.Time
	if airYear(&zero) != nil {
		t.Error("zero date should give nil year")
	}
	d := time.Date(1999, 12, 31, 23, 0, 0, 0, time.UTC)
	if got := airYear(&d); got == nil || *got != 1999 {
		t.Errorf("airYear = %v, want 1999", got)
	}
}

func TestNumID(t *testing.T) {
	// Zero is "this app does not carry that id", not the id zero.
	if got := numID(0); got != "" {
		t.Errorf("numID(0) = %q", got)
	}
	if got := numID(81189); got != "81189" {
		t.Errorf("numID(81189) = %q", got)
	}
}

func TestSharedIDsDropsAbsentValues(t *testing.T) {
	id := model.NewID()
	rows := sharedIDs(id, map[string]string{
		model.ProviderTMDB: "603",
		model.ProviderIMDB: "",
		model.ProviderTVDB: "0",
	})
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(rows), rows)
	}
	if rows[0].Provider != model.ProviderTMDB || rows[0].Value != "603" {
		t.Errorf("row = %+v", rows[0])
	}
	if rows[0].ContentID != id {
		t.Error("content id not carried through")
	}
}

// TestSharedIDsRejectsZeroAsAString is the subtle half of the above.
//
// numID renders an absent numeric id as "", but Radarr sends imdbId as a string
// and an artist with no MusicBrainz id sends "". A literal "0" reaches here from
// payloads that stringify the number, and storing it would make every
// unidentified item collide on the same external id.
func TestSharedIDsRejectsZeroAsAString(t *testing.T) {
	if rows := sharedIDs(model.NewID(), map[string]string{"tvdb": "0"}); len(rows) != 0 {
		t.Errorf(`"0" was stored: %+v`, rows)
	}
}

func TestGenreRowsSkipsEmpties(t *testing.T) {
	id := model.NewID()
	rows := genreRows(id, []string{"Drama", "", "Crime"})
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	for _, r := range rows {
		if r.ContentID != id || r.Genre == "" {
			t.Errorf("bad row %+v", r)
		}
	}
	if genreRows(id, nil) == nil {
		t.Error("nil genres should give an empty slice, not nil, so the caller can append")
	}
}

// TestArrIndexContentForPrefersTheLeaf pins the resolution order.
//
// A Sonarr history record names BOTH its series and its episode. The episode is
// the answer: attributing an import to the series would make every file in the
// library point at the show rather than at what it actually holds.
func TestArrIndexContentForPrefersTheLeaf(t *testing.T) {
	ix := newArrIndex()
	series := model.NewID()
	episode := model.NewID()
	ix.parent[10] = series
	ix.child[99] = episode

	got := ix.contentFor(99, 0, 10, 0, 0)
	if got == nil || *got != episode {
		t.Fatalf("got %v, want the episode %s", got, episode)
	}

	// With no episode, the series is the best available answer.
	got = ix.contentFor(0, 0, 10, 0, 0)
	if got == nil || *got != series {
		t.Fatalf("got %v, want the series %s", got, series)
	}
}

// TestArrIndexContentForUnknownIsNil covers old history.
//
// A record naming a series that has since been deleted resolves to nothing, and
// nothing is the correct answer — the alternative is a dangling foreign key.
func TestArrIndexContentForUnknownIsNil(t *testing.T) {
	ix := newArrIndex()
	if got := ix.contentFor(1, 2, 3, 4, 5); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if got := ix.contentFor(0, 0, 0, 0, 0); got != nil {
		t.Errorf("all-zero ids gave %v, want nil", got)
	}
}

// TestArrIndexContentForAlbumBeatsArtist is the music half of the same rule.
func TestArrIndexContentForAlbumBeatsArtist(t *testing.T) {
	ix := newArrIndex()
	artist := model.NewID()
	album := model.NewID()
	ix.parent[7] = artist
	ix.child[70] = album

	got := ix.contentFor(0, 70, 0, 0, 7)
	if got == nil || *got != album {
		t.Fatalf("got %v, want the album %s", got, album)
	}
}
