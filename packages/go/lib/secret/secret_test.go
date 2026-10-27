package secret_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"homelab/secret"
)

const plaintext = "hunter2"

// Redaction is the whole point of the type, so every rendering path Go offers
// is asserted rather than assumed. A gap here is a credential in a log file.
func TestValueRedactsEveryRenderingPath(t *testing.T) {
	t.Parallel()

	v := secret.New(plaintext)

	renders := map[string]string{
		"String":     v.String(),
		"%v":         fmt.Sprintf("%v", v), //nolint:gocritic // the point is that fmt routes through String
		"%s":         fmt.Sprintf("%s", v), //nolint:gocritic,staticcheck // the point is that fmt routes through String
		"%q":         fmt.Sprintf("%q", v),
		"%#v":        fmt.Sprintf("%#v", v),
		"%+v":        fmt.Sprintf("%+v", v),
		"Sprint":     fmt.Sprint(v), //nolint:gocritic // the point is that fmt routes through String
		"in a slice": fmt.Sprintf("%v", []secret.Value{v}),
		"in a map":   fmt.Sprintf("%v", map[string]secret.Value{"k": v}),
		"in a struct": fmt.Sprintf("%v", struct {
			Password secret.Value
		}{v}),
		"wrapped in an error": fmt.Errorf("failed with %v", v).Error(),
	}

	for name, got := range renders {
		if strings.Contains(got, plaintext) {
			t.Errorf("%s leaked the plaintext: %s", name, got)
		}
		if !strings.Contains(got, secret.Redacted) {
			t.Errorf("%s = %s, want it to contain %s", name, got, secret.Redacted)
		}
	}
}

func TestValueRedactsInJSON(t *testing.T) {
	t.Parallel()

	payload := struct {
		User     string        `json:"user"`
		Password secret.Value  `json:"password"`
		Token    *secret.Value `json:"token"`
	}{User: "admin", Password: secret.New(plaintext), Token: ptr(secret.New("tok"))}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(encoded), plaintext) {
		t.Errorf("JSON leaked the plaintext: %s", encoded)
	}
	if !strings.Contains(string(encoded), "admin") {
		t.Errorf("JSON = %s, want the non-secret field to survive", encoded)
	}
}

// slog.LogValuer is what makes redaction survive an ordinary log call, with no
// discipline required at the call site.
func TestValueRedactsInSlog(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	v := secret.New(plaintext)
	logger.Info("authenticating", "password", v)
	logger.Info("grouped", slog.Group("creds", "password", v))
	logger.With("password", v).Info("with-attrs")

	if strings.Contains(buf.String(), plaintext) {
		t.Errorf("slog leaked the plaintext:\n%s", buf.String())
	}
	if strings.Count(buf.String(), secret.Redacted) != 3 {
		t.Errorf("want three redactions, got:\n%s", buf.String())
	}
}

func TestRevealIsTheOnlyWayOut(t *testing.T) {
	t.Parallel()

	if got := secret.New(plaintext).Reveal(); got != plaintext {
		t.Errorf("Reveal() = %q, want %q", got, plaintext)
	}
}

func TestIsZero(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		value secret.Value
		want  bool
	}{
		"zero value": {value: secret.Value{}, want: true},
		"empty":      {value: secret.New(""), want: true},
		"set":        {value: secret.New(plaintext), want: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := tc.value.IsZero(); got != tc.want {
				t.Errorf("IsZero() = %v, want %v", got, tc.want)
			}
		})
	}
}

// A trailing newline is what every editor and `echo` adds, and is never part
// of the secret. Everything else is left alone, because it might be.
func TestFromFileTrimsTrailingNewlineOnly(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		contents string
		want     string
	}{
		"trailing lf":     {contents: "value\n", want: "value"},
		"trailing crlf":   {contents: "value\r\n", want: "value"},
		"several":         {contents: "value\n\n", want: "value"},
		"none":            {contents: "value", want: "value"},
		"interior spaces": {contents: "two words\n", want: "two words"},
		"leading space":   {contents: " value\n", want: " value"},
		"trailing space":  {contents: "value \n", want: "value "},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			v, err := secret.FromFile(writeTemp(t, tc.contents))
			if err != nil {
				t.Fatalf("FromFile() error = %v", err)
			}
			if got := v.Reveal(); got != tc.want {
				t.Errorf("Reveal() = %q, want %q", got, tc.want)
			}
		})
	}
}

// An empty secret is always a deployment fault; failing beats authenticating
// with the empty string.
func TestFromFileRejectsEmpty(t *testing.T) {
	t.Parallel()

	for name, contents := range map[string]string{
		"nothing":       "",
		"just newlines": "\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := secret.FromFile(writeTemp(t, contents))
			if !errors.Is(err, secret.ErrEmpty) {
				t.Fatalf("FromFile() error = %v, want %v", err, secret.ErrEmpty)
			}
		})
	}
}

func TestFromFileReportsAMissingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "absent")
	_, err := secret.FromFile(path)
	if err == nil {
		t.Fatal("FromFile() error = nil, want a read failure")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the path", err)
	}
}

// Not parallel: t.Setenv forbids t.Parallel on the test or any parent.
func TestFromEnv(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("TEST_SECRET", plaintext)

		v, err := secret.FromEnv("TEST_SECRET")
		if err != nil {
			t.Fatalf("FromEnv() error = %v", err)
		}
		if got := v.Reveal(); got != plaintext {
			t.Errorf("Reveal() = %q, want %q", got, plaintext)
		}
	})

	t.Run("unset", func(t *testing.T) {
		t.Setenv("TEST_SECRET", "")

		if _, err := secret.FromEnv("TEST_SECRET"); !errors.Is(err, secret.ErrMissing) {
			t.Fatalf("FromEnv() error = %v, want %v", err, secret.ErrMissing)
		}
	})
}

// The --x / --x-file pair is what lets the host decide whether a value is a
// secret: a username that is public in this repo can be a literal, one that
// lives in sops is a path, and the tool is written once either way.
func TestFlag(t *testing.T) {
	t.Parallel()

	t.Run("literal", func(t *testing.T) {
		t.Parallel()

		v, err := secret.Flag("admin-user", "admin", "")
		if err != nil {
			t.Fatalf("Flag() error = %v", err)
		}
		if got := v.Reveal(); got != "admin" {
			t.Errorf("Reveal() = %q, want admin", got)
		}
	})

	t.Run("file", func(t *testing.T) {
		t.Parallel()

		v, err := secret.Flag("admin-user", "", writeTemp(t, "from-file\n"))
		if err != nil {
			t.Fatalf("Flag() error = %v", err)
		}
		if got := v.Reveal(); got != "from-file" {
			t.Errorf("Reveal() = %q, want from-file", got)
		}
	})

	// Silently preferring one would make a misconfigured unit authenticate
	// with a value its author did not intend.
	t.Run("both is ambiguous", func(t *testing.T) {
		t.Parallel()

		_, err := secret.Flag("admin-user", "admin", writeTemp(t, "other"))
		if !errors.Is(err, secret.ErrAmbiguous) {
			t.Fatalf("Flag() error = %v, want %v", err, secret.ErrAmbiguous)
		}
		if !strings.Contains(err.Error(), "--admin-user-file") {
			t.Errorf("error %q does not name both flags", err)
		}
	})

	t.Run("neither is missing", func(t *testing.T) {
		t.Parallel()

		_, err := secret.Flag("admin-user", "", "")
		if !errors.Is(err, secret.ErrMissing) {
			t.Fatalf("Flag() error = %v, want %v", err, secret.ErrMissing)
		}
		if !strings.Contains(err.Error(), "--admin-user") {
			t.Errorf("error %q does not name the flag", err)
		}
	})

	t.Run("propagates a file failure", func(t *testing.T) {
		t.Parallel()

		_, err := secret.Flag("admin-user", "", writeTemp(t, ""))
		if !errors.Is(err, secret.ErrEmpty) {
			t.Fatalf("Flag() error = %v, want %v", err, secret.ErrEmpty)
		}
	})
}

// A failure must never carry the secret it failed on.
func TestErrorsNeverCarryPlaintext(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, plaintext)

	_, err := secret.Flag("admin-password", plaintext, path)
	if err == nil {
		t.Fatal("Flag() error = nil")
	}
	if strings.Contains(err.Error(), plaintext) {
		t.Errorf("error leaked a secret: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

func writeTemp(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
