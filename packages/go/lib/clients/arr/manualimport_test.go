package arr

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"homelab/httpx"
)

func newTestLidarr(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := New(Lidarr, httpx.Config{BaseURL: srv.URL, RetryMax: -1}, "k")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// resolved is one fully-identified listing entry.
func resolved(path string) ManualImportItem {
	return ManualImportItem{
		Path:           path,
		Artist:         Ref{ID: 34},
		Album:          Ref{ID: 408},
		AlbumReleaseID: 1589,
		Tracks:         []Ref{{ID: 10808}},
		Quality:        json.RawMessage(`{"quality":{"id":6,"name":"FLAC"}}`),
	}
}

// Everything has to resolve. Anything short of all four leaves a file with no
// track to import it as, and importing the rest would leave the album short.
func TestManualImportItemPlaced(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mutate func(*ManualImportItem)
		want   bool
	}{
		"fully resolved": {func(*ManualImportItem) {}, true},
		"no artist":      {func(i *ManualImportItem) { i.Artist = Ref{} }, false},
		"no album":       {func(i *ManualImportItem) { i.Album = Ref{} }, false},
		"no release":     {func(i *ManualImportItem) { i.AlbumReleaseID = 0 }, false},
		"no tracks":      {func(i *ManualImportItem) { i.Tracks = nil }, false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			item := resolved("/a/01.flac")
			tc.mutate(&item)
			if got := item.Placed(); got != tc.want {
				t.Errorf("Placed() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Lidarr sends null for a reference it could not resolve, which must decode
// to an unresolved item rather than failing the whole listing.
func TestManualImportItemDecodesNullReferences(t *testing.T) {
	t.Parallel()

	var item ManualImportItem
	raw := `{"path":"/a/01.flac","artist":null,"album":{},"albumReleaseId":null,"tracks":null}`
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if item.Placed() {
		t.Error("Placed() = true for an item with nothing resolved")
	}
}

func TestManualImportItemReasons(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		rejections []Rejection
		want       []string
	}{
		"none given":   {nil, []string{"no reason given"}},
		"one":          {[]Rejection{{Reason: "Unknown album"}}, []string{"Unknown album"}},
		"empty reason": {[]Rejection{{}}, []string{"no reason given"}},
		"several": {
			[]Rejection{{Reason: "Unknown album"}, {Reason: "Unknown artist"}},
			[]string{"Unknown album", "Unknown artist"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			item := ManualImportItem{Rejections: tc.rejections}
			if diff := cmp.Diff(tc.want, item.Reasons()); diff != "" {
				t.Errorf("Reasons() (-want +got):\n%s", diff)
			}
		})
	}
}

func TestManualImportAsksForTheFolder(t *testing.T) {
	t.Parallel()

	var gotPath, gotQuery, gotKey string
	c := newTestLidarr(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query().Encode()
		gotKey = r.Header.Get("X-Api-Key")
		_, _ = w.Write([]byte(`[{"path":"/rips/A/B/01.flac","artist":{"id":34},
		  "album":{"id":408},"albumReleaseId":1589,"tracks":[{"id":10808}],
		  "quality":{"quality":{"id":6,"name":"FLAC"}}}]`))
	}))

	got, err := c.ManualImport(context.Background(), "/rips/A/B")
	if err != nil {
		t.Fatalf("ManualImport() error = %v", err)
	}

	if gotPath != "/api/v1/manualimport" {
		t.Errorf("path = %q", gotPath)
	}
	// filterExistingFiles=false asks for the folder as it is rather than
	// as a diff against the library.
	if gotQuery != "filterExistingFiles=false&folder=%2Frips%2FA%2FB" {
		t.Errorf("query = %q", gotQuery)
	}
	if gotKey != "k" {
		t.Errorf("X-Api-Key = %q, want k", gotKey)
	}
	if len(got) != 1 || !got[0].Placed() {
		t.Errorf("items = %+v, want one resolved entry", got)
	}
}

// The listing is Lidarr-shaped. Run against Sonarr or Radarr every id would
// decode as zero and the import would merely look unresolved.
func TestManualImportRefusesOtherApps(t *testing.T) {
	t.Parallel()

	for _, kind := range []Kind{Sonarr, Radarr} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			c, err := New(kind, httpx.Config{BaseURL: "http://arr.test"}, "k")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.ManualImport(context.Background(), "/a"); !errors.Is(err, ErrNotLidarr) {
				t.Errorf("ManualImport() error = %v, want ErrNotLidarr", err)
			}
		})
	}
}

func TestManualImportReportsServerFailure(t *testing.T) {
	t.Parallel()

	c := newTestLidarr(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	if _, err := c.ManualImport(context.Background(), "/a"); err == nil {
		t.Fatal("ManualImport() error = nil, want a failure")
	}
}

func TestNewManualImportCarriesEveryIDLidarrResolved(t *testing.T) {
	t.Parallel()

	got := NewManualImport([]ManualImportItem{resolved("/a/01.flac")}, ImportCopy)

	want := ManualImportCommand{
		Name:                 "ManualImport",
		ImportMode:           "Copy",
		ReplaceExistingFiles: false,
		Files: []ManualImportFile{{
			Path:           "/a/01.flac",
			ArtistID:       34,
			AlbumID:        408,
			AlbumReleaseID: 1589,
			TrackIDs:       []int64{10808},
			Quality:        json.RawMessage(`{"quality":{"id":6,"name":"FLAC"}}`),
			// Left on so Lidarr may still adopt the release the files
			// actually are, which is the point of tagging them from
			// the disc.
			DisableReleaseSwitching: false,
		}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("NewManualImport() (-want +got):\n%s", diff)
	}
}

func TestNewManualImportOneEntryPerFile(t *testing.T) {
	t.Parallel()

	got := NewManualImport(
		[]ManualImportItem{resolved("/a/01.flac"), resolved("/a/02.flac")},
		ImportMove,
	)

	if got.ImportMode != "Move" {
		t.Errorf("ImportMode = %q, want Move", got.ImportMode)
	}
	paths := []string{got.Files[0].Path, got.Files[1].Path}
	if diff := cmp.Diff([]string{"/a/01.flac", "/a/02.flac"}, paths); diff != "" {
		t.Errorf("paths (-want +got):\n%s", diff)
	}
}

// The quality object is Lidarr's own model and goes back verbatim: a field
// dropped in the round trip is a different quality than the one identified.
func TestNewManualImportRoundTripsQualityVerbatim(t *testing.T) {
	t.Parallel()

	item := resolved("/a/01.flac")
	item.Quality = json.RawMessage(`{"quality":{"id":6,"name":"FLAC"},"revision":{"version":2}}`)

	encoded, err := json.Marshal(NewManualImport([]ManualImportItem{item}, ImportCopy))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	want := `"quality":{"quality":{"id":6,"name":"FLAC"},"revision":{"version":2}}`
	if !strings.Contains(string(encoded), want) {
		t.Errorf("encoded command dropped quality detail:\n%s", encoded)
	}
}
