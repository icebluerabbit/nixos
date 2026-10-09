// Package server wires the HTTP surface onto the application dependencies. Each
// route delegates to an action constructor in the actions package.
package server

import (
	"net/http"

	"homelab/mousetrap/internal/actions"
	"homelab/mousetrap/internal/app"
)

// New builds the HTTP handler for the application.
func New(a *app.App) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.Handle("POST /announce", actions.Announce(a))
	mux.Handle("POST /search", actions.Search(a))
	mux.Handle("POST /refresh/account", actions.Account(a))
	mux.Handle("POST /refresh/freeleech", actions.Freeleech(a))
	mux.Handle("POST /refresh/bonus", actions.Bonus(a))
	mux.Handle("POST /shop", actions.Shop(a))
	mux.Handle("/torznab/api", actions.Torznab(a))
	mux.Handle("/torznab/download", actions.TorznabDownload(a))
	mux.Handle("POST /query", actions.Query(a))
	mux.Handle("GET /metrics", a.Metrics.Handler())

	return mux
}
