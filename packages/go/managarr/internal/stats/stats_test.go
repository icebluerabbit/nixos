package stats

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gorm.io/gorm"

	"homelab/managarr/internal/model"
	"homelab/managarr/internal/store"
)

// These run against a real PostgreSQL, like the DAO's own tests and for the
// same reason: what is under test is the SQL. Every query here names columns
// and tables by hand, and the failure mode — a renamed column — is invisible to
// anything but the database.
//
//	MANAGARR_TEST_DSN=postgres://managarr@host:5432/managarr_test?sslmode=disable go test ./...
const testDSNEnv = "MANAGARR_TEST_DSN"

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testDB(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s not set", testDSNEnv)
	}
	ctx := context.Background()
	db, err := store.OpenGorm(ctx, dsn, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(db, discardLogger()); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

// The test that matters, and the one that would have caught the Bazarr routes
// months earlier had it existed for that client.
//
// Every query names tables and columns by hand. A column renamed in a model is
// a schema change Postgres knows about and Go does not, so the only way to
// learn that a metric has silently stopped reporting is to run the SQL.
//
// Asserts on failures rather than on values: the shared test database holds
// whatever previous runs left, so the counts are not predictable — but a query
// that cannot execute is unambiguous.
func TestEveryQueryExecutes(t *testing.T) {
	db, ctx := testDB(t)
	reg := prometheus.NewRegistry()
	c := New(db, discardLogger(), reg)

	c.Refresh(ctx)

	for _, q := range queries {
		if n := testutil.ToFloat64(c.failures.WithLabelValues(q.name)); n != 0 {
			t.Errorf("%s: query failed — its SQL no longer matches the schema", q.name)
		}
	}
}

// A refresh in which every query succeeded stamps the timestamp. A refresh with
// any failure does not: a dashboard showing stale numbers must be able to tell
// that they are stale.
func TestRefreshStampsOnlyOnFullSuccess(t *testing.T) {
	db, ctx := testDB(t)
	c := New(db, discardLogger(), prometheus.NewRegistry())

	c.Refresh(ctx)
	if got := testutil.ToFloat64(c.refreshed); got == 0 {
		t.Fatal("a clean refresh did not stamp the timestamp")
	}

	// Break one query and confirm the stamp stops advancing.
	before := testutil.ToFloat64(c.refreshed)
	c.gauges["managarr_broken"] = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "managarr_broken", Help: "x"}, []string{"l"})
	queries = append(queries, query{
		name: "managarr_broken", labels: []string{"l"},
		sql: "SELECT no_such_column FROM no_such_table",
	})
	t.Cleanup(func() { queries = queries[:len(queries)-1] })

	c.Refresh(ctx)
	if got := testutil.ToFloat64(c.refreshed); got != before {
		t.Error("a failed query still stamped the refresh as complete")
	}
	if n := testutil.ToFloat64(c.failures.WithLabelValues("managarr_broken")); n != 1 {
		t.Errorf("failure counter = %v, want 1", n)
	}
}

// One failing query must not abandon the others. They are independent, and a
// column renamed in one table should cost that metric rather than the whole
// dashboard.
func TestOneFailureDoesNotStopTheRest(t *testing.T) {
	db, ctx := testDB(t)
	c := New(db, discardLogger(), prometheus.NewRegistry())

	c.gauges["managarr_broken"] = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "managarr_broken", Help: "x"}, []string{"l"})
	// Prepended, so everything real runs after it.
	queries = append([]query{{
		name: "managarr_broken", labels: []string{"l"},
		sql: "SELECT no_such_column FROM no_such_table",
	}}, queries...)
	t.Cleanup(func() { queries = queries[1:] })

	c.Refresh(ctx)

	for _, q := range queries[1:] {
		if n := testutil.ToFloat64(c.failures.WithLabelValues(q.name)); n != 0 {
			t.Errorf("%s did not run after an earlier query failed", q.name)
		}
	}
}

// Series for label combinations that no longer exist must disappear.
//
// Without the Reset, the last torrent leaving a state — or an indexer being
// removed — leaves its final count published forever, and a dashboard reads it
// as current. That is worse than a gap: a gap is visibly missing.
func TestRefreshRetiresStaleSeries(t *testing.T) {
	db, ctx := testDB(t)
	c := New(db, discardLogger(), prometheus.NewRegistry())

	// A value no query will produce.
	c.gauges["managarr_contents"].WithLabelValues("nonexistent-kind").Set(42)

	c.Refresh(ctx)

	if n := testutil.CollectAndCount(c.gauges["managarr_contents"]); n == 0 {
		t.Skip("no contents rows in the test database")
	}
	got := testutil.ToFloat64(c.gauges["managarr_contents"].WithLabelValues("nonexistent-kind"))
	if got == 42 {
		t.Error("a series that no longer exists kept its last value")
	}
}

// Label and value counts have to line up or the scan silently misreads a
// column. Checked statically because the mistake is in the table, not the data.
func TestQueryShapesAreConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, q := range queries {
		if q.name == "" || q.help == "" {
			t.Errorf("%q: name and help are both required", q.name)
		}
		if seen[q.name] {
			t.Errorf("%s: declared twice", q.name)
		}
		seen[q.name] = true

		if len(q.labels) == 0 {
			t.Errorf("%s: has no labels; a bare gauge does not need this machinery", q.name)
		}
		if !strings.HasPrefix(q.name, "managarr_") {
			t.Errorf("%s: every metric here is managarr's", q.name)
		}
		if !strings.Contains(strings.ToUpper(q.sql), "SELECT") {
			t.Errorf("%s: sql does not look like a query", q.name)
		}
	}
}

// Registration must not collide. Two metrics with one name is a panic at
// startup, which is a bad way to find out.
func TestNewRegistersEveryMetric(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := New(nil, discardLogger(), reg)

	if len(c.gauges) != len(queries) {
		t.Errorf("built %d gauges for %d queries", len(c.gauges), len(queries))
	}
	// Re-registering the same collectors must fail, which proves the first
	// registration actually happened.
	if err := reg.Register(c.refreshed); err == nil {
		t.Error("the refreshed gauge was never registered")
	}
}

// A nil registerer yields a usable collector, so a caller without metrics does
// not have to special-case them.
func TestNewWithoutRegistry(t *testing.T) {
	c := New(nil, discardLogger(), nil)

	if c == nil || len(c.gauges) == 0 {
		t.Fatal("expected a usable collector")
	}
	c.gauges["managarr_contents"].WithLabelValues("movie").Set(1)
}

func TestModelIsRegistered(t *testing.T) {
	// sidecar_cursors is queried by two metrics here; if it ever leaves the
	// migration list those go silent rather than erroring.
	var found bool
	for _, m := range store.Models {
		if _, ok := m.(*model.SidecarCursor); ok {
			found = true
		}
	}
	if !found {
		t.Error("sidecar_cursors is not in the migration list but is queried here")
	}
}
