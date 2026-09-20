package beetssvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"homelab/clients/beets"
	"homelab/httpx"
	"homelab/sidecar"
)

// newService points a service at a handler standing in for beets' web plugin.
func newService(t *testing.T, h http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := beets.New(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return New(c, srv.URL)
}

// library serves n items, each with a path unless pathless is set.
func library(t *testing.T, n int, pathless bool) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/stats") {
			_, _ = w.Write([]byte(`{"items":7,"albums":2}`))
			return
		}
		items := make([]string, 0, n)
		for i := range n {
			path := fmt.Sprintf(`"/music/track-%d.flac"`, i)
			if pathless {
				path = "null"
			}
			items = append(items, fmt.Sprintf(
				`{"id":%d,"path":%s,"album":"A","mb_releasegroupid":"rg-%d","added":1690000000}`,
				i, path, i))
		}
		_, _ = fmt.Fprintf(w, `{"items":[%s]}`, strings.Join(items, ","))
	}
}

func TestInfo(t *testing.T) {
	got := newService(t, library(t, 0, false)).Info()

	if got.Sidecar != "managarr-beets" || got.Source != Source {
		t.Errorf("info = %+v", got)
	}
	// Changes only: beets is a command line tool with a read API bolted on
	// and has no notification of any kind.
	if len(got.Capabilities) != 1 || got.Capabilities[0] != sidecar.CapChanges {
		t.Errorf("capabilities = %v, want [changes]", got.Capabilities)
	}
}

func TestEnrichUnsupported(t *testing.T) {
	_, err := newService(t, library(t, 1, false)).
		Enrich(context.Background(), []byte(`{}`))

	if !errors.Is(err, sidecar.ErrUnsupported) {
		t.Errorf("err = %v, want ErrUnsupported", err)
	}
}

func TestChanges(t *testing.T) {
	events, err := newService(t, library(t, 3, false)).
		Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	e := events[0]
	if e.Source != Source || e.Kind != KindItem {
		t.Errorf("event = %s/%s", e.Source, e.Kind)
	}
	// The path is the key in managarr's table and the only thing tying a
	// tagged item to a file — and through that file's inode, to the download
	// that produced it.
	if e.Ref != "/music/track-0.flac" {
		t.Errorf("ref = %q", e.Ref)
	}
	if e.Cursor != "0" {
		t.Errorf("cursor = %q, want the item id", e.Cursor)
	}
	if e.OccurredAt.IsZero() {
		t.Error("added time was lost")
	}

	got, err := sidecar.DecodePayload[beets.Item](e)
	if err != nil {
		t.Fatal(err)
	}
	if got.MbReleaseGroupID != "rg-0" {
		t.Errorf("payload = %+v; mb_releasegroupid is what joins to Lidarr", got)
	}
}

func TestChangesResumes(t *testing.T) {
	svc := newService(t, library(t, 5, false))

	events, err := svc.Changes(context.Background(), "2", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want the 2 after id 2", len(events))
	}
	if events[0].Ref != "/music/track-3.flac" {
		t.Errorf("resumed at %q", events[0].Ref)
	}
}

func TestChangesRespectsLimit(t *testing.T) {
	events, err := newService(t, library(t, 10, false)).
		Changes(context.Background(), "", 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Errorf("got %d events, want 4", len(events))
	}
}

func TestChangesEmptyLibrary(t *testing.T) {
	events, err := newService(t, library(t, 0, false)).
		Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
	}
}

// include_paths defaults to off, and with it off every item comes back with a
// null path. Said out loud rather than left to the consumer: from managarr's
// side an empty result is indistinguishable from a library of untagged imports,
// and the path is what carries the inode that links a file to its download.
func TestChangesRefusesAPathlessLibrary(t *testing.T) {
	_, err := newService(t, library(t, 3, true)).Changes(context.Background(), "", 10)

	if err == nil {
		t.Fatal("expected an error when every item lacks a path")
	}
	if !strings.Contains(err.Error(), "include_paths") {
		t.Errorf("err = %v; it must name the option to set", err)
	}
}

func TestChangesBadCursor(t *testing.T) {
	svc := newService(t, library(t, 1, false))

	for _, c := range []string{"abc", "-1"} {
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

func TestStatsRoute(t *testing.T) {
	svc := newService(t, library(t, 1, false))
	mux := http.NewServeMux()
	svc.Routes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, StatsPath, http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}

	var got beets.Stats
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Items != 7 || got.Albums != 2 {
		t.Errorf("stats = %+v", got)
	}
}

func TestStatsRouteReportsFailure(t *testing.T) {
	svc := newService(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux := http.NewServeMux()
	svc.Routes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, StatsPath, http.NoBody))
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
