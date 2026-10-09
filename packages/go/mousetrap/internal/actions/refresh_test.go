package actions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"homelab/mousetrap/internal/app"
	"homelab/mousetrap/internal/types"
)

// refreshApp builds an app against a MAM mock that routes by path, so one
// fixture serves the account, freeleech and bonus refreshes as well as the
// ad-hoc search.
func refreshApp(t *testing.T, mamURL string) *app.App {
	t.Helper()
	seed := filepath.Join(t.TempDir(), "seed")
	if err := os.WriteFile(seed, []byte("initial"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := app.New(types.Config{
		StateDir: t.TempDir(),
		MAM:      types.MAMConfig{APIBase: mamURL, SeedPath: seed},
	}, prometheus.NewRegistry())
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// mamRoutes answers the endpoints the refresh handlers call.
func mamRoutes(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonLoad.php", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"classname":"User","ratio":"2.0","uploaded":"1.00 TiB","downloaded":"500 GiB","wedges":2,"unsat":{"count":1,"limit":20}}`))
	})
	mux.HandleFunc("/tor/js/loadSearchJSONbasic.php", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total":1,"total_found":1,"data":[{"id":7,"title":"Freebie","size":"1.0 MiB"}]}`))
	})
	mux.HandleFunc("/json/userBonusHistory.php", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"timestamp":1700000000,"amount":100,"type":"giftPoints","tid":null,"title":null,"other_userid":null,"other_name":null},
			{"timestamp":1700000100,"amount":-5,"type":"wedgePF","tid":null,"title":null,"other_userid":null,"other_name":null}]`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestAccountRefreshesSnapshot(t *testing.T) {
	a := refreshApp(t, mamRoutes(t).URL)

	w := httptest.NewRecorder()
	Account(a)(w, httptest.NewRequest(http.MethodPost, "/refresh/account", http.NoBody))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := a.State.Account(); got.Wedges != 2 || got.UnsatLimit != 20 {
		t.Errorf("snapshot = %+v", got)
	}
}

// A MAM that refuses is reported as a bad gateway: the fault is upstream, and
// the timer's failure should not read as a mousetrap bug.
func TestAccountReportsMAMFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	a := refreshApp(t, srv.URL)

	w := httptest.NewRecorder()
	Account(a)(w, httptest.NewRequest(http.MethodPost, "/refresh/account", http.NoBody))

	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
}

func TestFreeleechQueriesMAM(t *testing.T) {
	a := refreshApp(t, mamRoutes(t).URL)

	w := httptest.NewRecorder()
	Freeleech(a)(w, httptest.NewRequest(http.MethodPost, "/refresh/freeleech", http.NoBody))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// Bonus mirrors new rows and dedupes by the stored cursor, so a second run
// inserts nothing.
func TestBonusMirrorsAndDedupes(t *testing.T) {
	a := refreshApp(t, mamRoutes(t).URL)
	ctx := context.Background()

	for i := range 2 {
		w := httptest.NewRecorder()
		Bonus(a)(w, httptest.NewRequest(http.MethodPost, "/refresh/bonus", http.NoBody))
		if w.Code != http.StatusOK {
			t.Fatalf("run %d: status = %d, want 200", i, w.Code)
		}
	}

	total, err := a.Store.BonusTotal(ctx)
	if err != nil {
		t.Fatalf("BonusTotal: %v", err)
	}
	if total != 2 {
		t.Errorf("bonus rows = %d, want 2 (the second run must dedupe)", total)
	}

	cursor, err := a.Store.BonusCursor(ctx)
	if err != nil {
		t.Fatalf("BonusCursor: %v", err)
	}
	if cursor != 1700000100 {
		t.Errorf("cursor = %v, want 1700000100", cursor)
	}
}

func TestSearchProxiesMAM(t *testing.T) {
	a := refreshApp(t, mamRoutes(t).URL)

	w := httptest.NewRecorder()
	Search(a)(w, httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(`{"text":"free"}`)))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, `"total_found":1`) {
		t.Errorf("body = %s", body)
	}
}

// Query runs SQL against the ledgers and returns rows as JSON.
func TestQueryReturnsRows(t *testing.T) {
	a := refreshApp(t, mamRoutes(t).URL)

	w := httptest.NewRecorder()
	Query(a)(w, httptest.NewRequest(http.MethodPost, "/query",
		strings.NewReader(`SELECT COUNT(*) AS n FROM announces`)))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, `"n":0`) {
		t.Errorf("body = %s", body)
	}
}

func TestQueryRejectsBadSQL(t *testing.T) {
	a := refreshApp(t, mamRoutes(t).URL)

	w := httptest.NewRecorder()
	Query(a)(w, httptest.NewRequest(http.MethodPost, "/query", strings.NewReader("SELECT * FROM nope")))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
