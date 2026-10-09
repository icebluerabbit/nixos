// Package seed merges declared rows into Home Assistant's .storage files.
//
// HA has no declarative configuration for areas, config entries or persons,
// so these are merged into its own state before home-assistant.service
// starts. Each registry has its own rules and they are not interchangeable:
// areas are append-only, persons are never created here because SSO owns
// them, and config entries sync identity fields and replace subentries
// wholesale.
//
// Every seeder is non-destructive by construction. HA rewrites these files
// itself at runtime, so anything already there belongs to HA or to a person
// who changed something in the UI.
package seed

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// filePerm is what a written storage file gets. HA runs as its own user and
// these hold credentials.
const filePerm = 0o600

// dirPerm is what a created .storage directory gets.
const dirPerm = 0o700

// indent is what HA itself writes these files with.
const indent = "  "

// Storage is one HA .storage document, kept as decoded JSON rather than a
// struct.
//
// A struct would be wrong in both directions: HA owns this schema, so a
// field this does not know about has to survive the rewrite, and a field
// the document does not have must not be invented. Typing the header once
// added "minor_version": 0 to a registry that had never carried one, which
// is exactly the kind of thing HA's schema migration reads.
type Storage map[string]any

// Data returns the document's data object, creating it when absent.
func (s Storage) Data() map[string]any {
	data, ok := s["data"].(map[string]any)
	if !ok {
		data = map[string]any{}
		s["data"] = data
	}
	return data
}

// ReadStorage loads a storage file. missing reports whether it was absent,
// which each registry answers differently.
func ReadStorage(path string) (storage Storage, missing bool, err error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is HA's own storage file
	if os.IsNotExist(err) {
		return Storage{}, true, nil
	}
	if err != nil {
		return Storage{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &storage); err != nil {
		return Storage{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return storage, false, nil
}

// WriteStorage replaces a storage file, creating its directory if needed.
func WriteStorage(path string, storage Storage) error {
	encoded, err := json.MarshalIndent(storage, "", indent)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, encoded, filePerm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// ReadSeed decodes the declaration from stdin into v.
func ReadSeed(r io.Reader, v any) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("parse seed: %w", err)
	}
	return nil
}

// ReadSeedExpanded is ReadSeed with environment substitution first, which is
// how a config entry carries a secret: the declaration names $VAR and the
// unit supplies it through EnvironmentFile.
func ReadSeedExpanded(r io.Reader, lookup func(string) (string, bool), v any) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	expanded := ExpandVars(string(raw), lookup)
	if strings.TrimSpace(expanded) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(expanded), v); err != nil {
		return fmt.Errorf("parse seed: %w", err)
	}
	return nil
}

// ExpandVars substitutes $VAR and ${VAR} from lookup.
//
// An unset variable is left exactly as written, which is what Python's
// os.path.expandvars does and is load-bearing here: blanking it instead —
// what os.ExpandEnv would do — would silently write an empty password into
// HA's config entry and the integration would fail to authenticate with no
// hint why.
func ExpandVars(s string, lookup func(string) (string, bool)) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '$' || i+1 >= len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}

		name, raw, width := varAt(s[i:])
		if name == "" {
			b.WriteString(raw)
			i += width
			continue
		}
		if value, ok := lookup(name); ok {
			b.WriteString(value)
		} else {
			b.WriteString(raw)
		}
		i += width
	}
	return b.String()
}

// varAt reads one $VAR or ${VAR} reference from the front of s, returning
// the name, the exact text it was written as, and how much was consumed. A
// name of "" means this was not a reference.
func varAt(s string) (name, raw string, width int) {
	if strings.HasPrefix(s, "${") {
		end := strings.IndexByte(s, '}')
		if end < 0 {
			return "", s, len(s)
		}
		return s[2:end], s[:end+1], end + 1
	}

	i := 1
	for i < len(s) && (isAlphaNum(s[i]) || s[i] == '_') {
		i++
	}
	if i == 1 {
		return "", s[:1], 1 // a bare $, or $$
	}
	return s[1:i], s[:i], i
}

func isAlphaNum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// rows reads one list out of a storage document's data.
func rows(storage Storage, key string) []any {
	list, _ := storage.Data()[key].([]any)
	return list
}

// Timestamp formats a time the way Home Assistant's own Python writes one:
// microsecond precision and a numeric UTC offset, not a Z. HA parses both,
// but a file that suddenly changes shape is a file somebody has to explain.
func Timestamp(t time.Time) string {
	utc := t.UTC()
	if utc.Nanosecond() == 0 {
		return utc.Format("2006-01-02T15:04:05-07:00")
	}
	return utc.Format("2006-01-02T15:04:05.000000-07:00")
}

// asObject reads one row as an object.
func asObject(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

// str reads a string field from a row.
func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}
