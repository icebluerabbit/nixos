// Package sidecar is the shared half of every managarr-* sidecar.
//
// A sidecar runs inside the container of the service it wraps and does two
// things managarr cannot do from outside: it reads what is only reachable from
// in there, and it receives that service's webhook, fills in what the payload
// left out, and relays it in one shape.
//
// The contract is fixed so managarr can eventually enumerate its sidecars
// rather than carry a hardcoded list:
//
//	GET  /healthz   liveness; never touches the upstream
//	GET  /info      what this is, what it wraps, what it can do
//	GET  /changes   retrospective read, with a since-cursor the upstream lacks
//	POST /events    webhook in, enriched and relayed
//
// Everything above is here. What differs per service is the Service interface —
// three methods — so a new sidecar is a small file and a call to Run.
//
// Nothing in this package persists anything. See Relay for why.
package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"homelab/app"
)

// Config is the part of a sidecar's command line this package owns.
type Config struct {
	// Addr is the listen address. Every managarr-* sidecar uses the same
	// port: each container has its own IP, so there is nothing to
	// disambiguate and no per-service arithmetic to get wrong.
	Addr string `json:"addr"`
	// ManagarrURL is where events are relayed. Empty disables relaying
	// entirely, which leaves the read half working — useful for bringing a
	// sidecar up before managarr knows about it.
	ManagarrURL string `json:"managarr_url"`
	// RelayPath overrides managarr's ingest route. Empty means
	// DefaultRelayPath.
	RelayPath string `json:"relay_path"`
	// MaxBodyBytes bounds one webhook body.
	MaxBodyBytes int64 `json:"max_body_bytes"`
	// ShutdownTimeout is the grace period for in-flight requests.
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// DefaultAddr is the listen address every managarr-* sidecar uses.
//
// One port across all of them: each container has its own IP, so the number is
// unambiguous everywhere and there is no per-service arithmetic to get wrong.
const DefaultAddr = ":9910"

// Default bounds.
const (
	// defaultMaxBody caps a webhook body. *arr payloads are a few
	// kilobytes; a megabyte is far past anything legitimate and still small
	// enough that a hostile body cannot matter.
	defaultMaxBody = 1 << 20
	// defaultChangesLimit is what /changes returns without a limit.
	defaultChangesLimit = 1000
	// maxChangesLimit bounds one page. The response is assembled in memory
	// before any of it is sent, so an unbounded limit would let one request
	// ask for an entire history.
	maxChangesLimit = 5000
)

// Defaults is what a sidecar's flags are populated from.
func Defaults() Config {
	return Config{
		Addr:            DefaultAddr,
		MaxBodyBytes:    defaultMaxBody,
		ShutdownTimeout: 10 * time.Second,
	}
}

// Run serves the contract until ctx is cancelled.
func Run(ctx context.Context, env *app.Env, cfg Config, svc Service) error {
	var relay *Relay
	metrics := NewMetrics(env.Metrics, svc.Info().Source)

	// Pre-create the event series when this sidecar can receive events, so
	// "nothing has arrived" is distinguishable from "nothing is scraping".
	if slices.Contains(svc.Info().Capabilities, CapEvents) {
		metrics.Register()
	}

	if cfg.ManagarrURL != "" {
		r, err := NewRelay(
			httpxConfig(cfg.ManagarrURL, env.Logger),
			cfg.RelayPath, env.Logger, metrics,
		)
		if err != nil {
			return fmt.Errorf("sidecar: relay: %w", err)
		}
		relay = r
	} else if slices.Contains(svc.Info().Capabilities, CapEvents) {
		// Worth saying out loud, but only for a sidecar that actually
		// receives webhooks: it looks healthy, answers /changes, and
		// silently discards every one — indistinguishable from the upstream
		// not sending any.
		//
		// A read-only sidecar has nothing to relay, so warning there would
		// be noise on every start and would train the reader to ignore it.
		env.Logger.Warn("no managarr URL; webhooks will be accepted and dropped")
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           Handler(env.Logger, cfg, svc, relay, metrics),
		ReadHeaderTimeout: 10 * time.Second,
	}

	info := svc.Info()
	errCh := make(chan error, 1)
	go func() {
		env.Logger.Info("listening",
			"addr", cfg.Addr, "source", info.Source, "upstream", info.Upstream,
			"capabilities", info.Capabilities)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("sidecar: listen: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		// WithoutCancel, not Background: the deadline has to survive the
		// cancellation that triggered the shutdown, while the request values
		// and tracing on ctx carry through to the drain.
		shutdownCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("sidecar: shutdown: %w", err)
		}
		return nil
	}
}

// Handler builds the contract routes.
//
// Separate from Run so every behaviour worth checking is reachable without
// binding a port. A nil relay accepts and drops.
func Handler(log *slog.Logger, cfg Config, svc Service, relay *Relay, m *Metrics) http.Handler {
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBody
	}
	mux := http.NewServeMux()

	// Liveness. Deliberately answers without touching the upstream: a probe
	// that depends on another service restarts this one for a problem
	// restarting cannot fix.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /info", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, log, svc.Info(), nil)
	})

	mux.HandleFunc("GET /changes", func(w http.ResponseWriter, r *http.Request) {
		limit, err := intParam(r, "limit", defaultChangesLimit)
		if err != nil {
			writeError(w, log, http.StatusBadRequest, err)
			return
		}
		if limit > maxChangesLimit {
			limit = maxChangesLimit
		}

		events, err := svc.Changes(r.Context(), r.URL.Query().Get("since"), limit)
		if errors.Is(err, ErrUnsupported) {
			// 501, not 500. "This upstream keeps no history" is a fact
			// about the service, not a fault, and a consumer should be
			// able to stop asking rather than retry forever.
			writeError(w, log, http.StatusNotImplemented, err)
			return
		}
		if events == nil {
			events = []Event{}
		}
		writeJSON(w, log, map[string]any{"events": events}, err)
	})

	mux.HandleFunc("POST /events", func(w http.ResponseWriter, r *http.Request) {
		handleEvents(w, r, log, cfg, svc, relay, m)
	})

	// Registered last so a service cannot quietly shadow a contract route:
	// ServeMux panics on a duplicate pattern rather than letting the
	// override through.
	if e, ok := svc.(Extender); ok {
		e.Routes(mux)
	}

	return mux
}

// handleEvents takes one webhook, fills it in, and passes it on.
func handleEvents(
	w http.ResponseWriter,
	r *http.Request,
	log *slog.Logger,
	cfg Config,
	svc Service,
	relay *Relay,
	m *Metrics,
) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, cfg.MaxBodyBytes))
	if err != nil {
		m.failed.WithLabelValues(StageDecode).Inc()
		writeError(w, log, http.StatusBadRequest, fmt.Errorf("read body: %w", err))
		return
	}
	m.received.WithLabelValues("").Inc()

	started := time.Now()
	events, err := svc.Enrich(r.Context(), body)
	switch {
	case errors.Is(err, ErrUnsupported):
		m.enrichDuration.WithLabelValues("unsupported").Observe(time.Since(started).Seconds())
		writeError(w, log, http.StatusNotImplemented, err)
		return
	case err != nil:
		m.enrichDuration.WithLabelValues("error").Observe(time.Since(started).Seconds())
		m.failed.WithLabelValues(StageEnrich).Inc()
		writeError(w, log, http.StatusInternalServerError, err)
		return
	}
	m.enrichDuration.WithLabelValues("ok").Observe(time.Since(started).Seconds())

	// No events is a normal outcome, not an empty result: health pings and
	// event kinds this sidecar ignores both land here.
	if len(events) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if relay == nil {
		m.failed.WithLabelValues(StageRelay).Add(float64(len(events)))
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// 202 either way. The sender is an *arr, which does not retry and
	// cannot do anything useful with a failure — and the reconcile is what
	// actually recovers a dropped event. Answering 500 would only make the
	// upstream log an error about something already handled.
	if err := relay.Send(r.Context(), events); err != nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]int{"relayed": len(events)})
}
