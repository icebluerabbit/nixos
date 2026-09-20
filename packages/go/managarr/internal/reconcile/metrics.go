package reconcile

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics reports what the reconcile is doing, per pass.
//
// Until this existed a run was a black box: each pass logged once, on
// completion, so a backfill that takes half an hour was indistinguishable from
// a hung one for that whole half hour. Process CPU was the only evidence it was
// alive, which is not an answer to "how far through is it".
//
// Deliberately per PASS rather than per pass-and-instance: the question is
// always "did the *arr side run and how long did it take", and a label per
// Sonarr/Radarr/Lidarr would triple the series for nothing. The instance is in
// the log line.
type Metrics struct {
	duration    *prometheus.HistogramVec
	lastSuccess *prometheus.GaugeVec
	failures    *prometheus.CounterVec
	running     *prometheus.GaugeVec
}

// NewMetrics registers the collectors.
//
// A nil Registerer yields a usable value whose methods do nothing, so a caller
// without metrics — a test — does not have to special-case them.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "managarr_reconcile_duration_seconds",
			Help: "How long each reconcile pass took.",
			// A pass runs from under a second (rips) to tens of minutes
			// (qBittorrent over NFS), so the default buckets are useless here.
			Buckets: []float64{1, 5, 15, 60, 180, 600, 1800, 3600},
		}, []string{"pass"}),

		lastSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "managarr_reconcile_last_success_timestamp_seconds",
			Help: "When each pass last completed without error.",
		}, []string{"pass"}),

		failures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "managarr_reconcile_failures_total",
			Help: "Reconcile passes that returned an error.",
		}, []string{"pass"}),

		running: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "managarr_reconcile_running",
			Help: "1 while a pass is executing, 0 otherwise.",
		}, []string{"pass"}),
	}

	if reg != nil {
		reg.MustRegister(m.duration, m.lastSuccess, m.failures, m.running)
	}
	return m
}

// Start marks a pass as running and returns the function that ends it.
//
// The running gauge is what makes a stalled pass visible: a pass stuck for an
// hour holds its gauge at 1, and nothing else in the system says so.
func (m *Metrics) Start(pass string) func(error) {
	if m == nil {
		return func(error) {}
	}
	m.running.WithLabelValues(pass).Set(1)
	started := time.Now()

	return func(err error) {
		m.running.WithLabelValues(pass).Set(0)
		m.duration.WithLabelValues(pass).Observe(time.Since(started).Seconds())
		if err != nil {
			m.failures.WithLabelValues(pass).Inc()
			return
		}
		m.lastSuccess.WithLabelValues(pass).SetToCurrentTime()
	}
}

// Register creates the series for a pass before it has ever run.
//
// A *Vec publishes nothing until a label value is used, so on a fresh process
// none of these exist — and an alert on "no successful pass in three hours"
// cannot fire on a series that is absent. That is the failure mode worth
// avoiding: managarr restarts on every deploy, and the window where it looks
// healthy because it is reporting nothing is exactly the window after a change.
//
// last_success is deliberately NOT pre-set to a time. Zero means "never
// succeeded", which is true, and stamping it with the process start would claim
// a success that did not happen.
func (m *Metrics) Register(passes ...string) {
	if m == nil {
		return
	}
	for _, p := range passes {
		m.running.WithLabelValues(p).Set(0)
		m.failures.WithLabelValues(p).Add(0)
		m.lastSuccess.WithLabelValues(p).Set(0)
	}
}
