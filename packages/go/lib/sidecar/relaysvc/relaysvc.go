// Package relaysvc is the sidecar for a service that only pushes.
//
// unpackerr, seasonpackarr and soularr have no API worth reading — no history,
// no queue, no state that outlives the moment. What they have is a webhook, and
// the payload is this homelab's own shape in every case: unpackerr and soularr
// post what their config templates say, and seasonpackarr posts a pack attempt.
//
// So there is nothing to enrich and nothing to page. What a sidecar still buys
// here is uniformity: managarr ingests one envelope from twelve services rather
// than twelve shapes, and each of these gets a /healthz and an /info like every
// other one, which is what makes the fleet enumerable.
//
// Deliberately does not parse the body. A relay that understood its payload
// would have to be released every time a template changed; this one forwards
// the bytes and lets managarr, which already owns those types, decode them.
package relaysvc

import (
	"context"
	"encoding/json"
	"fmt"

	"homelab/sidecar"
)

// KindWebhook is what every relayed body becomes.
const KindWebhook = "webhook"

// Service relays one service's webhook.
type Service struct {
	// Nothing retrospective: these services keep no history to walk.
	sidecar.Unsupported

	source  string
	name    string
	refKeys []string
}

// New builds a relay for one source.
//
// refKeys are top-level JSON keys to try, in order, for the identifier the
// event is about. managarr can correlate without one — the payload is intact
// either way — but a Ref means the event is routable before anything decodes
// it, which is what lets the envelope stay useful.
func New(source string, refKeys ...string) *Service {
	return &Service{
		source:  source,
		name:    "managarr-" + source,
		refKeys: refKeys,
	}
}

// Info describes this sidecar.
//
// Upstream is empty and that is honest: this one does not read anything, so
// there is no address it could be pointed at wrongly.
func (s *Service) Info() sidecar.Info {
	return sidecar.Info{
		Sidecar:      s.name,
		Source:       s.source,
		Capabilities: []string{sidecar.CapEvents},
	}
}

// Enrich forwards one body, unparsed.
//
// The only thing read out of it is the identifier, and only to fill Ref. A
// malformed body is still an error — an unparseable payload will fail on
// managarr's side too, and failing here says which service sent it.
func (s *Service) Enrich(_ context.Context, body []byte) ([]sidecar.Event, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("relaysvc: %s sent a body that is not a JSON object: %w",
			s.source, err)
	}

	return []sidecar.Event{{
		Source:  s.source,
		Kind:    KindWebhook,
		Ref:     s.ref(fields),
		Payload: json.RawMessage(body),
	}}, nil
}

// ref takes the first configured key that holds a non-empty string.
//
// Only strings. A key whose value is an object or a number is not an
// identifier, and coercing one would put something in Ref that matches nothing.
func (s *Service) ref(fields map[string]json.RawMessage) string {
	for _, key := range s.refKeys {
		raw, ok := fields[key]
		if !ok {
			continue
		}
		var v string
		if err := json.Unmarshal(raw, &v); err == nil && v != "" {
			return v
		}
	}
	return ""
}
