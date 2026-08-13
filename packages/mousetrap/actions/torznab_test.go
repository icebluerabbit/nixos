package actions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"mousetrap/app"
	"mousetrap/store"
	"mousetrap/types"
)

// healthyUser is a self view well under the unsat ceiling (5/20) with wedges.
const healthyUser = `{"classname":"User","ratio":"10","uploaded":"2.00 TiB","downloaded":"1.00 TiB","wedges":5,"unsat":{"count":5,"limit":20}}`

// ceilingUser sits at the unsat ceiling so the grab-gate holds.
const ceilingUser = `{"classname":"User","ratio":"10","uploaded":"2.00 TiB","downloaded":"1.00 TiB","wedges":5,"unsat":{"count":20,"limit":20}}`

// twoResults is a search page with one ebook (mediatype 2) and one audiobook (1).
const twoResults = `{"total":2,"total_found":2,"data":[
	{"id":42,"title":"An Ebook","catname":"Ebooks - Fantasy","main_cat":14,"mediatype":2,"filetype":"epub","lang_code":"English","size":"6.0 MiB","seeders":10,"leechers":2,"numfiles":1,"times_completed":5,"free":0,"vip":0},
	{"id":43,"title":"An Audiobook","catname":"Audiobooks - Horror","main_cat":13,"mediatype":1,"filetype":"m4b","lang_code":"English","size":"200.0 MiB","seeders":8,"leechers":1,"numfiles":3,"times_completed":9,"free":1,"vip":0}
]}`

// searchMediaTypes is the id→name table the search path needs to speak the same
// vocabulary as the announce. Deliberately partial: id 9 is unmapped, as MAM's
// unnamed ids are in production.
var searchMediaTypes = map[string]string{"1": "Audiobook", "2": "Ebook"}

// torznabApp builds an app whose MAM mock routes jsonLoad → userJSON, search →
// searchJSON, and download → fixed .torrent bytes.
func torznabApp(t *testing.T, accept types.AcceptConfig, grab types.GrabConfig, tz types.TorznabConfig, userJSON, searchJSON string) *app.App {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonLoad.php", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(userJSON))
	})
	mux.HandleFunc("/tor/js/loadSearchJSONbasic.php", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(searchJSON))
	})
	mux.HandleFunc("/tor/download.php", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("TORRENTBYTES"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	seed := filepath.Join(t.TempDir(), "seed")
	if err := os.WriteFile(seed, []byte("seed"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := app.New(types.Config{
		StateDir: t.TempDir(),
		MAM:      types.MAMConfig{APIBase: srv.URL, SeedPath: seed, MediaTypeNames: searchMediaTypes},
		Accept:   accept,
		Grab:     grab,
		Torznab:  tz,
	})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// get calls a handler with a GET request at the given raw query.
func get(h http.HandlerFunc, rawQuery string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/x?"+rawQuery, nil)
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func TestTorznabCaps(t *testing.T) {
	a := torznabApp(t, types.AcceptConfig{}, types.GrabConfig{}, types.TorznabConfig{}, healthyUser, "")
	w := get(Torznab(a), "t=caps")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`<subcat id="3030" name="Audiobook">`,
		`<subcat id="7020" name="Ebook">`,
		`<book-search available="yes"`,
		`<category id="3000" name="Audio">`,
		`<category id="7000" name="Books">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("caps missing %q\n%s", want, body)
		}
	}
}

func TestTorznabSearchFeed(t *testing.T) {
	a := torznabApp(t, types.AcceptConfig{}, types.GrabConfig{},
		types.TorznabConfig{BaseURL: "http://mt:8080", LinkTTL: time.Hour}, healthyUser, twoResults)
	w := get(Torznab(a), "t=search&q=book")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	checks := []string{
		`xmlns:torznab="http://torznab.com/schemas/2015/feed"`,
		`name="category" value="7020"`, // ebook
		`name="category" value="3030"`, // audiobook
		`http://mt:8080/torznab/download?`,
		`id=42`,
		`type="application/x-bittorrent"`,
	}
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Errorf("search feed missing %q\n%s", want, body)
		}
	}
}

func TestTorznabSearchLayer1Filter(t *testing.T) {
	// only ebooks allowed → the audiobook result is filtered out of the feed.
	a := torznabApp(t, types.AcceptConfig{MediaTypes: []string{"Ebook"}}, types.GrabConfig{},
		types.TorznabConfig{BaseURL: "http://mt"}, healthyUser, twoResults)
	body := get(Torznab(a), "t=search").Body.String()
	if !strings.Contains(body, `value="7020"`) {
		t.Error("ebook should be present")
	}
	if strings.Contains(body, `value="3030"`) {
		t.Error("audiobook should be filtered by Layer-1 media type")
	}
}

func TestTorznabDownloadGrab(t *testing.T) {
	a := torznabApp(t, types.AcceptConfig{}, types.GrabConfig{UnsatMargin: 5},
		types.TorznabConfig{}, healthyUser, "")
	w := get(TorznabDownload(a),
		"id=42&name=An+Ebook&cat=Ebooks+-+Fantasy&ft=epub&lang=English&size=100&vip=0&fl=1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/x-bittorrent" {
		t.Errorf("content-type = %q", ct)
	}
	if w.Body.String() != "TORRENTBYTES" {
		t.Errorf("body = %q", w.Body.String())
	}
	if seen, _ := a.Store.GrabSeen(context.Background(), 42); !seen {
		t.Error("grab should be recorded")
	}
	rows, _ := a.Store.GrabsBySource(context.Background())
	if len(rows) != 1 || rows[0].Key != string(types.SourceTorznab) || rows[0].N != 1 {
		t.Errorf("grabs by source = %+v, want one torznab grab", rows)
	}
}

func TestTorznabDownloadHeld(t *testing.T) {
	// account at the unsat ceiling → gate holds → 404, no grab recorded.
	a := torznabApp(t, types.AcceptConfig{}, types.GrabConfig{UnsatMargin: 5},
		types.TorznabConfig{}, ceilingUser, "")
	w := get(TorznabDownload(a), "id=42&cat=Ebooks+-+Fantasy&size=100&fl=0")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if seen, _ := a.Store.GrabSeen(context.Background(), 42); seen {
		t.Error("held grab must not be recorded")
	}
}

func TestTorznabDownloadIdempotent(t *testing.T) {
	a := torznabApp(t, types.AcceptConfig{}, types.GrabConfig{UnsatMargin: 5},
		types.TorznabConfig{}, healthyUser, "")
	// a prior grab of tid 99 already exists.
	if _, err := a.Store.InsertGrab(context.Background(), store.Grab{
		Source: types.SourceTorznab, TID: 99, Result: string(types.ResultAdded),
	}); err != nil {
		t.Fatal(err)
	}
	w := get(TorznabDownload(a), "id=99&cat=Ebooks+-+Fantasy&size=100&fl=0")
	if w.Code != http.StatusOK || w.Body.String() != "TORRENTBYTES" {
		t.Fatalf("re-serve: status=%d body=%q", w.Code, w.Body.String())
	}
	// no second grab row was written.
	rows, _ := a.Store.GrabsByResult(context.Background())
	var added int64
	for _, r := range rows {
		if r.Key == string(types.ResultAdded) {
			added = r.N
		}
	}
	if added != 1 {
		t.Errorf("added grabs = %d, want 1 (idempotent re-serve)", added)
	}
}

func TestTorznabDownloadTTLExpired(t *testing.T) {
	a := torznabApp(t, types.AcceptConfig{}, types.GrabConfig{UnsatMargin: 5},
		types.TorznabConfig{LinkTTL: time.Hour}, healthyUser, "")
	stale := time.Now().Add(-2 * time.Hour).Unix()
	q := url.Values{"id": {"42"}, "cat": {"Ebooks - Fantasy"}, "size": {"100"}, "fl": {"0"}, "ts": {itoa(stale)}}
	w := get(TorznabDownload(a), q.Encode())
	if w.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", w.Code)
	}
}

func TestTorznabAuth(t *testing.T) {
	tz := types.TorznabConfig{APIKey: "secret"}
	a := torznabApp(t, types.AcceptConfig{}, types.GrabConfig{}, tz, healthyUser, twoResults)

	if w := get(Torznab(a), "t=caps"); w.Code != http.StatusUnauthorized {
		t.Errorf("no key: status = %d, want 401", w.Code)
	}
	if w := get(Torznab(a), "t=caps&apikey=wrong"); w.Code != http.StatusUnauthorized {
		t.Errorf("bad key: status = %d, want 401", w.Code)
	}
	if w := get(Torznab(a), "t=caps&apikey=secret"); w.Code != http.StatusOK {
		t.Errorf("good key: status = %d, want 200", w.Code)
	}
}

func TestMainCatsFor(t *testing.T) {
	cases := map[string][]int{
		"":          {13, 14},
		"7020":      {14},
		"3030":      {13},
		"7000,3000": {13, 14},
		"9999":      {13, 14}, // unrecognized → default both
	}
	for in, want := range cases {
		got := mainCatsFor(in)
		if len(got) != len(want) {
			t.Errorf("mainCatsFor(%q) = %v, want %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("mainCatsFor(%q) = %v, want %v", in, got, want)
				break
			}
		}
	}
}

// The search API sends ISO 639-2/B codes where the announce sends names; both must
// end up as the same value or the substring language rule rejects every search result.
func TestLangSliceCSV(t *testing.T) {
	cases := map[string][]string{
		// bibliographic spellings MAM actually sends (FRE not FRA, GER not DEU)
		"ENG":     {"english"},
		"FRE":     {"french"},
		"GER":     {"german"},
		"DUT":     {"dutch"},
		"SPA":     {"spanish"},
		"ENG,POL": {"english", "polish"},
		// names arrive already resolved on the minted-link round trip — leave them be
		"english":              {"english"},
		"castilian spanish":    {"castilian spanish"},
		"brazilian portuguese": {"brazilian portuguese"},
		// a 2-letter value must not be read as a BCP-47 tag ("no" = Norwegian Bokmål)
		"no": {"no"},
		// unknown code stays itself: matches no allowlist, still legible in the logs
		"zzz": {"zzz"},
		"":    nil,
	}
	for in, want := range cases {
		got := langSliceCSV(in)
		if len(got) != len(want) {
			t.Errorf("langSliceCSV(%q) = %q, want %q", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("langSliceCSV(%q) = %q, want %q", in, got, want)
				break
			}
		}
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
