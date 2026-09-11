package prune_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"homelab/app"
	"homelab/clients/slskd"
	"homelab/slskd-prune/internal/prune"
)

func testEnv(dryRun bool) *app.Env {
	return &app.Env{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Stdout: io.Discard,
		DryRun: dryRun,
	}
}

// fake is a stand-in slskd holding a search history, recording which ids were
// deleted so a test can assert on the selection rather than only the count.
type fake struct {
	mu       sync.Mutex
	searches []slskd.Search
	deleted  []string
	// failOn makes DELETE fail for these ids, standing in for the 404 a
	// search deleted between the listing and the delete produces.
	failOn map[string]bool
	// listStatus, when non-zero, fails the listing instead of serving it.
	listStatus int
}

func (f *fake) server(t *testing.T) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/searches", func(w http.ResponseWriter, _ *http.Request) {
		if f.listStatus != 0 {
			http.Error(w, "nope", f.listStatus)
			return
		}
		writeSearches(t, w, f.searches)
	})
	mux.HandleFunc("DELETE /api/v0/searches/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.failOn[id] {
			http.Error(w, "gone", http.StatusNotFound)
			return
		}
		f.deleted = append(f.deleted, id)
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// deletedIDs reports what was deleted, sorted: the deletes run concurrently, so
// arrival order carries no meaning and asserting on it would flake.
func (f *fake) deletedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := append([]string(nil), f.deleted...)
	slices.Sort(out)
	return out
}

func writeSearches(t *testing.T, w http.ResponseWriter, searches []slskd.Search) {
	t.Helper()

	var b strings.Builder
	b.WriteString("[")
	for i, s := range searches {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"id":"` + s.ID + `","isComplete":`)
		if s.IsComplete {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString(`,"startedAt":"` + s.StartedAt.Format(time.RFC3339Nano) + `"}`)
	}
	b.WriteString("]")

	w.Header().Set("Content-Type", "application/json")
	if _, err := io.WriteString(w, b.String()); err != nil {
		t.Fatalf("write searches: %v", err)
	}
}

func search(id string, age time.Duration, complete bool) slskd.Search {
	return slskd.Search{
		ID:         id,
		StartedAt:  time.Now().Add(-age).UTC(),
		IsComplete: complete,
	}
}

const day = 24 * time.Hour

func TestStaleSelectsOnlyOldCompleteSearches(t *testing.T) {
	t.Parallel()

	cutoff := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	searches := []slskd.Search{
		{ID: "old", StartedAt: cutoff.Add(-time.Hour), IsComplete: true},
		{ID: "new", StartedAt: cutoff.Add(time.Hour), IsComplete: true},
		{ID: "old-running", StartedAt: cutoff.Add(-time.Hour), IsComplete: false},
		{ID: "exactly-at-cutoff", StartedAt: cutoff, IsComplete: true},
	}

	got := prune.Stale(searches, cutoff)

	want := []string{"old"}
	ids := make([]string, 0, len(got))
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	if diff := cmp.Diff(want, ids); diff != "" {
		t.Errorf("Stale (-want +got):\n%s", diff)
	}
}

func TestStaleOnEmptyHistory(t *testing.T) {
	t.Parallel()

	if got := prune.Stale(nil, time.Now()); len(got) != 0 {
		t.Errorf("Stale(nil) = %v, want empty", got)
	}
}

func TestCutoffSubtractsRetention(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	cfg := prune.Config{RetentionDays: 7}

	want := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if got := cfg.Cutoff(now); !got.Equal(want) {
		t.Errorf("Cutoff = %s, want %s", got, want)
	}
}

// A retention window of zero would put the cutoff at now and take the whole
// history with it, so it has to be refused rather than run.
func TestValidateRejectsUnsafeConfig(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		cfg  prune.Config
		want error
	}{
		"zero retention":     {prune.Config{RetentionDays: 0, Workers: 1}, prune.ErrRetention},
		"negative retention": {prune.Config{RetentionDays: -1, Workers: 1}, prune.ErrRetention},
		"zero workers":       {prune.Config{RetentionDays: 7, Workers: 0}, prune.ErrWorkers},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := tc.cfg.Validate(); !errors.Is(err, tc.want) {
				t.Errorf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDefaultsAreValid(t *testing.T) {
	t.Parallel()

	if err := prune.Defaults().Validate(); err != nil {
		t.Errorf("Defaults().Validate() = %v, want nil", err)
	}
}

func TestRunDeletesOnlyStaleSearches(t *testing.T) {
	t.Parallel()

	f := &fake{searches: []slskd.Search{
		search("stale-1", 30*day, true),
		search("stale-2", 8*day, true),
		search("fresh", 2*day, true),
		search("running", 30*day, false),
	}}
	cfg := prune.Config{Base: f.server(t), RetentionDays: 7, Workers: 4}

	if err := prune.Run(context.Background(), testEnv(false), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"stale-1", "stale-2"}
	if diff := cmp.Diff(want, f.deletedIDs()); diff != "" {
		t.Errorf("deleted (-want +got):\n%s", diff)
	}
}

func TestRunHonoursDryRun(t *testing.T) {
	t.Parallel()

	f := &fake{searches: []slskd.Search{search("stale", 30*day, true)}}
	cfg := prune.Config{Base: f.server(t), RetentionDays: 7, Workers: 4}

	if err := prune.Run(context.Background(), testEnv(true), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := f.deletedIDs(); len(got) != 0 {
		t.Errorf("dry run deleted %v, want nothing", got)
	}
}

// One failed delete must not abandon the rest: the usual cause is a search that
// went away between the listing and the delete, and stopping there would mean a
// backlog that never drains.
func TestRunContinuesPastAFailure(t *testing.T) {
	t.Parallel()

	f := &fake{
		searches: []slskd.Search{
			search("gone", 30*day, true),
			search("stale-1", 30*day, true),
			search("stale-2", 30*day, true),
		},
		failOn: map[string]bool{"gone": true},
	}
	cfg := prune.Config{Base: f.server(t), RetentionDays: 7, Workers: 2}

	err := prune.Run(context.Background(), testEnv(false), cfg)
	if err == nil {
		t.Fatal("Run() = nil, want the failure reported")
	}

	want := []string{"stale-1", "stale-2"}
	if diff := cmp.Diff(want, f.deletedIDs()); diff != "" {
		t.Errorf("deleted (-want +got):\n%s", diff)
	}
}

func TestRunReportsAFailedListing(t *testing.T) {
	t.Parallel()

	f := &fake{listStatus: http.StatusInternalServerError}
	cfg := prune.Config{Base: f.server(t), RetentionDays: 7, Workers: 1}

	if err := prune.Run(context.Background(), testEnv(false), cfg); err == nil {
		t.Error("Run() = nil, want the listing failure reported")
	}
}

func TestRunRejectsUnsafeConfigBeforeCalling(t *testing.T) {
	t.Parallel()

	f := &fake{searches: []slskd.Search{search("stale", 30*day, true)}}
	cfg := prune.Config{Base: f.server(t), RetentionDays: 0, Workers: 1}

	if err := prune.Run(context.Background(), testEnv(false), cfg); !errors.Is(err, prune.ErrRetention) {
		t.Errorf("Run() = %v, want ErrRetention", err)
	}
	if got := f.deletedIDs(); len(got) != 0 {
		t.Errorf("deleted %v despite an invalid config", got)
	}
}

func TestRunOnEmptyHistory(t *testing.T) {
	t.Parallel()

	f := &fake{}
	cfg := prune.Config{Base: f.server(t), RetentionDays: 7, Workers: 1}

	if err := prune.Run(context.Background(), testEnv(false), cfg); err != nil {
		t.Errorf("Run: %v", err)
	}
}

func TestRunRejectsABadBaseURL(t *testing.T) {
	t.Parallel()

	cfg := prune.Config{Base: "", RetentionDays: 7, Workers: 1}

	if err := prune.Run(context.Background(), testEnv(false), cfg); err == nil {
		t.Error("Run() = nil, want the base URL rejected")
	}
}

// A cancelled context stops the run and is reported as such, rather than
// surfacing as thousands of individual delete failures.
func TestRunReportsCancellation(t *testing.T) {
	t.Parallel()

	f := &fake{searches: []slskd.Search{search("stale", 30*day, true)}}
	cfg := prune.Config{Base: f.server(t), RetentionDays: 7, Workers: 1}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := prune.Run(ctx, testEnv(false), cfg); !errors.Is(err, context.Canceled) {
		t.Errorf("Run() = %v, want context.Canceled", err)
	}
}
