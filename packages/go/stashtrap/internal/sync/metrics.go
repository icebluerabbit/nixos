package sync

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// metrics describe the catalogue and the crawls that fill it.
//
// The entity gauges are set after a crawl and once at startup rather than
// computed per scrape: counting 200k scene rows every fifteen seconds to learn
// a number that changes daily is work for nothing.
type metrics struct {
	entities    *prometheus.GaugeVec
	lastSuccess prometheus.Gauge
	duration    prometheus.Gauge
	running     prometheus.Gauge
	failures    prometheus.Counter
	fetched     *prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		entities: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "stashtrap_catalogue_entities",
			Help: "Rows stored per catalogue entity.",
		}, []string{"entity"}),
		lastSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "stashtrap_sync_last_success_timestamp_seconds",
			Help: "When the last crawl finished without error.",
		}),
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "stashtrap_sync_duration_seconds",
			Help: "How long the last completed crawl took.",
		}),
		running: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "stashtrap_sync_running",
			Help: "1 while a crawl is in flight.",
		}),
		failures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "stashtrap_sync_failures_total",
			Help: "Crawls that ended in an error.",
		}),
		// Fetched, not stored: a scene credited to a followed performer at
		// a followed studio is fetched twice and stored once, and the gap
		// between this and the entity gauge is how much the crawl overlaps.
		fetched: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "stashtrap_sync_fetched_total",
			Help: "Items fetched from the stash-box, by entity.",
		}, []string{"entity"}),
	}
	reg.MustRegister(m.entities, m.lastSuccess, m.duration, m.running, m.failures, m.fetched)
	return m
}

// refreshCatalogue reads the stored counts into the entity gauges.
//
// A failure here is logged rather than returned: stale gauges are worth less
// than a crawl, and a counting error should not fail the sync that succeeded.
func (s *Syncer) refreshCatalogue(ctx context.Context) {
	if s.metrics == nil {
		return
	}
	performers, studios, scenes, err := s.store.CatalogueCounts(ctx, s.endpoint)
	if err != nil {
		s.logger.Error("catalogue counts", "error", err)
		return
	}
	s.metrics.entities.WithLabelValues("performers").Set(float64(performers))
	s.metrics.entities.WithLabelValues("studios").Set(float64(studios))
	s.metrics.entities.WithLabelValues("scenes").Set(float64(scenes))
}
