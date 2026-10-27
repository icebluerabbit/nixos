package bazarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	"homelab/httpx"
)

// newTestClient wires a client to a stub server.
func newTestClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	// Retries disabled: a test that exercises an error path should fail once,
	// not four times with a backoff.
	c, err := New(httpx.Config{BaseURL: srv.URL, RetryMax: -1}, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestAuthHeaderName is the whole reason this client exists separately.
//
// Bazarr authenticates with X-API-KEY. The Servarr family uses X-Api-Key —
// different capitalisation, and Bazarr rejects the wrong one with a 401 whose
// body talks about browsers and credentials rather than about the header.
func TestAuthHeaderName(t *testing.T) {
	var got http.Header
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(`{"data":{"bazarr_version":"1.4.3"}}`))
	}))

	if _, err := c.Version(context.Background()); err != nil {
		t.Fatal(err)
	}
	if v := got.Get("X-API-KEY"); v != "test-key" {
		t.Errorf("X-API-KEY = %q, want test-key", v)
	}
}

func TestVersion(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/system/status" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"bazarr_version":"1.4.3","operating_system":"Linux"}}`))
	}))

	got, err := c.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.4.3" {
		t.Errorf("version = %q", got)
	}
}

// TestMoviesAsksForEverything pins the paging argument.
//
// Unlike the Servarr family, Bazarr honours a negative length as "no limit". A
// wrong value here silently returns the first page and the subtitle table ends
// up holding a fraction of the library with nothing to indicate it.
func TestMoviesAsksForEverything(t *testing.T) {
	var q url.Values
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		_, _ = w.Write([]byte(`{"data":[{
		  "radarrId": 42,
		  "title": "Some Film",
		  "path": "/media/films/Some Film/Some Film.mkv",
		  "subtitles": [
		    {"name":"English","code2":"en","code3":"eng","path":"/media/films/Some Film/Some Film.en.srt","forced":false,"hi":false,"file_size":41234},
		    {"name":"English","code2":"en","code3":"eng","path":null,"forced":false,"hi":true}
		  ],
		  "missing_subtitles": [{"name":"Spanish","code2":"es","code3":"spa","forced":false,"hi":false}]
		}],"total":1}`))
	}))

	movies, err := c.Movies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("length") != "-1" {
		t.Errorf("length = %q, want -1", q.Get("length"))
	}
	if len(movies) != 1 {
		t.Fatalf("got %d movies", len(movies))
	}
	m := movies[0]
	if m.RadarrID != 42 {
		t.Errorf("radarrId = %d; this is the join key to Radarr's catalog", m.RadarrID)
	}
	if len(m.Subtitles) != 2 {
		t.Fatalf("got %d subtitles", len(m.Subtitles))
	}
	// The second track has a null path: it is embedded in the video, not a
	// file. The client must surface that distinction rather than hide it.
	if m.Subtitles[0].Path == "" {
		t.Error("external subtitle lost its path")
	}
	if m.Subtitles[1].Path != "" {
		t.Error("embedded subtitle should have an empty path")
	}
	if len(m.MissingSubtitles) != 1 || m.MissingSubtitles[0].Code3 != "spa" {
		t.Errorf("missing = %+v", m.MissingSubtitles)
	}
}

// TestEpisodesBatchesSeriesIDs checks the repeated query parameter.
//
// The endpoint takes seriesid[] repeated and — unlike the Servarr file
// collections, which honour the first and silently ignore the rest — actually
// respects all of them. Batching is safe here and a request per series is not
// worth a thousand round trips.
func TestEpisodesBatchesSeriesIDs(t *testing.T) {
	var batches [][]string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batches = append(batches, r.URL.Query()["seriesid[]"])
		_, _ = w.Write([]byte(`{"data":[],"total":0}`))
	}))

	ids := make([]int64, episodeBatch+5)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	if _, err := c.Episodes(context.Background(), ids); err != nil {
		t.Fatal(err)
	}

	if len(batches) != 2 {
		t.Fatalf("made %d requests, want 2", len(batches))
	}
	if len(batches[0]) != episodeBatch {
		t.Errorf("first batch had %d ids, want %d", len(batches[0]), episodeBatch)
	}
	if len(batches[1]) != 5 {
		t.Errorf("second batch had %d ids, want 5", len(batches[1]))
	}

	var all []string
	for _, b := range batches {
		all = append(all, b...)
	}
	if len(all) != len(ids) {
		t.Errorf("sent %d ids, want %d", len(all), len(ids))
	}
	if !slices.Contains(all, "1") || !slices.Contains(all, "55") {
		t.Error("ids were dropped between batches")
	}
}

func TestEpisodesEmptyInputMakesNoRequest(t *testing.T) {
	var calls int
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	got, err := c.Episodes(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Errorf("made %d requests for no series", calls)
	}
	if len(got) != 0 {
		t.Errorf("got %d episodes", len(got))
	}
}

// TestHistoryDecodesTheNestedLanguage covers the shape that differs from the
// listing.
//
// A subtitle listing carries a flat code2/code3; a history record carries a
// language OBJECT. Reusing one struct for both would silently leave every
// history language empty.
func TestHistoryDecodesTheNestedLanguage(t *testing.T) {
	var path string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[{
		  "id": 9,
		  "action": 1,
		  "sonarrSeriesId": 12,
		  "sonarrEpisodeId": 345,
		  "provider": "opensubtitles",
		  "subs_id": "sub-123",
		  "score": "98.5",
		  "language": {"name":"English","code2":"en","code3":"eng","forced":false,"hi":false},
		  "matches": ["series","season","episode"],
		  "dont_matches": ["release_group"],
		  "subtitles_path": "/media/tv/Show/S01E01.en.srt",
		  "video_path": "/media/tv/Show/S01E01.mkv",
		  "parsed_timestamp": "09/13/26 14:05:06",
		  "upgradable": true,
		  "blacklisted": false
		}],"total":1}`))
	}))

	recs, err := c.History(context.Background(), "series")
	if err != nil {
		t.Fatal(err)
	}
	// The collection comes first and the resource second, and Bazarr names the
	// TV collection after the episode. This assertion used to read
	// "/api/history/series", which is not a route Bazarr has — it pinned the
	// bug instead of the API, and the pass failed against the real Bazarr for
	// as long as it existed.
	//
	// Nothing louder happens when it is wrong: Bazarr serves its single-page
	// app on any path it does not recognise, so the wrong route returns 200
	// with HTML and fails later as a JSON decode error naming no route at all.
	if path != "/api/episodes/history" {
		t.Errorf("path = %q, want /api/episodes/history", path)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records", len(recs))
	}
	r := recs[0]
	if r.Action != ActionDownload {
		t.Errorf("action = %d, want %d", r.Action, ActionDownload)
	}
	if r.Language.Code3 != "eng" {
		t.Errorf("language = %+v", r.Language)
	}
	if r.SonarrEpisodeID != 345 {
		t.Errorf("episode id = %d; this is the join key to Sonarr", r.SonarrEpisodeID)
	}
	if len(r.Matches) != 3 || len(r.DontMatches) != 1 {
		t.Errorf("matches = %v / %v", r.Matches, r.DontMatches)
	}
}

// TestActionConstantsMatchUpstream pins the enum.
//
// Only a download produces a file. Treating a deletion as one would attach a
// provider and a score to a subtitle that is gone.
func TestActionConstantsMatchUpstream(t *testing.T) {
	want := map[int]string{
		0: "deleted", 1: "download", 2: "manual", 3: "upgrade", 4: "sync",
	}
	got := map[int]string{
		ActionDeleted: "deleted", ActionDownload: "download",
		ActionManual: "manual", ActionUpgrade: "upgrade", ActionSync: "sync",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("action %d = %q, want %q", k, got[k], v)
		}
	}
}

func TestBlacklistDecodes(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/movies/blacklist" {
			t.Errorf("path = %q, want /api/movies/blacklist", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{
		  "radarrId": 42,
		  "provider": "opensubtitles",
		  "subs_id": "sub-9",
		  "subtitles_path": "/media/films/Some Film/Some Film.en.srt",
		  "language": {"name":"English","code2":"en","code3":"eng"}
		}]}`))
	}))

	got, err := c.Blacklist(context.Background(), "movies")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].SubtitlesPath == "" {
		t.Fatalf("got %+v", got)
	}
}

// TestServerErrorIsAnError keeps a 500 from reading as an empty library.
//
// Silently returning no subtitles would make the pass wipe nothing (it only
// upserts) but report success — and "Bazarr is down" would look identical to
// "nothing has subtitles".
func TestServerErrorIsAnError(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	if _, err := c.Movies(context.Background()); err == nil {
		t.Fatal("a 500 did not produce an error")
	}
}

// TestMalformedJSONIsAnError is the other failure that must not look like
// success.
func TestMalformedJSONIsAnError(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":`))
	}))
	if _, err := c.SeriesList(context.Background()); err == nil {
		t.Fatal("truncated JSON did not produce an error")
	}
}

// TestEnvelopeRoundTrip is a guard on the generic wrapper itself.
func TestEnvelopeRoundTrip(t *testing.T) {
	var e envelope[Series]
	if err := json.Unmarshal([]byte(`{"data":[{"sonarrSeriesId":1,"title":"x"}],"total":1}`), &e); err != nil {
		t.Fatal(err)
	}
	if e.Total != 1 || len(e.Data) != 1 || e.Data[0].SonarrSeriesID != 1 {
		t.Errorf("envelope = %+v", e)
	}
}

// An unknown collection is refused rather than sent.
//
// Bazarr answers any unrecognised path with its single-page app — 200, HTML —
// so a typo'd collection would reach the server, come back looking successful,
// and fail as a decode error that names no route. Refusing here turns that into
// an error that says what is actually wrong.
func TestUnknownCollectionIsRefused(t *testing.T) {
	reached := false
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		reached = true
	}))

	if _, err := c.History(context.Background(), "books"); err == nil {
		t.Error("History accepted an unknown collection")
	}
	if _, err := c.Blacklist(context.Background(), "books"); err == nil {
		t.Error("Blacklist accepted an unknown collection")
	}
	if reached {
		t.Error("a request was sent for an unknown collection")
	}
}

// "series" and "episodes" both address the TV half. Bazarr's route says
// episodes; the rest of this stack says series, and callers use either.
func TestSeriesAndEpisodesAddressTheSameCollection(t *testing.T) {
	var paths []string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[],"total":0}`))
	}))

	for _, kind := range []string{"series", "episodes"} {
		if _, err := c.History(context.Background(), kind); err != nil {
			t.Fatal(err)
		}
	}
	if len(paths) != 2 || paths[0] != paths[1] {
		t.Errorf("paths = %v, want both /api/episodes/history", paths)
	}
}
