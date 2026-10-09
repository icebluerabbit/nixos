// Package app is the shared dependency container passed to every handler. Keeping
// it in its own package lets both server and actions import it without a cycle.
//
// Consistency rule: the pure engine (policy) and clients (mam, store) never reach
// across or persist on their own. state is a stateful service caching over the mam
// client; the actions layer is the glue that wires everything and owns storage
// writes and the grab serialization (GrabMu).
package app

import (
	"path/filepath"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"homelab/mousetrap/internal/client"
	"homelab/mousetrap/internal/mam"
	"homelab/mousetrap/internal/metrics"
	"homelab/mousetrap/internal/policy"
	"homelab/mousetrap/internal/state"
	"homelab/mousetrap/internal/store"
	"homelab/mousetrap/internal/types"
	"homelab/mousetrap/internal/webhooks"
)

// App is the wired set of dependencies shared by all handlers.
type App struct {
	Cfg     types.Config
	Store   *store.Store
	MAM     *mam.Client
	Client  *client.Client
	State   *state.State
	Metrics *metrics.Metrics
	Policy  *policy.Engine
	Webhook *webhooks.Notifier

	// GrabMu serializes the grab pipeline across actions so concurrent requests
	// can't race the optimistic in-flight accounting.
	GrabMu sync.Mutex
}

// New wires the application dependencies from configuration. It returns an error so
// fallible constructors (webhook template, store open/migrate, policy regexes) fail
// fast.
//
// reg is the Prometheus registry the metrics collector registers on — the one
// lib/app built for the process, so the tool's own /metrics route and the
// framework's endpoint export the same series.
func New(cfg types.Config, reg *prometheus.Registry) (*App, error) {
	wh, err := webhooks.New(cfg.Webhook)
	if err != nil {
		return nil, err
	}
	st, err := store.Open(filepath.Join(cfg.StateDir, "mousetrap.db"))
	if err != nil {
		return nil, err
	}
	pol, err := policy.New(cfg.Accept, cfg.Grab, cfg.Wedge, cfg.Shop)
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	mc := mam.New(cfg.MAM.APIBase, cfg.MAM.SeedPath, st.Session("default"))
	sn := state.New(mc)
	return &App{
		Cfg:     cfg,
		Store:   st,
		MAM:     mc,
		Client:  client.New(cfg.Client),
		State:   sn,
		Metrics: metrics.New(reg, sn, st, cfg.Grab.InFlightWindow),
		Policy:  pol,
		Webhook: wh,
	}, nil
}

// Close releases resources held by the app (currently the store).
func (a *App) Close() error {
	if a.Store != nil {
		return a.Store.Close()
	}
	return nil
}
