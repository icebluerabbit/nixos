package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"homelab/mousetrap/internal/app"
	"homelab/mousetrap/internal/types"
)

func TestHealth(t *testing.T) {
	a, err := app.New(types.Config{StateDir: t.TempDir()}, prometheus.NewRegistry())
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	defer a.Close()

	h := New(a)
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /health = %d, want 200", w.Code)
	}
}
