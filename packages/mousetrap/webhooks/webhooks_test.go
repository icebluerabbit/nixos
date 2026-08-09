package webhooks

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"mousetrap/types"
)

func TestDisabledIsNoop(t *testing.T) {
	n, err := New(types.WebhookConfig{}) // no URL
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := n.Notify(types.WebhookGrab, nil); err != nil {
		t.Errorf("no-URL Notify = %v, want nil (no-op)", err)
	}
}

func TestNilNotifierIsNoop(t *testing.T) {
	var n *Notifier
	if err := n.Notify(types.WebhookGrab, nil); err != nil {
		t.Errorf("nil Notify = %v, want nil (no-op)", err)
	}
}

func TestBadTemplateFailsFast(t *testing.T) {
	if _, err := New(types.WebhookConfig{URL: "http://x", Template: "{{ .Type ", Events: []string{"grab"}}); err == nil {
		t.Error("bad template should fail New")
	}
}

func TestNotifyRendersAndGates(t *testing.T) {
	var got string
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		hits++
	}))
	defer srv.Close()

	n, err := New(types.WebhookConfig{
		URL:      srv.URL,
		Template: `{"type":"{{ .Type }}","name":"{{ .Payload.name }}","p":{{ toRawJson .Payload }}}`,
		Headers:  map[string]string{"Content-Type": "application/json"},
		Events:   []string{"grab"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// enabled event → POSTs the rendered body
	if err := n.Notify(types.WebhookGrab, map[string]any{"name": "A Book", "tid": 42}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
	if got != `{"type":"grab","name":"A Book","p":{"name":"A Book","tid":42}}` {
		t.Errorf("body = %s", got)
	}

	// disabled event → no POST
	if err := n.Notify(types.WebhookReject, map[string]any{"name": "x"}); err != nil {
		t.Fatalf("Notify(reject): %v", err)
	}
	if hits != 1 {
		t.Errorf("disabled event fired: hits = %d, want 1", hits)
	}
}
