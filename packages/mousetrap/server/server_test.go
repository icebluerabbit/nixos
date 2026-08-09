package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"mousetrap/app"
	"mousetrap/types"
)

func TestHealth(t *testing.T) {
	a, err := app.New(types.Config{StateDir: t.TempDir()})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	defer a.Close()

	h := New(a)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET /health = %d, want 200", w.Code)
	}
}
