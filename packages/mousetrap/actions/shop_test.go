package actions

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"mousetrap/types"
)

// The default (empty) shop config decides hold; the handler just refreshes + 200s.
// The buy decision logic itself is covered by policy's shop tests.
func TestShopHold(t *testing.T) {
	a := testApp(t, types.AcceptConfig{}, types.GrabConfig{})
	req := httptest.NewRequest(http.MethodPost, "/shop", nil)
	w := httptest.NewRecorder()
	Shop(a)(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
