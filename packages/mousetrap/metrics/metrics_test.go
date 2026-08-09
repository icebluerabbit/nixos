package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mousetrap/mam"
	"mousetrap/state"
	"mousetrap/store"
	"mousetrap/types"
)

type fakeSession struct{}

func (fakeSession) Load() (string, error) { return "x", nil }
func (fakeSession) Save(string) error     { return nil }

func TestCollectorExports(t *testing.T) {
	mamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"classname":"User","ratio":"14.4","uploaded":"2.00 TiB","downloaded":"1.00 TiB","wedges":12,"unsat":{"count":11,"limit":20},"seedHnr":3,"leeching":2}`))
	}))
	defer mamSrv.Close()

	st := state.New(mam.New(mamSrv.URL, "", fakeSession{}))
	if err := st.RefreshAccount(context.Background()); err != nil {
		t.Fatalf("RefreshAccount: %v", err)
	}

	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	// seed ledgers so the derived counters have something to summarize.
	id, _ := db.InsertAnnounce(ctx, store.Announce{
		TID: 1, Kind: "ebook", Category: "Ebooks - Fantasy", AcceptDecision: "accept", AcceptReason: "ok",
	})
	_ = db.UpdateAnnounceGrab(ctx, id, "grab", "ok")
	_, _ = db.InsertGrab(ctx, store.Grab{Source: types.SourceAnnounce, TID: 1, Size: 500, CostsRatio: true, Result: "added"})

	m := New(st, db, time.Minute)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	for _, want := range []string{
		"mam_unsat_count 11",
		"mam_wedges 12",
		"mam_leeching_count 2",
		"mam_hnr_count 3",
		`mam_class_info{classname="User"} 1`,
		"mousetrap_announces_total 1",
		`mousetrap_announces_by_type_total{type="ebook"} 1`,
		`mousetrap_would_accept_total{decision="accept",reason="ok"} 1`,
		`mousetrap_would_grab_total{decision="grab",reason="ok"} 1`,
		`mousetrap_grab_action_total{result="added"} 1`,
		"mousetrap_grab_bytes_total 500",
		`mousetrap_announce_size_bytes_count{type="ebook"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
}
