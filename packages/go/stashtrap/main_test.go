package main

import (
	"strings"
	"testing"

	"homelab/stashtrap/internal/server"
)

// The flag names and their mapping onto Config's keys are the one thing in
// main.go that can be wrong, and a typo in either half stays silent until the
// unit runs. --help is the cheapest place to catch it.
func TestFlagsMatchConfig(t *testing.T) {
	t.Parallel()

	cfg := server.Defaults()
	a := newApp(&cfg)

	var out strings.Builder
	a.Stdout = &out
	a.Stderr = &out
	a.Args = []string{"--help"}

	if err := a.Execute(t.Context()); err != nil {
		t.Fatalf("--help: %v", err)
	}

	help := out.String()
	for _, flag := range []string{"--addr", "--db-path", "--shutdown-timeout"} {
		if !strings.Contains(help, flag) {
			t.Errorf("%s missing from --help:\n%s", flag, help)
		}
	}
}

// Defaults have to be usable by a unit that passes no flags at all, which is
// what the container aspect will do for everything except the database path.
func TestDefaults(t *testing.T) {
	t.Parallel()

	cfg := server.Defaults()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("defaults do not validate: %v", err)
	}
	if cfg.Addr == "" || cfg.DBPath == "" || cfg.ShutdownTimeout == 0 {
		t.Fatalf("incomplete defaults: %+v", cfg)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	cases := map[string]server.Config{
		"no addr": {DBPath: "/tmp/x.db"},
		"no db":   {Addr: ":8080"},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate accepted an unusable configuration")
			}
		})
	}
}
