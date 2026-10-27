package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"

	"homelab/app"
)

type toolConfig struct {
	BaseURL string `json:"base_url"`
	Retries int    `json:"retries"`
}

// harness builds an App wired to buffers, so a test observes exactly what a
// terminal would: product on stdout, logs on stderr.
type harness struct {
	stdout, stderr bytes.Buffer
	env            *app.Env
	ran            bool
}

func (h *harness) app(a app.App) app.App {
	a.Stdout = &h.stdout
	a.Stderr = &h.stderr
	if a.Args == nil {
		a.Args = []string{}
	}
	if a.Name == "" {
		a.Name = "testtool"
	}
	if a.Run == nil {
		a.Run = func(_ context.Context, env *app.Env) error {
			h.ran = true
			h.env = env
			return nil
		}
	} else {
		inner := a.Run
		a.Run = func(ctx context.Context, env *app.Env) error {
			h.ran = true
			h.env = env
			return inner(ctx, env)
		}
	}
	return a
}

func TestValidation(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		app  app.App
		want error
	}{
		"no name": {
			app:  app.App{Run: func(context.Context, *app.Env) error { return nil }},
			want: app.ErrNoName,
		},
		"no run": {
			app:  app.App{Name: "testtool"},
			want: app.ErrNoRun,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := tc.app.Execute(context.Background()); !errors.Is(err, tc.want) {
				t.Fatalf("Execute() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRunReceivesAPopulatedEnv(t *testing.T) {
	t.Parallel()

	var h harness
	if err := h.app(app.App{}).Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !h.ran {
		t.Fatal("Run was never called")
	}
	if h.env.Logger == nil {
		t.Error("Env.Logger = nil")
	}
	if h.env.Metrics == nil {
		t.Error("Env.Metrics = nil")
	}
	if h.env.Style == nil {
		t.Error("Env.Style = nil")
	}
	if h.env.Stdout == nil {
		t.Error("Env.Stdout = nil")
	}
}

// The product/diagnostics split is the whole reason Env carries a Stdout: a
// tool whose output is piped must not have log lines mixed into it.
func TestProductAndLogsAreSeparate(t *testing.T) {
	t.Parallel()

	var h harness
	a := h.app(app.App{
		Run: func(_ context.Context, env *app.Env) error {
			env.Logger.Info("working")
			if _, err := env.Stdout.Write([]byte("# the product\n")); err != nil {
				return err
			}
			return nil
		},
	})
	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got := h.stdout.String(); got != "# the product\n" {
		t.Errorf("stdout = %q, want only the product", got)
	}
	if got := h.stderr.String(); !strings.Contains(got, "working") {
		t.Errorf("stderr = %q, want the log record", got)
	}
}

// Logs are JSON off a terminal, and carry the tool's identity, because that is
// what Vector ships to Loki.
func TestLogsAreJSONWithIdentity(t *testing.T) {
	t.Parallel()

	var h harness
	a := h.app(app.App{
		Name: "komga-setup",
		Run: func(_ context.Context, env *app.Env) error {
			env.Logger.Info("hello")
			return nil
		},
	})
	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var record map[string]any
	line := strings.SplitN(strings.TrimSpace(h.stderr.String()), "\n", 2)[0]
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatalf("log line is not JSON: %v (%q)", err, line)
	}
	if record["service"] != "komga-setup" {
		t.Errorf("service = %v, want komga-setup", record["service"])
	}
	if record["version"] == nil {
		t.Error("version attribute missing")
	}
}

func TestToolConfigFromFlags(t *testing.T) {
	t.Parallel()

	cfg := toolConfig{Retries: 3}
	var h harness
	a := h.app(app.App{
		Config: &cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("base-url", "", "service root")
		},
		Args: []string{"--base-url", "http://from-flag"},
	})
	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if cfg.BaseURL != "http://from-flag" {
		t.Errorf("BaseURL = %q, want http://from-flag", cfg.BaseURL)
	}
	if cfg.Retries != 3 {
		t.Errorf("Retries = %d, want the default 3 to survive", cfg.Retries)
	}
}

// Not parallel: t.Setenv forbids t.Parallel on the test or any parent.
func TestToolConfigFromEnv(t *testing.T) {
	t.Setenv("HOMELAB_TESTTOOL_BASE_URL", "http://from-env")

	var cfg toolConfig
	var h harness
	if err := h.app(app.App{Config: &cfg}).Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if cfg.BaseURL != "http://from-env" {
		t.Errorf("BaseURL = %q, want http://from-env", cfg.BaseURL)
	}
}

// The shared prefix is what makes one drop-in able to turn up logging across
// every tool at once.
//
// Not parallel: t.Setenv forbids t.Parallel on the test or any parent.
func TestSharedSettingsFromEnv(t *testing.T) {
	t.Setenv("HOMELAB_LOG__LEVEL", "debug")

	var h harness
	a := h.app(app.App{
		Run: func(_ context.Context, env *app.Env) error {
			env.Logger.Debug("verbose")
			return nil
		},
	})
	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(h.stderr.String(), "verbose") {
		t.Errorf("stderr = %q, want the debug record", h.stderr.String())
	}
}

func TestSharedSettingsFromFlags(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args      []string
		wantDebug bool
	}{
		"default hides debug": {args: []string{}, wantDebug: false},
		"flag shows debug":    {args: []string{"--log.level", "debug"}, wantDebug: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var h harness
			a := h.app(app.App{
				Args: tc.args,
				Run: func(_ context.Context, env *app.Env) error {
					env.Logger.Debug("verbose")
					return nil
				},
			})
			if err := a.Execute(context.Background()); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got := strings.Contains(h.stderr.String(), "verbose"); got != tc.wantDebug {
				t.Errorf("debug emitted = %v, want %v", got, tc.wantDebug)
			}
		})
	}
}

func TestDryRunReachesEnv(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args []string
		want bool
	}{
		"absent":  {args: []string{}, want: false},
		"present": {args: []string{"--dry-run"}, want: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var h harness
			if err := h.app(app.App{Args: tc.args}).Execute(context.Background()); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if h.env.DryRun != tc.want {
				t.Errorf("Env.DryRun = %v, want %v", h.env.DryRun, tc.want)
			}
		})
	}
}

func TestRunErrorIsReturnedAndLogged(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("the tool failed")
	var h harness
	a := h.app(app.App{
		Run: func(context.Context, *app.Env) error { return sentinel },
	})

	err := a.Execute(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("Execute() error = %v, want %v", err, sentinel)
	}
	if !strings.Contains(h.stderr.String(), "the tool failed") {
		t.Errorf("stderr = %q, want the failure logged", h.stderr.String())
	}
}

// A daemon stopped by SIGTERM has succeeded, not failed — otherwise every
// clean restart would look like a crash in the journal.
func TestCancellationIsNotAFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var h harness
	a := h.app(app.App{
		Run: func(ctx context.Context, _ *app.Env) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	if err := a.Execute(ctx); err != nil {
		t.Fatalf("Execute() error = %v, want nil after cancellation", err)
	}
}

func TestInvalidStandardSettingIsRejected(t *testing.T) {
	t.Parallel()

	var h harness
	a := h.app(app.App{Args: []string{"--log.level", "trace"}})

	if err := a.Execute(context.Background()); err == nil {
		t.Fatal("Execute() error = nil, want a rejected log level")
	}
	if h.ran {
		t.Error("Run was called despite an invalid configuration")
	}
}

// A mistyped flag must say so. Both of cobra's silencing flags are on so that a
// run error is reported once through the logger, which previously left usage
// errors exiting silently.
func TestUsageErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args []string
		want string
	}{
		"unknown flag":    {args: []string{"--nonesuch"}, want: "nonesuch"},
		"positional args": {args: []string{"surprise"}, want: "surprise"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var h harness
			a := h.app(app.App{Args: tc.args})

			err := a.Execute(context.Background())
			if err == nil {
				t.Fatal("Execute() error = nil, want a usage error")
			}
			if !app.IsUsageError(err) {
				t.Errorf("IsUsageError() = false for %v", err)
			}
			if h.ran {
				t.Error("Run was called despite a usage error")
			}
			if got := h.stderr.String(); !strings.Contains(got, tc.want) {
				t.Errorf("stderr = %q, want it to mention %q", got, tc.want)
			}
			if got := h.stderr.String(); !strings.Contains(got, "--help") {
				t.Errorf("stderr = %q, want it to point at --help", got)
			}
		})
	}
}

// A failure in the work is not a usage error, so a wrapper can tell "called it
// wrong" (exit 2) from "ran and failed" (exit 1).
func TestRunFailureIsNotAUsageError(t *testing.T) {
	t.Parallel()

	var h harness
	a := h.app(app.App{
		Run: func(context.Context, *app.Env) error { return errors.New("boom") },
	})

	err := a.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute() error = nil, want a failure")
	}
	if app.IsUsageError(err) {
		t.Errorf("IsUsageError() = true for a run failure: %v", err)
	}
}

func TestVersionFlagSkipsRun(t *testing.T) {
	t.Parallel()

	var h harness
	a := h.app(app.App{Args: []string{"--version"}})
	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ran {
		t.Error("Run was called for --version")
	}
	if !strings.Contains(h.stdout.String(), "testtool") {
		t.Errorf("stdout = %q, want the version line", h.stdout.String())
	}
}
