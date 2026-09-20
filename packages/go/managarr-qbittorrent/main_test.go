package main

import (
	"context"
	"strings"
	"testing"
)

// The test this file exists for.
//
// newApp is split out of main so this can drive it, and the one thing here that
// can be wrong is the flag names and their mapping onto config's keys. That
// mapping broke exactly once and in a way nothing else would have caught: the
// embedded sidecar.Config arrives as a field literally named "Config", which
// collides with the framework's own --config flag — a file path, a string — and
// every start died with "'Config' expected a map or struct, got string".
//
// It reached a deploy because no test ever loaded this binary's configuration.
func TestFlagsMapOntoConfig(t *testing.T) {
	cfg := defaults()
	a := newApp(&cfg)

	// Exercise config loading without starting a listener: port 0 binds, and
	// a cancelled context returns from Run immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a.Args = []string{
		"--addr", "127.0.0.1:0",
		"--managarr-url", "http://managarr.example:8080",
		"--upstream-url", "http://127.0.0.1:8080",
	}

	// The run may fail for its own reasons — a missing API key, an upstream
	// that is not there. What matters is that configuration parsed, so a
	// config error is the failure this asserts against.
	err := a.Execute(ctx)
	if err != nil && strings.Contains(err.Error(), "config:") {
		t.Fatalf("configuration did not load: %v", err)
	}

	if cfg.Addr != "127.0.0.1:0" {
		t.Errorf("Addr = %q, want the flag value", cfg.Addr)
	}
	if cfg.ManagarrURL != "http://managarr.example:8080" {
		t.Errorf("ManagarrURL = %q, want the flag value", cfg.ManagarrURL)
	}
	if cfg.UpstreamURL != "http://127.0.0.1:8080" {
		t.Errorf("cfg.UpstreamURL = %q, want the flag value", cfg.UpstreamURL)
	}
}

// Defaults have to be usable on their own: every one of these runs with only
// the flags the Nix unit passes, and anything unset falls back to here.
func TestDefaults(t *testing.T) {
	cfg := defaults()

	if cfg.Addr == "" {
		t.Error("Addr has no default; the unit does not always pass one")
	}
	if cfg.ShutdownTimeout <= 0 {
		t.Errorf("ShutdownTimeout = %v, want a positive default", cfg.ShutdownTimeout)
	}
	if cfg.MaxBodyBytes <= 0 {
		t.Errorf("MaxBodyBytes = %v, want a positive default", cfg.MaxBodyBytes)
	}
}
