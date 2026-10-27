package obs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"homelab/obs"
)

func TestNewRejectsBadConfig(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		cfg  obs.Config
		want error
	}{
		"unknown level":  {cfg: obs.Config{Level: "trace"}, want: obs.ErrBadLevel},
		"unknown format": {cfg: obs.Config{Format: "logfmt"}, want: obs.ErrBadFormat},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := obs.New(tc.cfg); !errors.Is(err, tc.want) {
				t.Fatalf("New() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestLevelFiltering(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		level     string
		wantDebug bool
		wantInfo  bool
	}{
		"default is info": {level: "", wantDebug: false, wantInfo: true},
		"debug":           {level: "debug", wantDebug: true, wantInfo: true},
		"warn":            {level: "warn", wantDebug: false, wantInfo: false},
		"warning alias":   {level: "WARNING", wantDebug: false, wantInfo: false},
		"error":           {level: "error", wantDebug: false, wantInfo: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			o, err := obs.New(obs.Config{Level: tc.level, Out: &buf})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			o.Logger.Debug("dbg")
			o.Logger.Info("inf")

			got := buf.String()
			if strings.Contains(got, "dbg") != tc.wantDebug {
				t.Errorf("debug emitted = %v, want %v (output %q)",
					!tc.wantDebug, tc.wantDebug, got)
			}
			if strings.Contains(got, "inf") != tc.wantInfo {
				t.Errorf("info emitted = %v, want %v (output %q)",
					!tc.wantInfo, tc.wantInfo, got)
			}
		})
	}
}

func TestJSONRecordCarriesIdentity(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	o, err := obs.New(obs.Config{Service: "komga-setup", Version: "1.2.3", Out: &buf})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	o.Logger.Info("hello", "key", "value")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("log record is not JSON: %v (%q)", err, buf.String())
	}

	for field, want := range map[string]any{
		"service": "komga-setup",
		"version": "1.2.3",
		"msg":     "hello",
		"key":     "value",
	} {
		if got := record[field]; got != want {
			t.Errorf("record[%q] = %v, want %v", field, got, want)
		}
	}
}

// Identity attributes are omitted rather than emitted empty, so a tool that
// sets neither does not pay for two empty fields on every line shipped to Loki.
func TestIdentityOmittedWhenUnset(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	o, err := obs.New(obs.Config{Out: &buf})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	o.Logger.Info("hello")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("log record is not JSON: %v", err)
	}
	for _, field := range []string{"service", "version"} {
		if _, ok := record[field]; ok {
			t.Errorf("record contains %q, want it omitted", field)
		}
	}
}

// A bytes.Buffer is not a terminal, so auto must resolve to JSON. This is the
// case that matters in production: under systemd stderr is a pipe, and Vector's
// Loki pipeline needs JSON rather than styled text.
func TestFormatSelection(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		format   string
		wantJSON bool
	}{
		"auto off a terminal is json": {format: "", wantJSON: true},
		"auto explicit":               {format: "auto", wantJSON: true},
		"json forced":                 {format: "json", wantJSON: true},
		"text forced":                 {format: "text", wantJSON: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			o, err := obs.New(obs.Config{Format: tc.format, Out: &buf})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			o.Logger.Info("hello")

			var record map[string]any
			gotJSON := json.Unmarshal(buf.Bytes(), &record) == nil
			if gotJSON != tc.wantJSON {
				t.Errorf("json = %v, want %v (output %q)",
					gotJSON, tc.wantJSON, buf.String())
			}
			if !strings.Contains(buf.String(), "hello") {
				t.Errorf("output %q missing the message", buf.String())
			}
		})
	}
}

// Colour is resolved once and shared, so a tool's product and its logs cannot
// disagree about whether the terminal supports it.
func TestRendererIsAlwaysBuilt(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	o, err := obs.New(obs.Config{Out: &buf})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if o.Renderer == nil {
		t.Fatal("Renderer = nil, want a renderer even off a terminal")
	}
}

// Not parallel: t.Setenv forbids t.Parallel on the test or any parent.
func TestColorModes(t *testing.T) {
	t.Run("rejects an unknown mode", func(t *testing.T) {
		_, err := obs.New(obs.Config{Color: "maybe"})
		if !errors.Is(err, obs.ErrBadColor) {
			t.Fatalf("New() error = %v, want %v", err, obs.ErrBadColor)
		}
	})

	// Styled output off a terminal is plain by default; asking for colour
	// explicitly must still emit escape sequences, which is what makes
	// `just`-wrapped and piped-to-less runs usable.
	t.Run("always emits escapes off a terminal", func(t *testing.T) {
		var buf bytes.Buffer
		o, err := obs.New(obs.Config{Format: "text", Color: "always", Out: &buf})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		o.Logger.Error("boom")

		if !strings.Contains(buf.String(), "\x1b[") {
			t.Errorf("output %q has no escape sequences", buf.String())
		}
	})

	t.Run("never suppresses escapes", func(t *testing.T) {
		var buf bytes.Buffer
		o, err := obs.New(obs.Config{Format: "text", Color: "never", Out: &buf})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		o.Logger.Error("boom")

		if strings.Contains(buf.String(), "\x1b[") {
			t.Errorf("output %q contains escape sequences", buf.String())
		}
	})

	// https://no-color.org — honoured in auto mode only.
	t.Run("NO_COLOR wins in auto mode", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		t.Setenv("CLICOLOR_FORCE", "1")

		var buf bytes.Buffer
		o, err := obs.New(obs.Config{Format: "text", Out: &buf})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		o.Logger.Error("boom")

		if strings.Contains(buf.String(), "\x1b[") {
			t.Errorf("output %q contains escape sequences despite NO_COLOR", buf.String())
		}
	})

	t.Run("NO_COLOR loses to an explicit always", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")

		var buf bytes.Buffer
		o, err := obs.New(obs.Config{Format: "text", Color: "always", Out: &buf})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		o.Logger.Error("boom")

		if !strings.Contains(buf.String(), "\x1b[") {
			t.Errorf("output %q has no escape sequences despite Color=always", buf.String())
		}
	})

	t.Run("CLICOLOR_FORCE enables in auto mode", func(t *testing.T) {
		t.Setenv("CLICOLOR_FORCE", "1")

		var buf bytes.Buffer
		o, err := obs.New(obs.Config{Format: "text", Out: &buf})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		o.Logger.Error("boom")

		if !strings.Contains(buf.String(), "\x1b[") {
			t.Errorf("output %q has no escape sequences despite CLICOLOR_FORCE", buf.String())
		}
	})

	t.Run("CLICOLOR_FORCE=0 is not forcing", func(t *testing.T) {
		t.Setenv("CLICOLOR_FORCE", "0")

		var buf bytes.Buffer
		o, err := obs.New(obs.Config{Format: "text", Out: &buf})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		o.Logger.Error("boom")

		if strings.Contains(buf.String(), "\x1b[") {
			t.Errorf("output %q contains escape sequences", buf.String())
		}
	})
}

func TestMetricsHandlerExposesRuntimeCollectors(t *testing.T) {
	t.Parallel()

	o, err := obs.New(obs.Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	o.MetricsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, metric := range []string{"go_goroutines", "process_start_time_seconds"} {
		if !strings.Contains(body, metric) {
			t.Errorf("metrics output missing %q", metric)
		}
	}
}

func TestMetricsPathDefault(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ configured, want string }{
		"default":    {configured: "", want: "/metrics"},
		"overridden": {configured: "/telemetry", want: "/telemetry"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			o, err := obs.New(obs.Config{MetricsPath: tc.configured})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if got := o.MetricsPath(); got != tc.want {
				t.Errorf("MetricsPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A tool with no metrics endpoint configured must be able to call
// ServeMetrics unconditionally, so main() needs no branch around it.
func TestServeMetricsDisabledReturnsImmediately(t *testing.T) {
	t.Parallel()

	o, err := obs.New(obs.Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- o.ServeMetrics(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeMetrics() error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ServeMetrics() blocked with no MetricsAddr configured")
	}
}

func TestServeMetricsServesAndShutsDown(t *testing.T) {
	t.Parallel()

	addr := freeAddr(t)
	o, err := obs.New(obs.Config{MetricsAddr: addr})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- o.ServeMetrics(ctx) }()

	body := getWithRetry(t, "http://"+addr+"/metrics")
	if !strings.Contains(body, "go_goroutines") {
		t.Errorf("metrics body missing go_goroutines: %q", body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeMetrics() error = %v, want nil after cancel", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ServeMetrics() did not return after context cancellation")
	}
}

// freeAddr reserves a loopback port and releases it, so the server under test
// can bind it. Racy in principle, adequate in a single test binary.
func freeAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return addr
}

// getWithRetry polls until the server goroutine has finished binding.
func getWithRetry(t *testing.T, url string) string {
	t.Helper()

	var lastErr error
	for range 100 {
		body, err := get(url)
		if err != nil {
			lastErr = err
			time.Sleep(20 * time.Millisecond)
			continue
		}
		return body
	}
	t.Fatalf("metrics endpoint never came up: %v", lastErr)
	return ""
}

// get is split out of getWithRetry so the response body is closed on each
// attempt rather than accumulating deferred closes across the retry loop.
func get(url string) (string, error) {
	resp, err := http.Get(url) //nolint:noctx // short-lived test request
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
