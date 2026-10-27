package sidecar

import (
	"reflect"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics is what a sidecar reports about itself.
//
// Four series, because the questions worth asking from outside are few: is it
// receiving anything, is it relaying anything, is it failing, and how long does
// the upstream take. Everything past that is answerable from managarr's own
// tables, which have the actual data.
type Metrics struct {
	// received counts webhook bodies accepted at /events, before enrichment.
	// Separate from relayed so a sidecar that receives and then drops
	// everything is visible as a gap rather than as silence.
	received *prometheus.CounterVec
	// relayed counts events successfully handed to managarr.
	relayed *prometheus.CounterVec
	// failed counts events that could not be enriched or relayed. Labelled
	// by stage, because "the upstream would not answer" and "managarr would
	// not accept it" are different outages with different fixes.
	failed *prometheus.CounterVec
	// enrichDuration times the callback into the upstream. The slow part of
	// a sidecar is always this, never the relay.
	enrichDuration *prometheus.HistogramVec
}

// NewMetrics registers the collectors. A nil registerer leaves them
// unregistered but still usable, which is what tests want.
func NewMetrics(reg prometheus.Registerer, source string) *Metrics {
	labels := prometheus.Labels{"source": source}

	m := &Metrics{
		received: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "sidecar_events_received_total",
			Help:        "Webhook bodies accepted, before enrichment.",
			ConstLabels: labels,
		}, []string{"kind"}),
		relayed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "sidecar_events_relayed_total",
			Help:        "Events successfully handed to managarr.",
			ConstLabels: labels,
		}, []string{"kind"}),
		failed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "sidecar_events_failed_total",
			Help:        "Events lost, by the stage that lost them.",
			ConstLabels: labels,
		}, []string{"stage"}),
		enrichDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "sidecar_enrich_duration_seconds",
			Help:        "Time spent calling the upstream to fill in a webhook.",
			ConstLabels: labels,
			Buckets:     prometheus.DefBuckets,
		}, []string{"outcome"}),
	}

	if registerable(reg) {
		reg.MustRegister(m.received, m.relayed, m.failed, m.enrichDuration)
	}
	return m
}

// registerable reports whether reg can actually be registered against.
//
// Not the same as reg != nil. A nil *prometheus.Registry stored in a
// Registerer makes the interface non-nil — it carries a type — and
// MustRegister then dereferences it and panics. The plain nil check reads as
// correct and is not, which is exactly the kind of thing that only shows up
// when a sidecar is started without a registry.
func registerable(reg prometheus.Registerer) bool {
	if reg == nil {
		return false
	}
	switch v := reflect.ValueOf(reg); v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func:
		return !v.IsNil()
	default:
		return true
	}
}

// Failure stages, as they appear on sidecar_events_failed_total.
const (
	// StageDecode is a body that was not readable at all.
	StageDecode = "decode"
	// StageEnrich is the upstream refusing or failing to fill it in.
	StageEnrich = "enrich"
	// StageRelay is managarr refusing or being unreachable. The event is
	// gone; the reconcile pass is what recovers it.
	StageRelay = "relay"
)

// Register creates the event series before any event has arrived.
//
// A *Vec publishes nothing until a label value is used, so a sidecar that has
// received no webhook exports no sidecar_* series at all — and "received zero"
// then looks identical to "not scraped" and to "not deployed". An alert on
// events failing cannot fire against a series that does not exist.
//
// Only for a sidecar that actually receives events. Publishing zeros on a
// read-only one would assert a capability it does not have: cross-seed, bazarr,
// beets and slskd have no webhook, and a flat zero on their relay counter would
// read as a push path that is working.
//
// relayed is deliberately absent. Its label is the event kind, which a service
// invents as it goes, so there is no set to pre-create — and unlike the others
// its absence is unambiguous, because received rising while relayed stays
// missing is exactly the signal worth having.
func (m *Metrics) Register() {
	if m == nil {
		return
	}
	m.received.WithLabelValues("").Add(0)
	for _, stage := range []string{StageDecode, StageEnrich, StageRelay} {
		m.failed.WithLabelValues(stage).Add(0)
	}
}
