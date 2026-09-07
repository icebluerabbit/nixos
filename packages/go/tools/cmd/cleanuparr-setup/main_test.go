package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"homelab/app"
	"homelab/tools/internal/cleanuparrsetup"
)

// run drives the command with args, replacing the work so the test asserts on
// configuration alone — the setup path itself is covered in the internal
// package.
func run(t *testing.T, cfg *cleanuparrsetup.Config, args ...string) {
	t.Helper()

	a := newApp(cfg)
	a.Args = args
	a.Stdout = &bytes.Buffer{}
	a.Stderr = &bytes.Buffer{}
	a.Run = func(context.Context, *app.Env) error { return nil }

	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute(%v) error = %v", args, err)
	}
}

// Every flag must reach its Config field. lib/app proves the general mapping
// from a flag name to a config key; what it cannot know is whether this tool's
// flag names match this tool's json tags. A mismatch is silent — the flag
// parses, the field stays at its default, and the tool misbehaves at runtime.
func TestFileFlagsReachConfig(t *testing.T) {
	t.Parallel()

	userFile := writeTemp(t, "admin\n")
	passwordFile := writeTemp(t, "hunter2\n")

	cfg := cleanuparrsetup.Defaults()
	run(t, &cfg,
		"--base", "http://cleanuparr.test:11011",
		"--key-file", "/run/cleanuparr/elsewhere.env",
		"--admin-user-file", userFile,
		"--admin-password-file", passwordFile,
	)

	want := cleanuparrsetup.Config{
		Base:              "http://cleanuparr.test:11011",
		KeyFile:           "/run/cleanuparr/elsewhere.env",
		AdminUserFile:     userFile,
		AdminPasswordFile: passwordFile,
	}
	if cfg != want {
		t.Errorf("config = %+v, want %+v", cfg, want)
	}
}

// The literal half of each pair maps too.
func TestLiteralFlagsReachConfig(t *testing.T) {
	t.Parallel()

	cfg := cleanuparrsetup.Defaults()
	run(t, &cfg, "--admin-user", "admin", "--admin-password", "hunter2")

	if cfg.AdminUser != "admin" {
		t.Errorf("AdminUser = %q, want admin", cfg.AdminUser)
	}
	if cfg.AdminPassword != "hunter2" {
		t.Errorf("AdminPassword = %q, want hunter2", cfg.AdminPassword)
	}
}

// Defaults survive when a flag is not given, so the container unit can pass
// only the flags it cares about — which is exactly what it does.
func TestUnsetFlagsKeepDefaults(t *testing.T) {
	t.Parallel()

	cfg := cleanuparrsetup.Defaults()
	defaults := cleanuparrsetup.Defaults()

	run(t, &cfg, "--admin-user", "admin", "--admin-password", "hunter2")

	if cfg.Base != defaults.Base {
		t.Errorf("Base = %q, want the default %q", cfg.Base, defaults.Base)
	}
	if cfg.KeyFile != defaults.KeyFile {
		t.Errorf("KeyFile = %q, want the default %q", cfg.KeyFile, defaults.KeyFile)
	}
}

// The credentials the flags carry must survive the round trip into the
// resolved pair, which is the whole point of the wiring.
func TestFlagsResolveToCredentials(t *testing.T) {
	t.Parallel()

	passwordFile := writeTemp(t, "hunter2\n")

	cfg := cleanuparrsetup.Defaults()
	run(t, &cfg, "--admin-user", "admin", "--admin-password-file", passwordFile)

	creds, err := cfg.Credentials()
	if err != nil {
		t.Fatalf("Credentials() error = %v", err)
	}
	if creds.Username != "admin" {
		t.Errorf("Username = %q, want admin", creds.Username)
	}
	if got := creds.Password.Reveal(); got != "hunter2" {
		t.Errorf("Password = %q, want hunter2", got)
	}
}

func writeTemp(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
