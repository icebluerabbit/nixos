package bazarrsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"homelab/clients/bazarr"
	"homelab/httpx"
	"homelab/sidecar"
)

// newService points a service at a handler standing in for Bazarr.
func newService(t *testing.T, h http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := bazarr.New(httpx.Config{BaseURL: srv.URL}, "key")
	if err != nil {
		t.Fatal(err)
	}
	return New(c, srv.URL)
}

// history serves series and movies separately, which is how Bazarr keeps them —
// two collections rather than one with a discriminator.
func history(t *testing.T, series, movies int) http.HandlerFunc {
	t.Helper()
	records := func(n int, prefix string) string {
		out := make([]string, 0, n)
		for i := range n {
			out = append(out, fmt.Sprintf(
				`{"id":%d,"action":1,"provider":"opensubtitles",`+
					`"video_path":"/media/%s-%d.mkv","subtitles_path":"/media/%s-%d.srt"}`,
				i, prefix, i, prefix, i))
		}
		return strings.Join(out, ",")
	}
	// Bazarr's real route shape: collection first, resource second, and the TV
	// collection named after the episode. Matched exactly rather than loosely
	// — a stub that accepted both orderings is what let the client call a path
	// Bazarr has never served.
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/episodes/history":
			fmt.Fprintf(w, `{"data":[%s],"total":%d}`, records(series, "ep"), series)
		case "/api/movies/history":
			fmt.Fprintf(w, `{"data":[%s],"total":%d}`, records(movies, "mv"), movies)
		case "/api/episodes/blacklist", "/api/movies/blacklist":
			_, _ = w.Write([]byte(`{"data":[{"provider":"p","subs_id":"s"}],"total":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestInfo(t *testing.T) {
	got := newService(t, history(t, 0, 0)).Info()

	if got.Sidecar != "managarr-bazarr" || got.Source != Source {
		t.Errorf("info = %+v", got)
	}
	// Changes only. Bazarr notifies through Apprise providers rather than a
	// plain HTTP callback, so claiming CapEvents would promise a webhook that
	// will never arrive.
	if len(got.Capabilities) != 1 || got.Capabilities[0] != sidecar.CapChanges {
		t.Errorf("capabilities = %v, want [changes]", got.Capabilities)
	}
}

func TestEnrichUnsupported(t *testing.T) {
	_, err := newService(t, history(t, 1, 1)).
		Enrich(context.Background(), []byte(`{}`))

	if !errors.Is(err, sidecar.ErrUnsupported) {
		t.Errorf("err = %v, want ErrUnsupported", err)
	}
}

// Both halves have to be reachable from one walk, or movies are never read:
// the consumer stops on a short page and has no idea there is a second
// collection behind it.
func TestChangesCoversBothHalves(t *testing.T) {
	svc := newService(t, history(t, 3, 2))
	ctx := context.Background()

	var (
		kinds  = map[string]int{}
		cursor string
	)
	for range 10 {
		events, err := svc.Changes(ctx, cursor, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) == 0 {
			break
		}
		for _, e := range events {
			kinds[e.Kind]++
		}
		cursor = events[len(events)-1].Cursor
	}

	if kinds[KindSeriesHistory] != 3 {
		t.Errorf("series records = %d, want 3", kinds[KindSeriesHistory])
	}
	if kinds[KindMovieHistory] != 2 {
		t.Errorf("movie records = %d, want 2", kinds[KindMovieHistory])
	}
}

func TestChangesEventShape(t *testing.T) {
	events, err := newService(t, history(t, 1, 0)).
		Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("no events")
	}

	e := events[0]
	if e.Source != Source || e.Kind != KindSeriesHistory {
		t.Errorf("event = %s/%s", e.Source, e.Kind)
	}
	// The video file the subtitle belongs to — the only identifier Bazarr
	// shares with the rest of the library.
	if e.Ref != "/media/ep-0.mkv" {
		t.Errorf("ref = %q", e.Ref)
	}

	got, err := sidecar.DecodePayload[bazarr.HistoryRecord](e)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "opensubtitles" {
		t.Errorf("payload = %+v", got)
	}
}

// Bazarr numbers from 1 today, which is exactly what would hide an off-by-one
// on the first record forever.
func TestChangesIncludesRecordZero(t *testing.T) {
	events, err := newService(t, history(t, 1, 0)).
		Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want the record with id 0", len(events))
	}
}

func TestChangesResumesWithinAHalf(t *testing.T) {
	svc := newService(t, history(t, 5, 0))
	ctx := context.Background()

	first, err := svc.Changes(ctx, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("got %d events, want 2", len(first))
	}

	second, err := svc.Changes(ctx, first[1].Cursor, 2)
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Ref != "/media/ep-2.mkv" {
		t.Errorf("resumed at %q, want ep-2", second[0].Ref)
	}
}

func TestChangesEmpty(t *testing.T) {
	events, err := newService(t, history(t, 0, 0)).
		Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
	}
}

func TestChangesBadCursor(t *testing.T) {
	svc := newService(t, history(t, 1, 1))

	for _, c := range []string{"abc", "-1:0", "nocolon"} {
		if _, err := svc.Changes(context.Background(), c, 10); err == nil {
			t.Errorf("cursor %q was accepted", c)
		}
	}
}

func TestChangesUpstreamError(t *testing.T) {
	svc := newService(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if _, err := svc.Changes(context.Background(), "", 10); err == nil {
		t.Fatal("expected an error")
	}
}

func TestWantedRoute(t *testing.T) {
	svc := newService(t, history(t, 0, 0))
	mux := http.NewServeMux()
	svc.Routes(mux)

	for _, kind := range []string{"series", "movies"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, WantedPath+"?kind="+kind, http.NoBody))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d: %s", kind, rec.Code, rec.Body)
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body["blacklist"]; !ok {
			t.Errorf("%s: response has no blacklist: %s", kind, rec.Body)
		}
	}
}

// Bazarr keeps the two halves under different paths, so a missing or unknown
// kind cannot be guessed — refusing it is better than picking one.
func TestWantedRouteRequiresAValidKind(t *testing.T) {
	svc := newService(t, history(t, 0, 0))
	mux := http.NewServeMux()
	svc.Routes(mux)

	for _, q := range []string{"", "?kind=", "?kind=books"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, WantedPath+q, http.NoBody))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q = %d, want 400", q, rec.Code)
		}
	}
}

func TestWantedRouteReportsFailure(t *testing.T) {
	svc := newService(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux := http.NewServeMux()
	svc.Routes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, WantedPath+"?kind=series", http.NoBody))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestImplementsContract(t *testing.T) {
	var (
		_ sidecar.Service  = (*Service)(nil)
		_ sidecar.Extender = (*Service)(nil)
	)
}
