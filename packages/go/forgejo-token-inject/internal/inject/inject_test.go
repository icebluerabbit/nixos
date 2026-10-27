package inject

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// settingsWith writes a settings.json holding the given instances, at the
// 0644 the container seeds it with.
func settingsWith(t *testing.T, dir string, instances []any, extra map[string]any) string {
	t.Helper()
	settings := map[string]any{
		"editor.fontSize": 14,
		InstancesKey:      instances,
	}
	for k, v := range extra {
		settings[k] = v
	}
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil { //nolint:gosec // the mode under test
		t.Fatalf("write settings: %v", err)
	}
	return path
}

// tokenFile writes a credential the way systemd presents one.
func tokenFile(t *testing.T, dir, token string) string {
	t.Helper()
	path := filepath.Join(dir, "forgejo-token")
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	return path
}

// readSettings reads back what the tool wrote.
func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	return settings
}

// tokensOf pulls the per-instance tokens out of a settings document.
func tokensOf(t *testing.T, settings map[string]any) []any {
	t.Helper()
	instances, ok := settings[InstancesKey].([]any)
	if !ok {
		t.Fatalf("%s is %T, want an array", InstancesKey, settings[InstancesKey])
	}
	tokens := make([]any, 0, len(instances))
	for _, entry := range instances {
		instance, ok := entry.(map[string]any)
		if !ok {
			tokens = append(tokens, nil)
			continue
		}
		tokens = append(tokens, instance[TokenKey])
	}
	return tokens
}

// The token goes onto every instance, not the top-level key: the extension's
// migration only reads the per-instance field once instances are populated.
func TestRunWritesTokenOntoEveryInstance(t *testing.T) {
	dir := t.TempDir()
	settings := settingsWith(t, dir, []any{
		map[string]any{"id": "home", "url": "https://git.example"},
		map[string]any{"id": "work", "url": "https://git.work"},
	}, nil)

	written, err := Run(settings, tokenFile(t, dir, "secret-token"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !written {
		t.Fatal("Run reported no write")
	}

	got := readSettings(t, settings)
	for i, token := range tokensOf(t, got) {
		if token != "secret-token" {
			t.Errorf("instance %d token = %v, want secret-token", i, token)
		}
	}
	if _, ok := got["forgejo.token"]; ok {
		t.Error("a top-level forgejo.token was written; the extension ignores it")
	}
}

// Everything else in the file has to survive: it is the user's own editor
// configuration, not a generated file.
func TestRunPreservesOtherSettings(t *testing.T) {
	dir := t.TempDir()
	settings := settingsWith(t, dir,
		[]any{map[string]any{"id": "home"}},
		map[string]any{"workbench.colorTheme": "Solarized", "files.autoSave": "off"},
	)

	if _, err := Run(settings, tokenFile(t, dir, "tok")); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := readSettings(t, settings)
	if got["workbench.colorTheme"] != "Solarized" || got["files.autoSave"] != "off" {
		t.Errorf("other settings were lost: %v", got)
	}
	if got["editor.fontSize"] != float64(14) {
		t.Errorf("editor.fontSize = %v, want 14", got["editor.fontSize"])
	}
}

// Fields already on an instance are left alone; only the token is set.
func TestRunKeepsInstanceFields(t *testing.T) {
	dir := t.TempDir()
	settings := settingsWith(t, dir, []any{
		map[string]any{"id": "home", "url": "https://git.example", "name": "Home"},
	}, nil)

	if _, err := Run(settings, tokenFile(t, dir, "tok")); err != nil {
		t.Fatalf("Run: %v", err)
	}

	instances, _ := readSettings(t, settings)[InstancesKey].([]any)
	instance, _ := instances[0].(map[string]any)
	if instance["url"] != "https://git.example" || instance["name"] != "Home" {
		t.Errorf("instance fields were lost: %v", instance)
	}
}

// Running on every start must not rewrite a file that is already correct:
// an unnecessary write races code-server reading it.
func TestRunIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	settings := settingsWith(t, dir, []any{map[string]any{"id": "home"}}, nil)
	token := tokenFile(t, dir, "tok")

	written, err := Run(settings, token)
	if err != nil || !written {
		t.Fatalf("first Run: written=%v err=%v", written, err)
	}
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}

	written, err = Run(settings, token)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if written {
		t.Error("second Run rewrote a file that was already current")
	}

	after, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("second run changed the file:\n%s\n---\n%s", before, after)
	}
}

// The seeded mode is what the code-server user reads the file under, so the
// rewrite must not narrow it to the temporary file's 0600.
func TestRunPreservesFileMode(t *testing.T) {
	dir := t.TempDir()
	settings := settingsWith(t, dir, []any{map[string]any{"id": "home"}}, nil)

	if _, err := Run(settings, tokenFile(t, dir, "tok")); err != nil {
		t.Fatalf("Run: %v", err)
	}

	info, err := os.Stat(settings)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("mode = %o, want 644", got)
	}
}

// The write goes through a temporary file that must not survive it.
func TestRunLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	settings := settingsWith(t, dir, []any{map[string]any{"id": "home"}}, nil)

	if _, err := Run(settings, tokenFile(t, dir, "tok")); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".settings.") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

// Fail-open: a first boot before the credential exists has to come up
// cleanly rather than block the container on a missing secret.
func TestRunNoOpsWhenInputsAreMissing(t *testing.T) {
	dir := t.TempDir()
	settings := settingsWith(t, dir, []any{map[string]any{"id": "home"}}, nil)
	token := tokenFile(t, dir, "tok")

	cases := map[string]struct{ settings, token string }{
		"no settings file": {filepath.Join(dir, "absent.json"), token},
		"no token file":    {settings, filepath.Join(dir, "absent")},
		"both absent":      {filepath.Join(dir, "a.json"), filepath.Join(dir, "b")},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			written, err := Run(tc.settings, tc.token)
			if err != nil {
				t.Errorf("Run: %v", err)
			}
			if written {
				t.Error("Run wrote something with an input missing")
			}
		})
	}
}

// A credential that exists but is empty is the same situation: the secret is
// not provisioned yet.
func TestRunNoOpsOnEmptyToken(t *testing.T) {
	dir := t.TempDir()
	settings := settingsWith(t, dir, []any{map[string]any{"id": "home"}}, nil)

	written, err := Run(settings, tokenFile(t, dir, "   "))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if written {
		t.Error("Run wrote an empty token")
	}
}

// With no instances there is nothing to attach a token to, and inventing one
// would be a configuration decision this tool does not get to make.
func TestRunNoOpsWithoutInstances(t *testing.T) {
	dir := t.TempDir()
	token := tokenFile(t, dir, "tok")

	cases := map[string]any{
		"empty array":  []any{},
		"wrong type":   "not-an-array",
		"absent":       nil,
		"object value": map[string]any{"id": "home"},
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			settings := map[string]any{"editor.fontSize": 14}
			if value != nil {
				settings[InstancesKey] = value
			}
			raw, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, raw, 0o644); err != nil { //nolint:gosec // matches the seeded mode
				t.Fatal(err)
			}

			written, err := Run(path, token)
			if err != nil {
				t.Errorf("Run: %v", err)
			}
			if written {
				t.Error("Run wrote with no instances to attach to")
			}
		})
	}
}

// An entry that is not an object is skipped rather than crashing the start.
func TestRunSkipsNonObjectInstances(t *testing.T) {
	dir := t.TempDir()
	settings := settingsWith(t, dir, []any{
		"a string",
		map[string]any{"id": "home"},
	}, nil)

	written, err := Run(settings, tokenFile(t, dir, "tok"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !written {
		t.Fatal("the object instance should still have been written")
	}

	tokens := tokensOf(t, readSettings(t, settings))
	if tokens[0] != nil {
		t.Errorf("a string entry was given a token: %v", tokens[0])
	}
	if tokens[1] != "tok" {
		t.Errorf("object instance token = %v, want tok", tokens[1])
	}
}

// Settings that are not JSON are reported rather than replaced: overwriting
// would destroy the user's editor configuration.
func TestRunReportsUnparsableSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil { //nolint:gosec // matches the seeded mode
		t.Fatal(err)
	}

	_, err := Run(path, tokenFile(t, dir, "tok"))
	if err == nil || !strings.Contains(err.Error(), "parse settings") {
		t.Errorf("error = %v, want a parse failure", err)
	}
}

// An unreadable credential is a real failure, not the fail-open case: the
// file is there, so the secret was provisioned and something is wrong with
// how it was mounted.
func TestRunReportsUnreadableToken(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads regardless of mode")
	}

	dir := t.TempDir()
	settings := settingsWith(t, dir, []any{map[string]any{"id": "home"}}, nil)
	token := tokenFile(t, dir, "tok")
	if err := os.Chmod(token, 0o000); err != nil {
		t.Fatal(err)
	}

	_, err := Run(settings, token)
	if err == nil || !strings.Contains(err.Error(), "read token") {
		t.Errorf("error = %v, want a read failure", err)
	}
}

// A directory that cannot be written to fails before the settings file is
// touched, so the original survives intact.
func TestRunReportsUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes regardless of mode")
	}

	dir := t.TempDir()
	settings := settingsWith(t, dir, []any{map[string]any{"id": "home"}}, nil)
	token := tokenFile(t, dir, "tok")
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if _, err := Run(settings, token); err == nil {
		t.Fatal("Run succeeded with an unwritable directory")
	}

	after, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("the settings file was modified despite the failure")
	}
}

// A directory in place of the settings file is not a regular file, so the
// fail-open path catches it rather than the reader.
func TestRunNoOpsOnDirectory(t *testing.T) {
	dir := t.TempDir()
	written, err := Run(dir, tokenFile(t, dir, "tok"))

	if err != nil {
		t.Errorf("Run: %v", err)
	}
	if written {
		t.Error("Run wrote with a directory as its settings file")
	}
}
