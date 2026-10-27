package reconcile

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestMetricsRunningGaugeBracketsAPass is what makes a stalled pass visible.
//
// A pass that hangs holds its gauge at 1 and nothing else in the system says
// so — which is exactly the hole that left a half-hour backfill indistinguishable
// from a hung one.
func TestMetricsRunningGaugeBracketsAPass(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	done := m.Start("autobrr")
	if got := testutil.ToFloat64(m.running.WithLabelValues("autobrr")); got != 1 {
		t.Fatalf("running = %v during a pass, want 1", got)
	}

	done(nil)
	if got := testutil.ToFloat64(m.running.WithLabelValues("autobrr")); got != 0 {
		t.Errorf("running = %v after a pass, want 0", got)
	}
	if got := testutil.ToFloat64(m.lastSuccess.WithLabelValues("autobrr")); got == 0 {
		t.Error("last success was not stamped")
	}
	if got := testutil.ToFloat64(m.failures.WithLabelValues("autobrr")); got != 0 {
		t.Errorf("failures = %v after a clean pass", got)
	}
}

// TestMetricsFailureIsNotASuccess keeps a broken pass from looking fresh.
//
// last_success is what an alert would watch. Stamping it on a failed pass would
// make a pass that has been erroring for a day report as having just run.
func TestMetricsFailureIsNotASuccess(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.Start("prowlarr")(errors.New("boom"))

	if got := testutil.ToFloat64(m.failures.WithLabelValues("prowlarr")); got != 1 {
		t.Errorf("failures = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.lastSuccess.WithLabelValues("prowlarr")); got != 0 {
		t.Errorf("last success = %v; a failed pass must not stamp it", got)
	}
	if got := testutil.ToFloat64(m.running.WithLabelValues("prowlarr")); got != 0 {
		t.Errorf("running = %v; a failed pass must still clear the gauge", got)
	}
}

// TestMetricsPassesAreSeparate guards the label.
func TestMetricsPassesAreSeparate(t *testing.T) {
	m := NewMetrics(prometheus.NewRegistry())

	m.Start("qbittorrent")(nil)
	m.Start("autobrr")(errors.New("boom"))

	if got := testutil.ToFloat64(m.failures.WithLabelValues("autobrr")); got != 1 {
		t.Errorf("autobrr failures = %v", got)
	}
	if got := testutil.ToFloat64(m.failures.WithLabelValues("qbittorrent")); got != 0 {
		t.Errorf("qbittorrent inherited autobrr's failure: %v", got)
	}
}

// TestMetricsNilIsUsable lets a test or a metrics-less deployment call through.
func TestMetricsNilIsUsable(t *testing.T) {
	var m *Metrics
	m.Start("x")(nil)
	m.Start("x")(errors.New("boom"))

	// And a nil Registerer still yields working collectors.
	live := NewMetrics(nil)
	live.Start("x")(nil)
	if got := testutil.ToFloat64(live.lastSuccess.WithLabelValues("x")); got == 0 {
		t.Error("an unregistered Metrics should still record")
	}
}
