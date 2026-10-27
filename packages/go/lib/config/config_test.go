package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/pflag"

	"homelab/config"
)

type logSettings struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

type settings struct {
	BaseURL string      `json:"base_url"`
	Retries int         `json:"retries"`
	DryRun  bool        `json:"dry_run"`
	Log     logSettings `json:"log"`
}

func TestLoadRejectsNilDestination(t *testing.T) {
	t.Parallel()

	if err := config.Load(nil, config.Options{}); !errors.Is(err, config.ErrNoDestination) {
		t.Fatalf("Load(nil) error = %v, want %v", err, config.ErrNoDestination)
	}
}

// Defaults are expressed by pre-populating the destination; no layer may zero
// a field it does not mention.
// Not parallel: t.Setenv forbids t.Parallel on the test or any parent.
func TestDefaultsSurviveEveryLayer(t *testing.T) {
	path := writeConfig(t, `{"retries": 9}`)
	t.Setenv("HL_DRY_RUN", "true")

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("log.format", "json", "")
	mustParse(t, flags, []string{"--log.format", "text"})

	got := settings{BaseURL: "http://default", Log: logSettings{Level: "warn"}}
	err := config.Load(&got, config.Options{
		EnvPrefix: "HL_",
		FilePath:  path,
		Flags:     flags,
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := settings{
		BaseURL: "http://default",
		Retries: 9,
		DryRun:  true,
		Log:     logSettings{Level: "warn", Format: "text"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("settings mismatch (-want +got):\n%s", diff)
	}
}

// Not parallel: t.Setenv forbids t.Parallel on the test or any parent.
func TestPrecedence(t *testing.T) {
	t.Run("env beats file", func(t *testing.T) {
		path := writeConfig(t, `{"base_url": "http://from-file"}`)
		t.Setenv("HL_BASE_URL", "http://from-env")

		var got settings
		err := config.Load(&got, config.Options{EnvPrefix: "HL_", FilePath: path})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got.BaseURL != "http://from-env" {
			t.Errorf("BaseURL = %q, want http://from-env", got.BaseURL)
		}
	})

	t.Run("flag beats env", func(t *testing.T) {
		t.Setenv("HL_BASE_URL", "http://from-env")

		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.String("base-url", "", "")
		mustParse(t, flags, []string{"--base-url", "http://from-flag"})

		var got settings
		err := config.Load(&got, config.Options{EnvPrefix: "HL_", Flags: flags})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got.BaseURL != "http://from-flag" {
			t.Errorf("BaseURL = %q, want http://from-flag", got.BaseURL)
		}
	})

	// The critical case: an unset flag still carries its default value, and
	// that default must not clobber deployed configuration.
	t.Run("unset flag does not clobber env", func(t *testing.T) {
		t.Setenv("HL_BASE_URL", "http://from-env")

		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.String("base-url", "http://flag-default", "")
		mustParse(t, flags, nil)

		var got settings
		err := config.Load(&got, config.Options{EnvPrefix: "HL_", Flags: flags})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got.BaseURL != "http://from-env" {
			t.Errorf("BaseURL = %q, want http://from-env", got.BaseURL)
		}
	})
}

// Not parallel: t.Setenv forbids t.Parallel on the test or any parent.
func TestNestedKeysFromEnvAndFlags(t *testing.T) {
	t.Run("env double underscore nests", func(t *testing.T) {
		t.Setenv("HL_LOG__LEVEL", "debug")

		var got settings
		if err := config.Load(&got, config.Options{EnvPrefix: "HL_"}); err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got.Log.Level != "debug" {
			t.Errorf("Log.Level = %q, want debug", got.Log.Level)
		}
	})

	t.Run("flag dot nests", func(t *testing.T) {
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.String("log.level", "", "")
		mustParse(t, flags, []string{"--log.level", "error"})

		var got settings
		if err := config.Load(&got, config.Options{Flags: flags}); err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got.Log.Level != "error" {
			t.Errorf("Log.Level = %q, want error", got.Log.Level)
		}
	})
}

func TestFileLayer(t *testing.T) {
	t.Parallel()

	t.Run("missing file is an error by default", func(t *testing.T) {
		t.Parallel()

		var got settings
		err := config.Load(&got, config.Options{
			FilePath: filepath.Join(t.TempDir(), "absent.json"),
		})
		if err == nil {
			t.Fatal("Load() error = nil, want a missing-file failure")
		}
	})

	t.Run("missing file tolerated when optional", func(t *testing.T) {
		t.Parallel()

		got := settings{BaseURL: "http://default"}
		err := config.Load(&got, config.Options{
			FilePath:     filepath.Join(t.TempDir(), "absent.json"),
			FileOptional: true,
		})
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got.BaseURL != "http://default" {
			t.Errorf("BaseURL = %q, want the default to survive", got.BaseURL)
		}
	})

	t.Run("malformed file is an error", func(t *testing.T) {
		t.Parallel()

		var got settings
		err := config.Load(&got, config.Options{FilePath: writeConfig(t, "{not json")})
		if err == nil {
			t.Fatal("Load() error = nil, want a parse failure")
		}
	})
}

// A tool that passes no layers at all is a valid, if unusual, caller.
func TestLoadWithNoLayersIsANoop(t *testing.T) {
	t.Parallel()

	got := settings{BaseURL: "http://default"}
	if err := config.Load(&got, config.Options{}); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if diff := cmp.Diff(settings{BaseURL: "http://default"}, got); diff != "" {
		t.Errorf("settings mismatch (-want +got):\n%s", diff)
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func mustParse(t *testing.T, flags *pflag.FlagSet, args []string) {
	t.Helper()

	if err := flags.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
}
