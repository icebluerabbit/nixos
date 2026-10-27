package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"homelab/clients/crossseed"
	"homelab/sidecar"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE searchee (id INTEGER PRIMARY KEY, name TEXT);
CREATE TABLE decision (
	id INTEGER PRIMARY KEY, searchee_id INTEGER, guid TEXT, info_hash TEXT,
	decision TEXT, first_seen INTEGER, last_seen INTEGER
);
CREATE TABLE client_searchee (
	client_host TEXT, info_hash TEXT, name TEXT, title TEXT,
	files TEXT, length INTEGER, save_path TEXT, category TEXT,
	tags TEXT, trackers TEXT,
	PRIMARY KEY (client_host, info_hash)
);
CREATE TABLE ensemble (
	client_host TEXT, path TEXT, info_hash TEXT, ensemble TEXT, element TEXT,
	PRIMARY KEY (client_host, path)
);`

// newService builds a service over a database seeded with n decisions and one
// client torrent.
func newService(t *testing.T, n int) *Service {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cross-seed.db")

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	for i := range n {
		if _, err := db.Exec(`INSERT INTO searchee (id, name) VALUES (?, ?)`,
			i, fmt.Sprintf("Release.%d", i)); err != nil {
			t.Fatal(err)
		}
		_, err := db.Exec(`
			INSERT INTO decision (id, searchee_id, guid, info_hash, decision, first_seen, last_seen)
			VALUES (?, ?, ?, ?, 'SIZE_MISMATCH', ?, ?)`,
			i, i, fmt.Sprintf("https://x/%d?rss=KEY", i), fmt.Sprintf("%040x", i),
			1700000000000+int64(i)*1000, 1700000000000+int64(i)*1000)
		if err != nil {
			t.Fatal(err)
		}
	}
	// files is what makes a searchee identifiable when several torrents share
	// a name, so it is seeded as the JSON cross-seed actually stores.
	if _, err := db.Exec(`
		INSERT INTO client_searchee
		  (client_host, info_hash, name, title, files, length, save_path, category)
		VALUES ('10.10.130.5:8080', 'aaa', 'Release.0', 'Release 0',
		        '[{"name":"a.mkv","length":100,"path":"Release.0/a.mkv"}]',
		        100, '/mnt/media/complete', 'tv')`); err != nil {
		t.Fatal(err)
	}

	return New(path)
}

// Info must answer without opening anything: it is the diagnostic you reach for
// when the database is the thing that is wrong.
func TestInfoWithNoDatabase(t *testing.T) {
	got := New("/nonexistent/cross-seed.db").Info()

	if got.Sidecar != "managarr-crossseed" {
		t.Errorf("sidecar = %q", got.Sidecar)
	}
	if got.Source != crossseed.Source {
		t.Errorf("source = %q, want %q", got.Source, crossseed.Source)
	}
	if got.Upstream != "/nonexistent/cross-seed.db" {
		t.Errorf("upstream = %q", got.Upstream)
	}
	// Changes only. cross-seed's webhook fires on injections alone, so a
	// sidecar claiming CapEvents would be promising the matches and none of
	// the rejections — the exact gap this exists to close.
	if len(got.Capabilities) != 1 || got.Capabilities[0] != sidecar.CapChanges {
		t.Errorf("capabilities = %v, want [changes]", got.Capabilities)
	}
}

func TestChanges(t *testing.T) {
	events, err := newService(t, 3).Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	e := events[0]
	if e.Source != crossseed.Source || e.Kind != crossseed.KindDecision {
		t.Errorf("event = %s/%s", e.Source, e.Kind)
	}
	// Ref is the searchee name, not the candidate hash: it is what managarr
	// resolves a release by, and the candidate is frequently a torrent this
	// stack never obtained.
	if e.Ref != "Release.0" {
		t.Errorf("ref = %q, want Release.0", e.Ref)
	}
	if !e.OccurredAt.Equal(time.UnixMilli(1700000000000).UTC()) {
		t.Errorf("occurred_at = %v", e.OccurredAt)
	}
	if e.Cursor == "" {
		t.Error("event carries no cursor, so a walk could not advance")
	}

	// The payload round-trips to the shared type, which is the contract
	// between this sidecar and managarr.
	d, err := sidecar.DecodePayload[crossseed.Decision](e)
	if err != nil {
		t.Fatal(err)
	}
	if d.SearcheeName != "Release.0" || d.Decision != "SIZE_MISMATCH" {
		t.Errorf("payload = %+v", d)
	}
	if d.GUID != "https://x/0?rss=KEY" {
		t.Errorf("guid = %q; redaction is managarr's, not this sidecar's", d.GUID)
	}
}

func TestChangesResumesFromCursor(t *testing.T) {
	svc := newService(t, 5)
	ctx := context.Background()

	first, err := svc.Changes(ctx, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("got %d events, want 2", len(first))
	}

	next, err := svc.Changes(ctx, first[1].Cursor, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 2 || next[0].Ref != "Release.2" {
		t.Errorf("resumed at %+v, want Release.2", next)
	}
}

func TestChangesEmpty(t *testing.T) {
	events, err := newService(t, 0).Changes(context.Background(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
	}
}

func TestChangesBadCursor(t *testing.T) {
	if _, err := newService(t, 1).Changes(context.Background(), "garbage", 10); err == nil {
		t.Fatal("expected an error for a malformed cursor")
	}
}

func TestChangesMissingDatabase(t *testing.T) {
	svc := New(filepath.Join(t.TempDir(), "absent.db"))

	if _, err := svc.Changes(context.Background(), "", 10); err == nil {
		t.Fatal("expected an error")
	}
}

// cross-seed pushes nothing worth having: its webhook fires only on a match.
// Reporting that as unsupported rather than as a failure lets managarr tell
// "this service does not do that" from "this service is broken".
func TestEnrichUnsupported(t *testing.T) {
	_, err := newService(t, 1).Enrich(context.Background(), []byte(`{}`))
	if !errors.Is(err, sidecar.ErrUnsupported) {
		t.Errorf("err = %v, want ErrUnsupported", err)
	}
}

// routes serves the service's extra routes through a real mux, so the patterns
// are exercised rather than the handlers called directly.
func routes(t *testing.T, svc *Service) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	svc.Routes(mux)
	return mux
}

func TestClientTorrentsRoute(t *testing.T) {
	rec := httptest.NewRecorder()
	routes(t, newService(t, 1)).ServeHTTP(rec,
		httptest.NewRequest(http.MethodGet, crossseed.ClientTorrentsPath, http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		ClientTorrents []crossseed.ClientTorrent `json:"clientTorrents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.ClientTorrents) != 1 || body.ClientTorrents[0].InfoHash != "aaa" {
		t.Errorf("client torrents = %+v", body.ClientTorrents)
	}
}

// Empty must be an array, never null: managarr ranges over this to build the
// searchee index, and null would read the same as a failed request.
func TestClientTorrentsEmptyIsAnArray(t *testing.T) {
	svc := newService(t, 0)
	// Drop the seeded row so the table is genuinely empty.
	db, err := sql.Open("sqlite", "file:"+svc.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM client_searchee`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	rec := httptest.NewRecorder()
	routes(t, svc).ServeHTTP(rec,
		httptest.NewRequest(http.MethodGet, crossseed.ClientTorrentsPath, http.NoBody))

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw["clientTorrents"]) != "[]" {
		t.Errorf("clientTorrents = %s, want []", raw["clientTorrents"])
	}
}

func TestStatsRoute(t *testing.T) {
	rec := httptest.NewRecorder()
	routes(t, newService(t, 3)).ServeHTTP(rec,
		httptest.NewRequest(http.MethodGet, crossseed.StatsPath, http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got crossseed.Stats
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := crossseed.Stats{Decisions: 3, Searchees: 3, ClientTorrents: 1}
	if got != want {
		t.Errorf("stats = %+v, want %+v", got, want)
	}
}

// Both extra routes touch the database, so an unreadable one has to be a
// failure rather than a cheerful empty answer.
func TestExtraRoutesFailWithNoDatabase(t *testing.T) {
	h := routes(t, New(filepath.Join(t.TempDir(), "absent.db")))

	for _, path := range []string{crossseed.ClientTorrentsPath, crossseed.StatsPath} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s = %d, want 500", path, rec.Code)
		}
	}
}

// The service must satisfy both halves of the contract it claims.
func TestImplementsContract(t *testing.T) {
	var (
		_ sidecar.Service  = (*Service)(nil)
		_ sidecar.Extender = (*Service)(nil)
	)
}
