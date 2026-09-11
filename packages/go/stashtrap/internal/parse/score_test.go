package parse_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode"

	"homelab/stashtrap/internal/parse"
)

// fake is a catalogue held in maps, keyed the way the store keys its alias
// columns. Matching is about what the catalogue recognises, so a test of it
// needs a catalogue, not a database.
type fake struct {
	// entries maps a normalised probe to what answers to it.
	entries map[string][]parse.Hit
	// scenes maps a scene id to its relations.
	scenes map[string]parse.Scene
	err    error
}

func (f fake) Lookup(_ context.Context, probes []string) ([]parse.Hit, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []parse.Hit
	for _, p := range probes {
		for _, h := range f.entries[p] {
			h.Probe = p
			if h.Shared == 0 {
				h.Shared = 1
			}
			if h.Source == "" {
				h.Source = parse.SourceAlias
			}
			out = append(out, h)
		}
	}
	return out, nil
}

func (f fake) Scenes(_ context.Context, ids []string) ([]parse.Scene, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []parse.Scene
	for _, id := range ids {
		if s, ok := f.scenes[id]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f fake) ScenesOn(_ context.Context, date string, studioIDs, performerIDs []string) ([]parse.Scene, error) {
	if f.err != nil {
		return nil, f.err
	}
	iso := parse.ISODate(date)
	if iso == "" {
		return nil, nil
	}
	var out []parse.Scene
	for _, s := range f.scenes {
		if s.ReleaseDate != iso {
			continue
		}
		if contains(studioIDs, s.StudioID) || overlaps(performerIDs, s.Performers) {
			out = append(out, s)
		}
	}
	return out, nil
}

// SearchScenes stands in for the trigram index: a scene answers when its title
// shares a word with the text, scored by how many words it shares.
func (f fake) SearchScenes(_ context.Context, text string, limit int) ([]parse.SceneMatch, error) {
	if f.err != nil {
		return nil, f.err
	}
	words := strings.Fields(strings.ToLower(text))
	var out []parse.SceneMatch
	for id, s := range f.scenes {
		shared := 0.0
		for _, w := range words {
			if len(w) >= 3 && strings.Contains(strings.ToLower(s.Title), w) {
				shared++
			}
		}
		if shared > 0 {
			out = append(out, parse.SceneMatch{SceneID: id, Score: shared})
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func overlaps(a, b []string) bool {
	for _, x := range a {
		if contains(b, x) {
			return true
		}
	}
	return false
}

// norm mirrors store.Normalise, which parse deliberately does not import.
func norm(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func identify(t *testing.T, f fake, name string) (parse.Result, bool) {
	t.Helper()

	res, ok, err := parse.Identify(context.Background(), f, norm, name)
	if err != nil {
		t.Fatalf("Identify(%q): %v", name, err)
	}
	return res, ok
}

// performer builds a one-entity performer hit.
func performer(id string, shared int) parse.Hit {
	return parse.Hit{Kind: parse.KindPerformer, ID: id, Shared: shared}
}

func studio(id string) parse.Hit {
	return parse.Hit{Kind: parse.KindStudio, ID: id, Shared: 1}
}

func TestIdentifyRecognisesHeaderAndBody(t *testing.T) {
	t.Parallel()

	f := fake{entries: map[string][]parse.Hit{
		"firstsite": {studio("s1")},
		"janedoe":   {performer("p1", 1)},
	}}

	res, ok := identify(t, f, "[FirstSite] Jane Doe - A Title 1080p")
	if !ok {
		t.Fatal("no match")
	}
	if len(res.PerformerIDs) != 1 || res.PerformerIDs[0] != "p1" {
		t.Fatalf("performers = %v, want [p1]", res.PerformerIDs)
	}
	if len(res.StudioIDs) != 1 || res.StudioIDs[0] != "s1" {
		t.Fatalf("studios = %v, want [s1]", res.StudioIDs)
	}
}

// A header is not a studio field: the same position carries a performer on
// plenty of releases, and nothing may assume otherwise.
func TestIdentifyFindsAPerformerInTheHeader(t *testing.T) {
	t.Parallel()

	f := fake{entries: map[string][]parse.Hit{
		"manyvids":   {studio("s1")},
		"sweetiefox": {performer("p1", 1)},
	}}

	res, ok := identify(t, f, "[Manyvids: Sweetie Fox] A Title")
	if !ok {
		t.Fatal("no match")
	}
	if len(res.PerformerIDs) != 1 || res.PerformerIDs[0] != "p1" {
		t.Fatalf("performers = %v, want the performer named in the header", res.PerformerIDs)
	}
	if len(res.StudioIDs) != 1 {
		t.Fatalf("studios = %v, want the platform too", res.StudioIDs)
	}
}

// Delimited casts resolve from the chunks the delimiters produced, whatever
// the delimiter was.
func TestIdentifySplitsCastsHoweverWritten(t *testing.T) {
	t.Parallel()

	f := fake{entries: map[string][]parse.Hit{
		"firstsite":       {studio("s1")},
		"janedoe":         {performer("p1", 1)},
		"secondperformer": {performer("p2", 1)},
	}}

	for _, name := range []string{
		"[FirstSite] Jane Doe, Second Performer - A Title",
		"[FirstSite] Jane Doe & Second Performer - A Title",
		"[FirstSite] Jane Doe and Second Performer - A Title",
	} {
		res, ok := identify(t, f, name)
		if !ok {
			t.Fatalf("%q: no match", name)
		}
		if len(res.PerformerIDs) != 2 {
			t.Fatalf("%q: performers = %v, want both", name, res.PerformerIDs)
		}
	}
}

// Undelimited bodies are the case chunks cannot answer, so word runs inside an
// unresolved chunk are the fallback.
func TestIdentifyFallsBackToWordRuns(t *testing.T) {
	t.Parallel()

	f := fake{entries: map[string][]parse.Hit{
		"metartx":     {studio("s1")},
		"amberslassh": {performer("p1", 1)},
	}}

	res, ok := identify(t, f, "[MetArtX] 26.09.09 Amber Slassh Pink Confession 2 [1080p]")
	if !ok {
		t.Fatal("no match")
	}
	if len(res.PerformerIDs) != 1 || res.PerformerIDs[0] != "p1" {
		t.Fatalf("performers = %v, want the name inside the undivided body", res.PerformerIDs)
	}
	if res.Date != "26.09.09" {
		t.Fatalf("date = %q", res.Date)
	}
}

// A widely shared alias is a first name and must not outweigh a real one.
func TestIdentifyDiscountsSharedAliases(t *testing.T) {
	t.Parallel()

	shared := fake{entries: map[string][]parse.Hit{
		"firstsite": {studio("s1")},
		"janedoe":   {performer("p1", 40)},
	}}
	unique := fake{entries: map[string][]parse.Hit{
		"firstsite": {studio("s1")},
		"janedoe":   {performer("p1", 1)},
	}}

	loose, _ := identify(t, shared, "[FirstSite] Jane Doe - A Title")
	tight, _ := identify(t, unique, "[FirstSite] Jane Doe - A Title")
	if loose.Score >= tight.Score {
		t.Fatalf("shared alias scored %.2f, unique scored %.2f; shared must be worth less",
			loose.Score, tight.Score)
	}
}

// The catalogue's own relations decide whether the parts of a reading agree.
func TestIdentifyCorroboratesThroughTheScene(t *testing.T) {
	t.Parallel()

	f := fake{
		entries: map[string][]parse.Hit{
			"firstsite": {studio("s1")},
			"janedoe":   {performer("p1", 1)},
			"atitle":    {{Kind: parse.KindScene, ID: "sc1", Shared: 1, Source: parse.SourceSceneTitle}},
		},
		scenes: map[string]parse.Scene{
			"sc1": {ID: "sc1", StudioID: "s1", ReleaseDate: "2026-09-09", Performers: []string{"p1"}},
		},
	}

	res, ok := identify(t, f, "[FirstSite] Jane Doe - A Title (2026-09-09)")
	if !ok {
		t.Fatal("no match")
	}
	if res.SceneID != "sc1" {
		t.Fatalf("scene = %q, want sc1", res.SceneID)
	}
	if !res.Corroborated {
		t.Fatalf("reading not corroborated: %+v", res)
	}
}

// A first name is a candidate, not an identification: believed only when the
// scene the release was identified as actually credits that person.
func TestIdentifyDropsUncorroboratedMononyms(t *testing.T) {
	t.Parallel()

	f := fake{entries: map[string][]parse.Hit{
		"firstsite": {studio("s1")},
		"kendra":    {performer("p1", 1)},
	}}

	res, _ := identify(t, f, "[FirstSite] A Title: Kendra And Alina")
	if len(res.PerformerIDs) != 0 {
		t.Fatalf("performers = %v, want none: nothing corroborates a first name", res.PerformerIDs)
	}
	if len(res.Dropped) != 1 {
		t.Fatalf("dropped = %+v, want the mononym recorded", res.Dropped)
	}

	// The same name is believed when the identified scene credits them.
	corroborated := fake{
		entries: map[string][]parse.Hit{
			"firstsite": {studio("s1")},
			"kendra":    {performer("p1", 1)},
		},
		scenes: map[string]parse.Scene{
			"sc1": {ID: "sc1", StudioID: "s1", ReleaseDate: "2026-09-04", Performers: []string{"p1"}},
		},
	}
	res, ok := identify(t, corroborated, "[FirstSite] 26.09.04 A Title: Kendra")
	if !ok || len(res.PerformerIDs) != 1 || res.PerformerIDs[0] != "p1" {
		t.Fatalf("performers = %v (ok=%v), want [p1] once the scene credits them", res.PerformerIDs, ok)
	}
}

// One word matching both a performer and a short scene title is a single fact
// agreeing with itself — which is how a 2026 Vixen release was identified as a
// 2013 scene called "Alina" starring Alina Li.
func TestIdentifyRejectsCircularEvidence(t *testing.T) {
	t.Parallel()

	f := fake{
		entries: map[string][]parse.Hit{
			"vixen": {studio("s1")},
			"alina": {
				performer("wrong-alina", 1),
				{Kind: parse.KindScene, ID: "sc-old", Shared: 1, Source: parse.SourceSceneTitle},
			},
		},
		scenes: map[string]parse.Scene{
			"sc-old": {
				ID: "sc-old", Title: "Alina", StudioID: "other",
				ReleaseDate: "2013-12-14", Performers: []string{"wrong-alina"},
			},
		},
	}

	res, _ := identify(t, f, "[Vixen] The Angel Anthology: Kendra And Alina (04/09/2026)")
	if res.SceneID == "sc-old" {
		t.Fatalf("identified a 2013 scene for a 2026 release: %+v", res)
	}
	if contains(res.PerformerIDs, "wrong-alina") {
		t.Fatalf("performers = %v, want no one: a first name backed only by itself", res.PerformerIDs)
	}
}

// A release naming nobody is still identifiable from its date and its site.
func TestIdentifyFindsSceneByDateAndStudio(t *testing.T) {
	t.Parallel()

	f := fake{
		entries: map[string][]parse.Hit{"firstsite": {studio("s1")}},
		scenes: map[string]parse.Scene{
			"sc1": {ID: "sc1", StudioID: "s1", ReleaseDate: "2026-09-09", Performers: []string{"p1", "p2"}},
		},
	}

	res, ok := identify(t, f, "[FirstSite] 26.09.09 Some Untitled Thing")
	if !ok {
		t.Fatal("no match")
	}
	if res.SceneID != "sc1" {
		t.Fatalf("scene = %q, want the scene that site released that day", res.SceneID)
	}
	if len(res.PerformerIDs) != 2 {
		t.Fatalf("performers = %v, want the scene's cast", res.PerformerIDs)
	}
}

func TestIdentifyRejectsNamesItCannotRead(t *testing.T) {
	t.Parallel()

	if res, ok := identify(t, fake{}, "[Unknown Site] Nobody In Particular"); ok {
		t.Fatalf("unresolved name matched: %+v", res)
	}
}

func TestIdentifyPropagatesErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("catalogue unavailable")
	if _, _, err := parse.Identify(context.Background(), fake{err: want}, norm, "[FirstSite] Jane Doe"); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestISODate(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"26.09.09":       "2026-09-09",
		"2019-10-18":     "2019-10-18",
		"28/08/2026":     "2026-08-28",
		"Aug 14, 2026":   "2026-08-14",
		"August 14 2026": "2026-08-14",
		"not a date":     "",
	}
	for in, want := range tests {
		if got := parse.ISODate(in); got != want {
			t.Fatalf("ISODate(%q) = %q, want %q", in, got, want)
		}
	}
}
