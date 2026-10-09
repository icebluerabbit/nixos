package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	hlapp "homelab/app"
	"homelab/mousetrap/internal/types"
)

func testEnv() *hlapp.Env {
	return &hlapp.Env{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Metrics: prometheus.NewRegistry(),
		Stdout:  io.Discard,
	}
}

// freePort reserves a port and releases it, so Run binds somewhere nothing
// else is listening.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("split %q: %v", l.Addr(), err)
	}
	return port
}

// Run serves until its context is cancelled, and a cancelled run is a clean
// one: systemd stopping the unit is not a failure.
func TestRunServesUntilCancelled(t *testing.T) {
	cfg := types.Config{Listen: "127.0.0.1", Port: freePort(t), StateDir: t.TempDir()}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- Run(ctx, testEnv(), cfg) }()

	// Poll /health rather than sleeping a fixed interval: the listener is up
	// when it answers.
	client := &http.Client{Timeout: time.Second}
	var resp *http.Response
	var err error
	for range 50 {
		resp, err = client.Get("http://" + cfg.Addr() + "/health")
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		cancel()
		t.Fatalf("GET /health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status = %d, want 200", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() = %v, want nil after cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
}

// A configuration the application cannot be built from fails before the
// listener opens — here a state directory that cannot hold the database.
func TestRunReportsWiringFailure(t *testing.T) {
	cfg := types.Config{
		Listen:   "127.0.0.1",
		Port:     freePort(t),
		StateDir: "/proc/nonexistent-directory",
	}

	if err := Run(context.Background(), testEnv(), cfg); err == nil {
		t.Fatal("Run succeeded with an unusable state directory")
	}
}

// A port already in use is reported rather than silently retried.
func TestRunReportsListenFailure(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("split %q: %v", l.Addr(), err)
	}

	cfg := types.Config{Listen: "127.0.0.1", Port: port, StateDir: t.TempDir()}

	err = Run(context.Background(), testEnv(), cfg)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Run() = %v, want a bind failure", err)
	}
}
