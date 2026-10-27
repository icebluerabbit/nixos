// Package secret carries credentials without letting them escape by accident.
//
// A Value redacts itself in every rendering path Go has — fmt, JSON and slog —
// so leaking one requires explicitly calling Reveal. That turns "don't log the
// password" from a rule each tool has to remember into a property of the type.
//
// Secrets arrive by path. A sops-nix secret, a systemd LoadCredential file and
// a plain file are all the same thing to a tool: somewhere to read. The host
// knows which one it chose and says so on argv, matching this repo's rule that
// packages are fixed and hosts vary the flags — so nothing here probes the
// environment guessing at delivery.
//
// Usernames count. Fourteen of the secrets in this homelab are account
// identifiers rather than passwords, so every credential field gets a
// --x / --x-file pair and the host decides which half of it to use.
//
// What this package deliberately does not do: zero memory, mlock, cache, watch
// for rotation, or re-read on change. Go's garbage collector copies strings
// freely, so a type promising the plaintext had been scrubbed from RAM would be
// lying — and a lie in a security type is worse than a plain string.
package secret

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Redacted is what a Value renders as everywhere except Reveal.
const Redacted = "[REDACTED]"

// Value is a credential.
//
// The zero Value is empty and reports IsZero. It is comparable, so it can be
// used as a map key or compared for equality without revealing it.
type Value struct{ v string }

// New wraps a string that is already in hand. Prefer FromFile: a secret that
// arrives as a plain string has usually already been somewhere it should not
// have been.
func New(s string) Value { return Value{v: s} }

// Reveal returns the plaintext. Every call is a deliberate decision to expose
// it, which is what makes them worth grepping for.
func (v Value) Reveal() string { return v.v }

// IsZero reports whether the value is empty.
func (v Value) IsZero() bool { return v.v == "" }

// String implements fmt.Stringer, covering %v, %s and Print.
func (v Value) String() string { return Redacted }

// GoString implements fmt.GoStringer, covering %#v.
func (v Value) GoString() string { return Redacted }

// MarshalJSON keeps a Value out of any struct that gets serialised.
func (v Value) MarshalJSON() ([]byte, error) {
	return []byte(`"` + Redacted + `"`), nil
}

// MarshalText covers encoders that prefer TextMarshaler.
func (v Value) MarshalText() ([]byte, error) {
	return []byte(Redacted), nil
}

// LogValue implements slog.LogValuer.
//
// This is the one that matters in practice: passing a Value as a log attribute
// records [REDACTED] with no discipline required at the call site.
func (v Value) LogValue() slog.Value { return slog.StringValue(Redacted) }

// ErrMissing reports a credential the host supplied by neither route.
var ErrMissing = errors.New("secret: not supplied")

// ErrAmbiguous reports a credential supplied by both routes at once.
var ErrAmbiguous = errors.New("secret: supplied twice")

// ErrEmpty reports a file that exists but holds nothing.
var ErrEmpty = errors.New("secret: file is empty")

// FromFile reads a secret from path.
//
// The trailing newline every editor and `echo` adds is trimmed. Interior and
// leading whitespace are left alone — they may be part of the secret.
//
// An empty file is an error rather than an empty Value: it is always a
// deployment fault, and failing here beats authenticating with "".
func FromFile(path string) (Value, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is the host's choice
	if err != nil {
		return Value{}, fmt.Errorf("secret: read %s: %w", path, err)
	}

	v := New(strings.TrimRight(string(raw), "\r\n"))
	if v.IsZero() {
		return Value{}, fmt.Errorf("%w: %s", ErrEmpty, path)
	}
	return v, nil
}

// FromEnv reads a secret from an environment variable.
//
// Secondary to FromFile, for values that only ever arrive in a composed
// sops.templates env file and are not worth splitting into their own secret.
func FromEnv(name string) (Value, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return Value{}, fmt.Errorf("%w: $%s", ErrMissing, name)
	}
	return New(raw), nil
}

// Flag resolves a --<name> / --<name>-file pair, of which exactly one must be
// set. name is used only to build the error messages.
//
// The pair exists so the host decides whether a value is a secret. A username
// that is public in this repo can be a literal; one that lives in sops is a
// path. The tool is written once either way.
func Flag(name, literal, path string) (Value, error) {
	switch {
	case literal != "" && path != "":
		return Value{}, fmt.Errorf("%w: pass --%s or --%s-file, not both",
			ErrAmbiguous, name, name)
	case path != "":
		return FromFile(path)
	case literal != "":
		return New(literal), nil
	default:
		return Value{}, fmt.Errorf("%w: pass --%s or --%s-file",
			ErrMissing, name, name)
	}
}
