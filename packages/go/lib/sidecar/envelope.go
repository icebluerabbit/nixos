package sidecar

import (
	"encoding/json"
	"time"
)

// Event is the one shape every sidecar emits, whatever it wrapped.
//
// The point of normalising here rather than in managarr: there are eight
// upstreams and one consumer. Translating at the edge means managarr's ingest
// knows one schema instead of eight, and a new service is a new sidecar rather
// than a new branch in the consumer.
//
// Deliberately thin. A sidecar's job is to say what happened, in whose terms,
// about which identifier — not to decide what any of it means. The correlation,
// the model, and every foreign key stay in managarr, which is the only place
// that can see across services.
type Event struct {
	// Source is the service this came from: "sonarr", "cross-seed", "qbittorrent".
	// Matches the infra client name minus the container- prefix.
	Source string `json:"source"`
	// Kind is what happened, in this sidecar's vocabulary — "grab",
	// "import", "decision". Normalising the vocabulary ACROSS services is
	// managarr's job; a sidecar only has to be consistent with itself.
	Kind string `json:"kind"`
	// OccurredAt is when the upstream says it happened, not when this was
	// received. Zero when the upstream did not say, which is common for
	// webhooks — managarr substitutes receipt time rather than guessing.
	OccurredAt time.Time `json:"occurred_at,omitzero"`
	// Ref is the identifier this event is about, in the upstream's terms: an
	// infohash, a release title, a download id. The handle managarr
	// correlates on, so an event without one is nearly useless.
	Ref string `json:"ref,omitempty"`
	// Payload is the enriched body. Whatever the sidecar could assemble
	// about this event, already redacted.
	Payload json.RawMessage `json:"payload,omitempty"`
	// Cursor is what to pass as ?since= to resume immediately after this
	// event. Opaque: the sidecar picks the encoding and the consumer only
	// stores and returns it.
	//
	// Opaque rather than a timestamp because timestamps tie. cross-seed
	// records first_seen in milliseconds and writes several decisions per
	// millisecond routinely, so a timestamp cursor either re-reads the whole
	// tied group forever or steps past part of it. A cursor that includes
	// the row id cannot do either.
	Cursor string `json:"cursor,omitempty"`
}

// Info describes a sidecar to whoever asks.
//
// Served at /info, and the reason the control plane has something to talk to
// later: a managarr that can enumerate its sidecars and ask each what it can do
// does not need a hardcoded list of them.
type Info struct {
	// Sidecar is this binary's name, e.g. "managarr-crossseed".
	Sidecar string `json:"sidecar"`
	// Source is the service it wraps, matching Event.Source.
	Source string `json:"source"`
	// Upstream is the base URL or path it reads. Diagnostic: the most common
	// sidecar failure is pointing at the wrong thing, and that is invisible
	// from the outside otherwise.
	Upstream string `json:"upstream"`
	// Capabilities lists what this sidecar actually implements —
	// "changes", "events". Absent means the route is wired but answers
	// nothing, which a consumer should be able to discover rather than infer
	// from an empty result.
	Capabilities []string `json:"capabilities"`
}

// Capability names. A sidecar reports the ones it means.
const (
	// CapChanges means /changes returns real data.
	CapChanges = "changes"
	// CapEvents means the service enriches webhooks posted to /events.
	CapEvents = "events"
)
