// Package inject splices the forgejo API token into code-server's
// settings.json.
//
// The token arrives as a systemd credential, so the plaintext never enters
// the Nix store or the committed settings.json — it is written into the
// container's own appdata copy at runtime.
//
// It goes onto each forgejo.instances[].token, not the top-level
// forgejo.token: forgejo-vscode only consumes the top-level key through its
// legacy single-instance migration, which is skipped whenever
// forgejo.instances is populated, as it is here. The path that actually runs
// reads the per-instance token and stores it in SecretStorage keyed by the
// instance id, so the token has to ride on the instance object.
package inject

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstancesKey is the settings key holding the configured forgejo instances.
const InstancesKey = "forgejo.instances"

// TokenKey is the per-instance field the extension reads.
const TokenKey = "token"

// Run splices the token from tokenPath into settingsPath.
//
// It is fail-open by design and idempotent: a missing settings file, a
// missing or empty credential, or no instances to attach to are all
// no-action rather than errors, so a first boot before the secret exists
// still comes up cleanly. It reports whether it wrote anything.
func Run(settingsPath, tokenPath string) (written bool, err error) {
	if !isFile(settingsPath) || !isFile(tokenPath) {
		return false, nil
	}

	raw, err := os.ReadFile(tokenPath) //nolint:gosec // the path is the unit's own credential
	if err != nil {
		return false, fmt.Errorf("read token %s: %w", tokenPath, err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return false, nil
	}

	rawSettings, err := os.ReadFile(settingsPath) //nolint:gosec // the path is the unit's own settings file
	if err != nil {
		return false, fmt.Errorf("read settings %s: %w", settingsPath, err)
	}
	var settings map[string]any
	if err := json.Unmarshal(rawSettings, &settings); err != nil {
		return false, fmt.Errorf("parse settings %s: %w", settingsPath, err)
	}

	instances, ok := settings[InstancesKey].([]any)
	if !ok || len(instances) == 0 {
		return false, nil // nothing to attach the token to
	}

	changed := false
	for _, entry := range instances {
		instance, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if instance[TokenKey] != token {
			instance[TokenKey] = token
			changed = true
		}
	}
	if !changed {
		return false, nil // already current, nothing to write
	}

	if err := write(settingsPath, settings); err != nil {
		return false, err
	}
	return true, nil
}

// write replaces the settings file atomically, keeping its mode.
func write(path string, settings map[string]any) error {
	// The seeded file is 0644, which is the mode the code-server service
	// user reads it under; a fresh temporary file would be 0600 and would
	// narrow it.
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	encoded = append(encoded, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings.*.json")
	if err != nil {
		return fmt.Errorf("create temporary file beside %s: %w", path, err)
	}
	tmpName := tmp.Name()

	// Anything that fails from here leaves no half-written settings behind:
	// code-server reads this file at start, and truncated JSON would stop it
	// coming up at all.
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(encoded); err != nil {
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err = tmp.Chmod(info.Mode()); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s: %w", tmpName, err)
	}
	return nil
}

// isFile reports whether path names an existing regular file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
