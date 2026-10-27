package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"homelab/httpx"
)

// Client reads any managarr-* sidecar.
//
// One client for every sidecar, which is the payoff for fixing the contract:
// adding managarr-sonarr later needs a new sidecar binary and no new consumer
// code at all.
//
// It lives beside the server half deliberately. A client in its own package
// drifts from the routes it calls the first time either changes; here they fail
// to compile together.
type Client struct {
	http *httpx.Client
}

// NewClient builds a client against a sidecar's base URL.
func NewClient(cfg httpx.Config) (*Client, error) {
	c, err := httpx.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{http: c}, nil
}

// Info reports what the sidecar is and what it can do.
func (c *Client) Info(ctx context.Context) (Info, error) {
	var out Info
	if err := c.http.Do(ctx, http.MethodGet, "/info", nil, &out); err != nil {
		return out, fmt.Errorf("sidecar: info: %w", err)
	}
	return out, nil
}

// maxChangePages bounds a walk.
//
// A sidecar that keeps returning full pages without advancing its cursor would
// otherwise loop forever. At the default page size this is far past any real
// history, so hitting it means the cursor is broken, not that the data is big.
const maxChangePages = 1000

// Changes walks every event after cursor, calling fn per page.
//
// Streamed rather than returned whole: a full cross-seed history is tens of
// thousands of events and the caller writes each and forgets it.
//
// fn returning false stops the walk. The returned cursor is where to resume —
// valid whether the walk finished, was stopped, or found nothing, so a caller
// that stores it never re-reads what it already has.
func (c *Client) Changes(ctx context.Context, cursor string, pageSize int, fn func([]Event) bool) (string, error) {
	if pageSize <= 0 {
		pageSize = defaultChangesLimit
	}

	for page := 0; page < maxChangePages; page++ {
		q := url.Values{}
		q.Set("limit", fmt.Sprint(pageSize))
		if cursor != "" {
			q.Set("since", cursor)
		}

		var out struct {
			Events []Event `json:"events"`
		}
		if err := c.http.Do(ctx, http.MethodGet, "/changes?"+q.Encode(), nil, &out); err != nil {
			return cursor, fmt.Errorf("sidecar: changes at %q: %w", cursor, err)
		}
		if len(out.Events) == 0 {
			return cursor, nil
		}

		next := out.Events[len(out.Events)-1].Cursor
		// A page that does not advance the cursor would be re-fetched
		// forever. Better to stop and say so than to spin.
		if next == "" {
			return cursor, fmt.Errorf("sidecar: page ending at %q returned no cursor", cursor)
		}

		if !fn(out.Events) {
			return next, nil
		}
		cursor = next

		// A short page is the last one.
		if len(out.Events) < pageSize {
			return cursor, nil
		}
	}
	return cursor, fmt.Errorf("sidecar: changes exceeded %d pages", maxChangePages)
}

// Get reads a service-specific route, for the data that is not events.
//
// The escape hatch matching Extender on the server side. Anything reachable
// this way is by definition not portable across sidecars, so a caller using it
// is coupled to that one service — which is the point, and worth it only when
// the data genuinely has no event shape.
func (c *Client) Get(ctx context.Context, path string, out any) error {
	if err := c.http.Do(ctx, http.MethodGet, path, nil, out); err != nil {
		return fmt.Errorf("sidecar: get %s: %w", path, err)
	}
	return nil
}

// DecodePayload unmarshals an event's payload into the upstream's own type.
//
// The envelope carries the native object rather than flattening it, so a
// consumer that knows the service gets everything the sidecar saw, and one that
// does not can still route on Source, Kind and Ref.
func DecodePayload[T any](e Event) (T, error) {
	var out T
	if len(e.Payload) == 0 {
		return out, fmt.Errorf("sidecar: %s/%s event has no payload", e.Source, e.Kind)
	}
	if err := json.Unmarshal(e.Payload, &out); err != nil {
		return out, fmt.Errorf("sidecar: decode %s/%s payload: %w", e.Source, e.Kind, err)
	}
	return out, nil
}
