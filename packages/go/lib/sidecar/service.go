package sidecar

import (
	"context"
	"errors"
	"net/http"
)

// Service is what a sidecar implements. Everything else in this package is
// shared, so a new sidecar is these three methods plus a call to Run.
//
// Kept to three on purpose. The moment this grows a method per upstream quirk,
// every sidecar carries every other sidecar's special case.
type Service interface {
	// Info describes this sidecar. Must not block or touch the upstream:
	// it answers /info, which is a diagnostic and has to work when the
	// upstream does not.
	Info() Info

	// Changes returns at most limit events positioned after cursor. An empty
	// cursor starts the walk; every returned event carries the cursor that
	// resumes after it.
	//
	// This is the data plane read, and the reason a sidecar is more than a
	// proxy: most upstreams have no since-filter, so the sidecar
	// synthesizes one. Without it every reconcile re-walks full history.
	//
	// The ORDER is the sidecar's own, not necessarily oldest-first. Most
	// upstreams here page an append-only log ascending, where an offset is
	// stable; slskd serves its event log newest-first with no total to count
	// back from, and walking it in its native direction means an arriving
	// event re-appears rather than being skipped. A duplicate costs an
	// idempotent upsert; a skip costs the record.
	//
	// What a sidecar MUST guarantee is that following the cursor eventually
	// covers the whole log and then returns nothing.
	//
	// Return ErrUnsupported when the upstream has nothing retrospective.
	Changes(ctx context.Context, cursor string, limit int) ([]Event, error)

	// Enrich turns one raw webhook body into events.
	//
	// Returning several is normal: a single *arr import webhook covers
	// every episode in a season pack, and collapsing them to one would
	// lose exactly the per-file detail managarr wants.
	//
	// Returning none is also normal — a health-check ping, an event kind
	// this sidecar does not care about. Not an error.
	//
	// Return ErrUnsupported when the upstream sends no webhooks.
	Enrich(ctx context.Context, body []byte) ([]Event, error)
}

// ErrUnsupported reports a capability this service does not have.
//
// Distinct from a failure: /changes on a sidecar wrapping something with no
// history is a 501, not a 500, and a consumer should be able to tell "this
// cannot answer" from "this broke".
var ErrUnsupported = errors.New("sidecar: unsupported by this service")

// Unsupported implements the optional half of Service.
//
// Embed it and a service only writes the methods it actually has. Most
// sidecars are one or the other: cross-seed has history and no webhook, a
// *arr has both, beets has neither.
type Unsupported struct{}

// Changes reports that this service has nothing retrospective.
func (Unsupported) Changes(context.Context, string, int) ([]Event, error) {
	return nil, ErrUnsupported
}

// Enrich reports that this service receives no webhooks.
func (Unsupported) Enrich(context.Context, []byte) ([]Event, error) {
	return nil, ErrUnsupported
}

// Extender is the optional half: a service with routes beyond the contract.
//
// The contract is four routes and stays four routes, because managarr has to be
// able to treat every sidecar identically. But some upstreams hold something
// with no generic equivalent — cross-seed's name-to-infohash map is a lookup
// table, not a stream of events, and forcing it through /changes would be a
// worse lie than giving it its own route.
//
// The rule for reaching for this: the data is not events. If it is events,
// it belongs in Changes where every consumer can already read it.
type Extender interface {
	Routes(mux *http.ServeMux)
}
