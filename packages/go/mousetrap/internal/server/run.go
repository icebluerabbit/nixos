package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	hlapp "homelab/app"
	"homelab/mousetrap/internal/app"
	"homelab/mousetrap/internal/types"
)

// readHeaderTimeout bounds how long a client may take to send its request
// headers. Bodies are not bounded: a grab holds its request open while the
// torrent is fetched from MAM and handed to the download client.
const readHeaderTimeout = 30 * time.Second

// shutdownTimeout is how long in-flight requests have to finish after a
// signal. A grab is capped at 15s by its own context, so this clears one.
const shutdownTimeout = 20 * time.Second

// Run wires the application and serves it until ctx is cancelled.
//
// slog's default logger is replaced with the framework's because mousetrap's
// packages log through the package-level slog functions; this is the one place
// that knows about both.
func Run(ctx context.Context, env *hlapp.Env, cfg types.Config) error {
	slog.SetDefault(env.Logger)

	a, err := app.New(cfg, env.Metrics)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := a.Close(); cerr != nil {
			env.Logger.Error("close", slog.Any("error", cerr))
		}
	}()

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           New(a),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	// Serve and shutdown race on one channel: whichever happens first decides
	// the return, and the signal path always gets its grace period.
	errc := make(chan error, 1)
	go func() {
		env.Logger.Info("mousetrap listening", slog.String("addr", cfg.Addr()))
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
