// Package webhooks renders a configured Go text/template over an event and POSTs the
// result to the webhook URL with the configured headers. The template sees two fields
// — {{.Type}} (the event name) and {{.Payload}} (a map of event-specific values) — so
// it can pull individual keys ({{.Payload.name}}) or dump the lot ({{toJson .Payload}}).
// A nil Notifier, an empty URL/Template, or an event type not in the enabled set is a
// no-op, so callers never branch on whether notifications are on.
package webhooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"mousetrap/types"
)

// Event is the data a webhook template renders over.
type Event struct {
	Type    types.WebhookEvent
	Payload map[string]any
}

// Notifier renders a template over an Event and POSTs it.
type Notifier struct {
	cfg     types.WebhookConfig
	tmpl    *template.Template
	enabled map[string]bool
	client  *http.Client
}

// funcs exposes JSON encoders to templates: toJson (HTML-escaped, like html contexts)
// and toRawJson (no HTML escaping — the usual choice for a JSON body).
var funcs = template.FuncMap{
	"toJson":    func(v any) (string, error) { b, err := json.Marshal(v); return string(b), err },
	"toRawJson": toRawJSON,
}

func toRawJSON(v any) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return string(bytes.TrimRight(b.Bytes(), "\n")), nil
}

// New builds a Notifier, compiling the template once so a bad template fails at
// startup rather than on the first event.
func New(cfg types.WebhookConfig) (*Notifier, error) {
	n := &Notifier{
		cfg:     cfg,
		enabled: make(map[string]bool, len(cfg.Events)),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
	for _, e := range cfg.Events {
		n.enabled[e] = true
	}
	if cfg.URL != "" && cfg.Template != "" {
		t, err := template.New("webhook").Funcs(funcs).Parse(cfg.Template)
		if err != nil {
			return nil, fmt.Errorf("webhook template: %w", err)
		}
		n.tmpl = t
	}
	return n, nil
}

// Notify renders the template over the event and POSTs it with the configured
// headers. No-op when disabled, no template, or the event type isn't enabled.
// Blocking; returns any error for the caller to log — notifications never affect the
// triggering operation.
func (n *Notifier) Notify(eventType types.WebhookEvent, payload map[string]any) error {
	if n == nil || n.cfg.URL == "" || n.tmpl == nil || !n.enabled[string(eventType)] {
		return nil
	}
	var body bytes.Buffer
	if err := n.tmpl.Execute(&body, Event{Type: eventType, Payload: payload}); err != nil {
		return fmt.Errorf("webhook render (%s): %w", eventType, err)
	}
	req, err := http.NewRequest(http.MethodPost, n.cfg.URL, &body)
	if err != nil {
		return err
	}
	for k, v := range n.cfg.Headers {
		req.Header.Set(k, v)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook post (%s): %w", eventType, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook post (%s): status %d", eventType, resp.StatusCode)
	}
	return nil
}
