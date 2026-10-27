package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"homelab/httpx"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
	"homelab/managarr/internal/store"
	"homelab/sidecar"
)

// These run against a real PostgreSQL, like the DAO's own tests and for the
// same reason: what is being checked is that the cursor upsert arbitrates on
// the right conflict target and that events_read accumulates rather than being
// overwritten. Both are things Postgres owns.
//
//	MANAGARR_TEST_DSN=postgres://managarr@host:5432/managarr_test?sslmode=disable go test ./...
const testDSNEnv = "MANAGARR_TEST_DSN"

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testDAO opens a connection and reconciles the schema, skipping when no
// database is configured.
func testDAO(t *testing.T) (*dao.DAO, context.Context) {
	t.Helper()
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s not set", testDSNEnv)
	}
	ctx := context.Background()
	logger := discardLogger()

	db, err := store.OpenGorm(ctx, dsn, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(db, logger); err != nil {
		t.Fatal(err)
	}
	return dao.New(db, logger), ctx
}

// uniqueSource keeps repeated runs from colliding: these tests write into a
// shared database and never truncate it.
func uniqueSource(t *testing.T) string {
	t.Helper()
	return "test-" + model.NewID().String()
}

// changesServer serves n synthetic events with real cursor paging, so the walk
// runs against the same semantics a sidecar implements.
func changesServer(t *testing.T, n int, requests *int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests != nil {
			*requests++
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		start := 0
		if since := r.URL.Query().Get("since"); since != "" {
			i, err := strconv.Atoi(since)
			if err != nil {
				http.Error(w, "bad cursor", http.StatusBadRequest)
				return
			}
			start = i + 1
		}

		events := []sidecar.Event{}
		for i := start; i < start+limit && i < n; i++ {
			events = append(events, sidecar.Event{
				Source: "test", Kind: "thing",
				Ref:        fmt.Sprintf("item-%d", i),
				OccurredAt: time.Unix(1700000000+int64(i), 0).UTC(),
				Cursor:     strconv.Itoa(i),
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"events": events})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newWalk(t *testing.T, d *dao.DAO, srv *httptest.Server, source string, pageSize int) *SidecarWalk {
	t.Helper()
	c, err := sidecar.NewClient(httpx.Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return NewSidecarWalk(c, d, discardLogger(), source, pageSize)
}

// readCursor returns the stored row, or false when the source has never been
// walked.
func readCursor(t *testing.T, d *dao.DAO, ctx context.Context, source string) (model.SidecarCursor, bool) {
	t.Helper()
	var row model.SidecarCursor
	err := d.DB().WithContext(ctx).Where("source = ?", source).Take(&row).Error
	if errorIsNotFound(err) {
		return row, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return row, true
}

func TestRunWalksAndStoresACursor(t *testing.T) {
	d, ctx := testDAO(t)
	source := uniqueSource(t)
	w := newWalk(t, d, changesServer(t, 25, nil), source, 10)

	var seen int
	st, err := w.Run(ctx, func(_ context.Context, page []sidecar.Event) error {
		seen += len(page)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.Events != 25 || seen != 25 {
		t.Errorf("read %d events, handler saw %d, want 25 both", st.Events, seen)
	}
	// A first run has nothing to resume from, which is how a caller tells
	// "caught up" from "never started".
	if st.Resumed != "" {
		t.Errorf("resumed = %q, want empty on a first run", st.Resumed)
	}

	row, ok := readCursor(t, d, ctx, source)
	if !ok {
		t.Fatal("no cursor was stored")
	}
	if row.Cursor != "24" {
		t.Errorf("cursor = %q, want 24", row.Cursor)
	}
	if row.EventsRead != 25 {
		t.Errorf("events_read = %d, want 25", row.EventsRead)
	}
	if row.LastEventAt == nil {
		t.Error("last_event_at was not recorded")
	}
}

// The point of storing a cursor: the second pass reads only what is new.
func TestRunResumes(t *testing.T) {
	d, ctx := testDAO(t)
	source := uniqueSource(t)
	srv := changesServer(t, 25, nil)

	if _, err := newWalk(t, d, srv, source, 10).Run(ctx,
		func(context.Context, []sidecar.Event) error { return nil }); err != nil {
		t.Fatal(err)
	}

	var second int
	st, err := newWalk(t, d, srv, source, 10).Run(ctx,
		func(_ context.Context, page []sidecar.Event) error {
			second += len(page)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if second != 0 || st.Events != 0 {
		t.Errorf("second pass read %d events, want 0", st.Events)
	}
	if st.Resumed != "24" {
		t.Errorf("resumed = %q, want 24", st.Resumed)
	}

	// events_read accumulates across passes rather than being overwritten, so
	// a pass that read nothing leaves the total intact.
	row, _ := readCursor(t, d, ctx, source)
	if row.EventsRead != 25 {
		t.Errorf("events_read = %d, want the 25 from the first pass", row.EventsRead)
	}
}

// The rule the whole type exists to enforce. A handler that fails must leave
// the cursor at the start of the page it failed on, so the next pass re-reads
// it — a repeated read is free, a skipped one is permanent.
func TestRunDoesNotAdvancePastAFailedPage(t *testing.T) {
	d, ctx := testDAO(t)
	source := uniqueSource(t)
	w := newWalk(t, d, changesServer(t, 25, nil), source, 10)

	wantErr := errors.New("handler refused")
	pages := 0
	_, err := w.Run(ctx, func(_ context.Context, page []sidecar.Event) error {
		pages++
		if pages == 2 {
			return wantErr
		}
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want the handler's own error", err)
	}

	// The first page committed; the second did not.
	row, ok := readCursor(t, d, ctx, source)
	if !ok {
		t.Fatal("the first page should have committed")
	}
	if row.Cursor != "9" {
		t.Errorf("cursor = %q, want 9 — the end of the page that succeeded", row.Cursor)
	}
	if row.EventsRead != 10 {
		t.Errorf("events_read = %d, want 10", row.EventsRead)
	}
}

func TestRunEmptyUpstream(t *testing.T) {
	d, ctx := testDAO(t)
	source := uniqueSource(t)

	st, err := newWalk(t, d, changesServer(t, 0, nil), source, 10).Run(ctx,
		func(context.Context, []sidecar.Event) error {
			t.Error("handler ran on an empty upstream")
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if st.Events != 0 {
		t.Errorf("events = %d, want 0", st.Events)
	}
	// Nothing read means nothing to remember. A row here would claim a
	// position that was never reached.
	if _, ok := readCursor(t, d, ctx, source); ok {
		t.Error("a cursor was stored for a walk that read nothing")
	}
}

func TestRunUpstreamError(t *testing.T) {
	d, ctx := testDAO(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	_, err := newWalk(t, d, srv, uniqueSource(t), 10).Run(ctx,
		func(context.Context, []sidecar.Event) error { return nil })
	if err == nil {
		t.Fatal("expected an error")
	}
}

// RunAtomic hands everything over once and only then advances, for passes whose
// work straddles the read.
func TestRunAtomic(t *testing.T) {
	d, ctx := testDAO(t)
	source := uniqueSource(t)
	w := newWalk(t, d, changesServer(t, 25, nil), source, 10)

	calls := 0
	var got int
	st, err := w.RunAtomic(ctx, func(_ context.Context, events []sidecar.Event) error {
		calls++
		got = len(events)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Once, with all of them — not once per page.
	if calls != 1 {
		t.Errorf("handler ran %d times, want 1", calls)
	}
	if got != 25 || st.Events != 25 {
		t.Errorf("handler got %d events, stats say %d, want 25", got, st.Events)
	}

	row, _ := readCursor(t, d, ctx, source)
	if row.Cursor != "24" {
		t.Errorf("cursor = %q, want 24", row.Cursor)
	}
}

// A failing handler must leave no cursor at all: nothing was written, so
// nothing was read.
func TestRunAtomicDoesNotAdvanceOnFailure(t *testing.T) {
	d, ctx := testDAO(t)
	source := uniqueSource(t)
	w := newWalk(t, d, changesServer(t, 25, nil), source, 10)

	wantErr := errors.New("handler refused")
	_, err := w.RunAtomic(ctx, func(context.Context, []sidecar.Event) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want the handler's own error", err)
	}
	if _, ok := readCursor(t, d, ctx, source); ok {
		t.Error("a cursor was stored despite the handler failing")
	}
}

// Nothing new must not write. Saving an unchanged cursor every hour would make
// updated_at useless as a signal that a source is still producing.
func TestRunAtomicSkipsAnEmptyRead(t *testing.T) {
	d, ctx := testDAO(t)
	source := uniqueSource(t)
	srv := changesServer(t, 5, nil)

	if _, err := newWalk(t, d, srv, source, 10).RunAtomic(ctx,
		func(context.Context, []sidecar.Event) error { return nil }); err != nil {
		t.Fatal(err)
	}
	before, _ := readCursor(t, d, ctx, source)

	calls := 0
	if _, err := newWalk(t, d, srv, source, 10).RunAtomic(ctx,
		func(context.Context, []sidecar.Event) error {
			calls++
			return nil
		}); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Error("handler ran with nothing new")
	}

	after, _ := readCursor(t, d, ctx, source)
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Error("an empty read rewrote the cursor row")
	}
}

func TestReset(t *testing.T) {
	d, ctx := testDAO(t)
	source := uniqueSource(t)
	srv := changesServer(t, 5, nil)
	w := newWalk(t, d, srv, source, 10)

	if _, err := w.Run(ctx, func(context.Context, []sidecar.Event) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, ok := readCursor(t, d, ctx, source); !ok {
		t.Fatal("no cursor to reset")
	}

	if err := w.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := readCursor(t, d, ctx, source); ok {
		t.Error("the cursor survived a reset")
	}

	// And the next walk starts over.
	st, err := w.Run(ctx, func(context.Context, []sidecar.Event) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if st.Events != 5 {
		t.Errorf("re-walk read %d events, want all 5", st.Events)
	}
}

// Resetting a source that was never walked is a no-op, not an error: it is what
// a reset-everything script does.
func TestResetUnknownSource(t *testing.T) {
	d, ctx := testDAO(t)
	w := newWalk(t, d, changesServer(t, 0, nil), uniqueSource(t), 10)

	if err := w.Reset(ctx); err != nil {
		t.Errorf("resetting an unwalked source errored: %v", err)
	}
}

// Two sources must not share a cursor. The primary key is the source, so this
// is really asking whether the upsert arbitrates on the right column.
func TestCursorsAreIndependentPerSource(t *testing.T) {
	d, ctx := testDAO(t)
	srv := changesServer(t, 25, nil)
	a, b := uniqueSource(t), uniqueSource(t)

	if _, err := newWalk(t, d, srv, a, 10).Run(ctx,
		func(context.Context, []sidecar.Event) error { return nil }); err != nil {
		t.Fatal(err)
	}

	st, err := newWalk(t, d, srv, b, 10).Run(ctx,
		func(context.Context, []sidecar.Event) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if st.Events != 25 {
		t.Errorf("the second source read %d events, want its own full walk", st.Events)
	}
}

// A page whose events carry no timestamp must not blank a real one recorded
// earlier — that is what the COALESCE in the upsert is for.
func TestLastEventAtSurvivesAnUndatedPage(t *testing.T) {
	d, ctx := testDAO(t)
	source := uniqueSource(t)

	dated := changesServer(t, 5, nil)
	if _, err := newWalk(t, d, dated, source, 10).Run(ctx,
		func(context.Context, []sidecar.Event) error { return nil }); err != nil {
		t.Fatal(err)
	}
	before, _ := readCursor(t, d, ctx, source)
	if before.LastEventAt == nil {
		t.Fatal("the first walk recorded no timestamp")
	}

	// A sidecar whose events have no OccurredAt — bazarr's history is one, its
	// timestamp being a display string rather than an instant. One page, then
	// empty: a handler that kept returning the same cursor would never
	// terminate, which is a fake being wrong rather than anything under test.
	undated := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since := r.URL.Query().Get("since")
		if since == "" {
			http.Error(w, "expected a cursor", http.StatusBadRequest)
			return
		}
		events := []sidecar.Event{}
		if since != "99" {
			events = append(events, sidecar.Event{
				Source: "test", Kind: "thing", Ref: "x", Cursor: "99",
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"events": events})
	}))
	t.Cleanup(undated.Close)

	if _, err := newWalk(t, d, undated, source, 1).Run(ctx,
		func(context.Context, []sidecar.Event) error { return nil }); err != nil {
		t.Fatal(err)
	}

	after, _ := readCursor(t, d, ctx, source)
	if after.LastEventAt == nil {
		t.Fatal("an undated page blanked the recorded timestamp")
	}
	if !after.LastEventAt.Equal(*before.LastEventAt) {
		t.Errorf("last_event_at = %v, want the earlier %v",
			after.LastEventAt, before.LastEventAt)
	}
	if after.Cursor != "99" {
		t.Errorf("cursor = %q, want it to have advanced anyway", after.Cursor)
	}
}
