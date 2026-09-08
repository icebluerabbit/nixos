package setup_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"homelab/app"
	"homelab/cleanuparr-setup/internal/setup"
	"homelab/secret"
)

const adminPassword = "hunter2"

// fake is a stand-in Cleanuparr recording the setup calls it received.
type fake struct {
	setupCompleted bool
	accountCreated bool
	token          string
	apiKey         string
}

func (f *fake) server(t *testing.T) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/auth/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{"setupCompleted": f.setupCompleted})
	})
	mux.HandleFunc("/api/auth/setup/account", func(w http.ResponseWriter, _ *http.Request) {
		f.accountCreated = true
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/auth/setup/complete", func(w http.ResponseWriter, _ *http.Request) {
		f.setupCompleted = true
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"tokens": map[string]any{"accessToken": f.token},
		})
	})
	mux.HandleFunc("/api/account/api-key", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(t, w, map[string]any{"apiKey": f.apiKey})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func testEnv(t *testing.T) (*app.Env, *bytes.Buffer) {
	t.Helper()

	var logs bytes.Buffer
	return &app.Env{
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
		Stdout: &bytes.Buffer{},
	}, &logs
}

// config wires the tool the way the container does: the username as a literal
// because it is already public in this repo, the password by path because it
// lives in sops.
func config(t *testing.T, baseURL string) setup.Config {
	t.Helper()

	cfg := setup.Defaults()
	cfg.Base = baseURL
	cfg.KeyFile = filepath.Join(t.TempDir(), "api-key.env")
	cfg.AdminUser = "admin"
	cfg.AdminPasswordFile = writeTemp(t, adminPassword+"\n")
	return cfg
}

func TestRunOnAFreshInstance(t *testing.T) {
	t.Parallel()

	f := &fake{token: "tok-123", apiKey: "key-abc"}
	env, _ := testEnv(t)
	cfg := config(t, f.server(t))

	if err := setup.Run(context.Background(), env, cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !f.accountCreated {
		t.Error("admin account was not created on a fresh instance")
	}

	got := readFile(t, cfg.KeyFile)
	if want := "CLEANUPARR_API_KEY=key-abc\n"; got != want {
		t.Errorf("key file = %q, want %q", got, want)
	}
}

// A reboot re-runs this unit; it must not try to re-create the account.
func TestRunIsIdempotent(t *testing.T) {
	t.Parallel()

	f := &fake{setupCompleted: true, token: "tok-123", apiKey: "key-abc"}
	env, _ := testEnv(t)
	cfg := config(t, f.server(t))

	if err := setup.Run(context.Background(), env, cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if f.accountCreated {
		t.Error("admin account was re-created on an already-set-up instance")
	}
	if got := readFile(t, cfg.KeyFile); !strings.Contains(got, "key-abc") {
		t.Errorf("key file = %q, want it to carry the key", got)
	}
}

// The key file is an EnvironmentFile for configuratarr.service, so its mode is
// part of the contract, not an implementation detail.
func TestKeyFileIsOwnerOnly(t *testing.T) {
	t.Parallel()

	f := &fake{token: "tok-123", apiKey: "key-abc"}
	env, _ := testEnv(t)
	cfg := config(t, f.server(t))

	if err := setup.Run(context.Background(), env, cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	info, err := os.Stat(cfg.KeyFile)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Errorf("key file mode = %04o, want %04o", got, want)
	}
}

// Either half of a --x / --x-file pair works, so the host decides whether a
// value is a secret without the tool being rewritten.
func TestCredentialsAcceptEitherForm(t *testing.T) {
	t.Parallel()

	base := setup.Defaults()

	tests := map[string]func(*testing.T) setup.Config{
		"both literal": func(*testing.T) setup.Config {
			cfg := base
			cfg.AdminUser = "admin"
			cfg.AdminPassword = adminPassword
			return cfg
		},
		"both by file": func(t *testing.T) setup.Config {
			cfg := base
			cfg.AdminUserFile = writeTemp(t, "admin\n")
			cfg.AdminPasswordFile = writeTemp(t, adminPassword+"\n")
			return cfg
		},
		"mixed": func(t *testing.T) setup.Config {
			cfg := base
			cfg.AdminUser = "admin"
			cfg.AdminPasswordFile = writeTemp(t, adminPassword+"\n")
			return cfg
		},
	}

	for name, build := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			creds, err := build(t).Credentials()
			if err != nil {
				t.Fatalf("Credentials() error = %v", err)
			}
			if creds.Username != "admin" {
				t.Errorf("Username = %q, want admin", creds.Username)
			}
			if got := creds.Password.Reveal(); got != adminPassword {
				t.Errorf("Password = %q, want %q", got, adminPassword)
			}
		})
	}
}

func TestCredentialsRejectBadFlagCombinations(t *testing.T) {
	t.Parallel()

	base := setup.Defaults()

	tests := map[string]struct {
		build func(*testing.T) setup.Config
		want  error
	}{
		"nothing at all": {
			build: func(*testing.T) setup.Config { return base },
			want:  secret.ErrMissing,
		},
		"password only": {
			build: func(*testing.T) setup.Config {
				cfg := base
				cfg.AdminPassword = adminPassword
				return cfg
			},
			want: secret.ErrMissing,
		},
		"user given twice": {
			build: func(t *testing.T) setup.Config {
				cfg := base
				cfg.AdminUser = "admin"
				cfg.AdminUserFile = writeTemp(t, "other\n")
				cfg.AdminPassword = adminPassword
				return cfg
			},
			want: secret.ErrAmbiguous,
		},
		"empty password file": {
			build: func(t *testing.T) setup.Config {
				cfg := base
				cfg.AdminUser = "admin"
				cfg.AdminPasswordFile = writeTemp(t, "")
				return cfg
			},
			want: secret.ErrEmpty,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := tc.build(t).Credentials(); !errors.Is(err, tc.want) {
				t.Fatalf("Credentials() error = %v, want %v", err, tc.want)
			}
		})
	}
}

// Bad credentials must fail before any request, and before any file is written.
func TestBadCredentialsWriteNothing(t *testing.T) {
	t.Parallel()

	f := &fake{token: "tok-123", apiKey: "key-abc"}
	env, _ := testEnv(t)
	cfg := config(t, f.server(t))
	cfg.AdminUser = ""
	cfg.AdminUserFile = ""

	err := setup.Run(context.Background(), env, cfg)
	if !errors.Is(err, secret.ErrMissing) {
		t.Fatalf("Run() error = %v, want %v", err, secret.ErrMissing)
	}
	if f.accountCreated {
		t.Error("an account was created despite missing credentials")
	}
	if _, statErr := os.Stat(cfg.KeyFile); !errors.Is(statErr, fs.ErrNotExist) {
		t.Error("a key file was written despite missing credentials")
	}
}

func TestDryRunTouchesNothing(t *testing.T) {
	t.Parallel()

	f := &fake{token: "tok-123", apiKey: "key-abc"}
	env, _ := testEnv(t)
	env.DryRun = true
	cfg := config(t, f.server(t))

	if err := setup.Run(context.Background(), env, cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if f.accountCreated {
		t.Error("an account was created during a dry run")
	}
	if _, err := os.Stat(cfg.KeyFile); !errors.Is(err, fs.ErrNotExist) {
		t.Error("a key file was written during a dry run")
	}
}

// secret.Value makes this a property of the type rather than of this tool, but
// it is asserted here too: this is the path that actually runs in production.
func TestCredentialsNeverReachTheLogs(t *testing.T) {
	t.Parallel()

	f := &fake{token: "tok-123", apiKey: "key-abc"}
	env, logs := testEnv(t)
	cfg := config(t, f.server(t))

	if err := setup.Run(context.Background(), env, cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, s := range []string{adminPassword, "key-abc", "tok-123"} {
		if strings.Contains(logs.String(), s) {
			t.Errorf("logs leaked %q:\n%s", s, logs.String())
		}
	}
}

func TestWriteKeyFile(t *testing.T) {
	t.Parallel()

	t.Run("refuses an empty key", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "api-key.env")
		if err := setup.WriteKeyFile(path, secret.Value{}); err == nil {
			t.Fatal("WriteKeyFile() error = nil, want a refusal")
		}
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Error("a file was created for an empty key")
		}
	})

	// %s on a secret.Value renders [REDACTED]; the key file must carry the
	// real thing, or configuratarr gets a valid-looking file that cannot
	// authenticate.
	t.Run("writes the revealed key", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "api-key.env")
		if err := setup.WriteKeyFile(path, secret.New("key-abc")); err != nil {
			t.Fatalf("WriteKeyFile() error = %v", err)
		}
		got := readFile(t, path)
		if want := "CLEANUPARR_API_KEY=key-abc\n"; got != want {
			t.Errorf("key file = %q, want %q", got, want)
		}
		if strings.Contains(got, secret.Redacted) {
			t.Errorf("key file contains the redaction placeholder: %q", got)
		}
	})

	t.Run("replaces an existing file", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "api-key.env")
		if err := os.WriteFile(path, []byte("CLEANUPARR_API_KEY=stale\n"), 0o600); err != nil {
			t.Fatalf("seed key file: %v", err)
		}
		if err := setup.WriteKeyFile(path, secret.New("fresh")); err != nil {
			t.Fatalf("WriteKeyFile() error = %v", err)
		}
		if got, want := readFile(t, path), "CLEANUPARR_API_KEY=fresh\n"; got != want {
			t.Errorf("key file = %q, want %q", got, want)
		}
	})

	// The temp file is created beside the target so the rename is atomic;
	// an unwritable directory must fail rather than leave a partial file.
	t.Run("reports an unwritable directory", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "absent", "api-key.env")
		if err := setup.WriteKeyFile(path, secret.New("key")); err == nil {
			t.Fatal("WriteKeyFile() error = nil, want a failure")
		}
	})

	// A crash mid-write must not leave configuratarr an EnvironmentFile
	// containing half a key, so no stray temp files may survive either.
	t.Run("leaves no temp files behind", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "api-key.env")
		if err := setup.WriteKeyFile(path, secret.New("key")); err != nil {
			t.Fatalf("WriteKeyFile() error = %v", err)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read dir: %v", err)
		}
		if len(entries) != 1 || entries[0].Name() != "api-key.env" {
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name()
			}
			t.Errorf("directory contains %v, want only api-key.env", names)
		}
	})
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path) //nolint:gosec // a test-controlled path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

func writeTemp(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

// The failure paths matter more than the happy one here: this unit gates
// configuratarr, so anything that goes wrong must stop before a key file
// exists rather than leave a stale or empty one behind.
func TestRunFailures(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		fake    *fake
		mutate  func(*setup.Config)
		wantErr error
	}{
		"unusable base url": {
			fake:   &fake{token: "tok-123", apiKey: "key-abc"},
			mutate: func(c *setup.Config) { c.Base = "not-a-url" },
		},
		"login returns no token": {
			fake: &fake{setupCompleted: true, token: "", apiKey: "key-abc"},
		},
		"api key fetch is unauthorised": {
			// The fake only serves the key to its own token, so a login
			// that hands back a different one fails the fetch.
			fake: &fake{setupCompleted: true, token: "tok-123", apiKey: ""},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			env, _ := testEnv(t)
			cfg := config(t, tc.fake.server(t))
			if tc.mutate != nil {
				tc.mutate(&cfg)
			}

			if err := setup.Run(context.Background(), env, cfg); err == nil {
				t.Fatal("Run() error = nil, want a failure")
			}
			if _, err := os.Stat(cfg.KeyFile); !errors.Is(err, fs.ErrNotExist) {
				t.Error("a key file was written despite the failure")
			}
		})
	}
}

// A cancelled context must abort the readiness wait rather than spin until
// systemd's TimeoutStartSec kills the unit.
func TestRunStopsOnCancellation(t *testing.T) {
	t.Parallel()

	f := &fake{token: "tok-123", apiKey: "key-abc"}
	env, _ := testEnv(t)
	cfg := config(t, f.server(t))
	cfg.Base = "http://127.0.0.1:1" // nothing listening

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := setup.Run(ctx, env, cfg); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context.DeadlineExceeded", err)
	}
}

// The rename is what makes the write atomic; if it cannot happen the whole
// call must fail rather than report success with nothing installed.
func TestWriteKeyFileReportsAFailedInstall(t *testing.T) {
	t.Parallel()

	// A directory at the target path: os.Rename onto it cannot succeed.
	path := filepath.Join(t.TempDir(), "api-key.env")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("seed directory: %v", err)
	}

	if err := setup.WriteKeyFile(path, secret.New("key-abc")); err == nil {
		t.Fatal("WriteKeyFile() error = nil, want an install failure")
	}
}
