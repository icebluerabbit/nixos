package actions

import (
	"encoding/json"
	"io"
	"net/http"

	"homelab/mousetrap/internal/app"
)

// Query runs the SQL in the request body against the store (announces, grabs, and
// the other ledger tables) and returns the rows as JSON.
func Query(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		rows, err := a.Store.Query(r.Context(), string(q))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rows)
	}
}
