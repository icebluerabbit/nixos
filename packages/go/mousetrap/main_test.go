package main

// This is the end-to-end integration test: it wires the real application the way
// main() does — CLI flags → config.Parse → app.New → server.New — and drives it
// over a live httptest server against a path-routed MAM mock and a qBittorrent
// mock. Unlike the per-package unit tests (which exercise one handler or one pure
// function), this asserts the whole pipeline's cross-component effects: an announce
// flows accept → grab → download → client-add, the SQLite ledgers record it, and
// the derived series surface on /metrics.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"homelab/mousetrap/internal/app"
	"homelab/mousetrap/internal/config"
	"homelab/mousetrap/internal/server"
	"homelab/mousetrap/internal/types"
)

// userJSON is a healthy self view: class User (not VIP), 5/20 unsat (well under the
// ceiling), 5 wedges, 10000 points (enough for a VIP buy), full snatch breakdown.
const userJSON = `{
	"uid":"1","username":"tester","classname":"User",
	"ratio":"2.50","uploaded":"2.00 TiB","downloaded":"1.00 TiB",
	"seedbonus":10000,"wedges":5,
	"unsat":{"count":5,"limit":20},
	"sSat":{"count":100},"seedUnsat":{"count":2},"seedHnr":{"count":1},
	"leeching":{"count":3},"inactHnr":{"count":0}
}`

// mamMock routes MAM endpoints by path: jsonLoad → the user view, download → fake
// .torrent bytes, search → one result, bonusBuy → success, bonusHistory → one row.
func mamMock(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonLoad.php", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, userJSON)
	})
	mux.HandleFunc("/tor/download.php", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "d8:announce0:e") // minimal fake bencoded body
	})
	mux.HandleFunc("/tor/js/loadSearchJSONbasic.php", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"total":1,"total_found":1,"data":[{"id":42,"title":"A Book","size":"6.0 MiB"}]}`)
	})
	mux.HandleFunc("/json/bonusBuy.php", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success_count":1}`)
	})
	mux.HandleFunc("/json/userBonusHistory.php", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"timestamp":1700000000,"amount":100,"type":"giftPoints","tid":null,"title":null,"other_userid":null,"other_name":null}]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// qbtMock is a qBittorrent WebUI that accepts every add.
func qbtMock(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "Ok.")
	}))
	t.Cleanup(srv.Close)
	return srv
}

// buildApp assembles the application exactly as main() does — through the flag layer
// — pointed at the mocks, armed (grab + shop dry-run off).
func buildApp(t *testing.T, mamURL, qbtURL string) *app.App {
	t.Helper()
	seed := filepath.Join(t.TempDir(), "seed")
	if err := os.WriteFile(seed, []byte("seed-mam-id"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse([]string{
		"--state-dir=" + t.TempDir(),
		"--mam-api-base=" + mamURL,
		"--mam-seed-path=" + seed,
		"--client-url=" + qbtURL,
		"--client-category=ebook=books",
		"--client-category=audiobook=audiobooks",
		"--accept-size-min=50", // reject tiny releases so a reject path is exercised
		"--grab-unsat-margin=5",
		"--grab-dry-run=false", // arm grabs
		"--shop-dry-run=false", // arm shop
		"--shop-vip-enable=true",
		"--shop-vip-buyable-class=User",
		"--shop-vip-cost=5000",
	})
	if err != nil {
		t.Fatalf("config.Parse: %v", err)
	}
	a, err := app.New(cfg, prometheus.NewRegistry())
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// post fires a request at the live server and returns the status and body.
func post(t *testing.T, c *http.Client, url, body string) (status int, resBody string) {
	t.Helper()
	resp, err := c.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestIntegration(t *testing.T) {
	mam := mamMock(t)
	qbt := qbtMock(t)
	a := buildApp(t, mam.URL, qbt.URL)

	ts := httptest.NewServer(server.New(a))
	t.Cleanup(ts.Close)
	c := ts.Client()

	// ── health ──
	if resp, err := c.Get(ts.URL + "/health"); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /health: err=%v status=%v", err, resp)
	}

	// ── refresh account snapshot (feeds the gate + the account gauges) ──
	if code, body := post(t, c, ts.URL+"/refresh/account", ""); code != http.StatusOK {
		t.Fatalf("refresh/account: %d %s", code, body)
	}

	// ── announce that accepts and grabs (ebook, 100 bytes ≥ the 50-byte floor) ──
	code, body := post(t, c, ts.URL+"/announce",
		`{"category":"Ebooks - Fantasy","language":["English"],"languageName":"English","tags":"epub","size":"100","torrentId":"100"}`)
	if code != http.StatusOK {
		t.Fatalf("announce grab: status=%d body=%s", code, body)
	}
	if got := decodeField(t, body, "result"); got != string(types.ResultAdded) {
		t.Errorf("grab result = %q, want %q", got, types.ResultAdded)
	}
	if seen, _ := a.Store.GrabSeen(context.Background(), 100); !seen {
		t.Error("tid 100 should be recorded as grabbed")
	}

	// ── announce that is rejected by the size floor (10 < 50) ──
	code, body = post(t, c, ts.URL+"/announce",
		`{"category":"Ebooks - Fantasy","size":"10","torrentId":"101"}`)
	if code != http.StatusNotAcceptable {
		t.Fatalf("announce reject: status=%d body=%s", code, body)
	}
	if got := decodeField(t, body, "reason"); got != string(types.AcceptSizeMin) {
		t.Errorf("reject reason = %q, want %q", got, types.AcceptSizeMin)
	}

	// ── re-announcing the grabbed tid is deduped ──
	code, body = post(t, c, ts.URL+"/announce",
		`{"category":"Ebooks - Fantasy","size":"100","torrentId":"100"}`)
	if code != http.StatusNotAcceptable || decodeField(t, body, "reason") != string(types.AcceptDuplicate) {
		t.Errorf("dedup: status=%d body=%s", code, body)
	}

	// ── ad-hoc search proxies MAM verbatim ──
	code, body = post(t, c, ts.URL+"/search", `{"text":"a book"}`)
	if code != http.StatusOK {
		t.Fatalf("search: status=%d body=%s", code, body)
	}
	var sr struct {
		TotalFound int `json:"total_found"`
	}
	if err := json.Unmarshal([]byte(body), &sr); err != nil || sr.TotalFound != 1 {
		t.Errorf("search body = %s (err %v)", body, err)
	}

	// ── mirror the bonus ledger ──
	if code, body := post(t, c, ts.URL+"/refresh/bonus", ""); code != http.StatusOK {
		t.Fatalf("refresh/bonus: %d %s", code, body)
	}

	// ── shop buys VIP (class User, 10000 pts − 5000 cost ≥ 0 reserve) ──
	if code, body := post(t, c, ts.URL+"/shop", ""); code != http.StatusOK {
		t.Fatalf("shop: %d %s", code, body)
	}

	// ── scrape /metrics and assert the derived series reflect every ledger ──
	resp, err := c.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	m, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	metrics := string(m)

	wants := []string{
		`mam_class_info{classname="User"} 1`,            // account snapshot (state)
		`mam_wedges 5`,                                  // account snapshot (state)
		`mousetrap_announces_total 2`,                   // 1 accepted + 1 rejected (dedup not stored)
		`mousetrap_grab_action_total{result="added"} 1`, // grabs ledger
		`mousetrap_grabs_by_source_total{source="announce"} 1`,
		`mousetrap_would_accept_total{decision="reject",reason="size_min"} 1`,
		`mousetrap_spends_total{reason="shop_vip"} 1`,         // spends ledger (from the shop buy)
		`mousetrap_bonus_received_total{type="giftPoints"} 1`, // bonuses ledger
	}
	for _, want := range wants {
		if !strings.Contains(metrics, want) {
			t.Errorf("metrics missing %q", want)
		}
	}
}

// decodeField pulls one string field out of a JSON object body.
func decodeField(t *testing.T, body, field string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("decode %s: %v (body %s)", field, err, body)
	}
	s, _ := m[field].(string)
	return s
}
