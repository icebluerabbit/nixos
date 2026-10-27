package seed

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"homelab/app"
)

func testEnv() *app.Env {
	return &app.Env{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Stdout: io.Discard,
	}
}

// writeJSON puts a JSON document on disk and returns its path.
func writeJSON(t *testing.T, dir, name string, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// writeSecret puts a credential on disk with the trailing newline a sops
// file carries, which the reader has to strip.
func writeSecret(t *testing.T, dir, name, value string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// readUserJSON reads back what the tool wrote for a user.
func readUserJSON(t *testing.T, dir, user string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, user+".json"))
	if err != nil {
		t.Fatalf("read %s.json: %v", user, err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parse %s.json: %v", user, err)
	}
	return data
}

// networksOf pulls the networks array out of a user file.
func networksOf(t *testing.T, data map[string]any) []map[string]any {
	t.Helper()
	raw, ok := data["networks"].([]any)
	if !ok {
		t.Fatalf("networks is %T, want an array", data["networks"])
	}
	out := make([]map[string]any, 0, len(raw))
	for _, n := range raw {
		net, ok := n.(map[string]any)
		if !ok {
			t.Fatalf("network is %T, want an object", n)
		}
		out = append(out, net)
	}
	return out
}

// The uuid is what thelounge keys scrollback by, so it is pinned against the
// values the Python implementation produced. A change here silently orphans
// every user's history.
func TestNetworkUUIDMatchesTheDeclaredIdentity(t *testing.T) {
	cases := map[string]struct{ user, network, want string }{
		"libera": {"zms", "libera", "b597a5da-14f0-5f4c-a64e-70ce5877e1c9"},
		"oftc":   {"other", "oftc", "4836ab7e-6bb6-5353-a59d-1aea41df6101"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := NetworkUUID(tc.user, tc.network); got != tc.want {
				t.Errorf("NetworkUUID(%q, %q) = %s, want %s", tc.user, tc.network, got, tc.want)
			}
		})
	}
}

// Re-running must produce the same uuid, which is the whole reason it is
// derived rather than generated.
func TestNetworkUUIDIsStable(t *testing.T) {
	first := NetworkUUID("zms", "libera")
	again := NetworkUUID("zms", "libera")
	if first != again {
		t.Errorf("NetworkUUID is not deterministic: %s then %s", first, again)
	}
	if first == NetworkUUID("zms", "oftc") {
		t.Error("two networks share a uuid")
	}
	if first == NetworkUUID("other", "libera") {
		t.Error("two users share a network uuid")
	}
}

func TestLoadRequiresSeedFile(t *testing.T) {
	t.Setenv(EnvSeedFile, "")

	if _, err := Load(); err == nil {
		t.Fatal("Load succeeded with no seed file configured")
	}
}

func TestLoadDefaultsUsersDir(t *testing.T) {
	t.Setenv(EnvSeedFile, "/seed.json")
	t.Setenv(EnvUsersDir, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UsersDir != DefaultUsersDir {
		t.Errorf("UsersDir = %q, want %q", cfg.UsersDir, DefaultUsersDir)
	}

	t.Setenv(EnvUsersDir, "/elsewhere")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UsersDir != "/elsewhere" {
		t.Errorf("UsersDir = %q, want /elsewhere", cfg.UsersDir)
	}
}

// A network states only what differs from the defaults, so the rest have to
// be filled in.
func TestRunAppliesNetworkDefaults(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	seedFile := writeJSON(t, dir, "seed.json", []User{{
		Name:     "zms",
		Networks: []Network{{Name: "libera", Host: "irc.libera.chat"}},
	}})

	if err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: users}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	net := networksOf(t, readUserJSON(t, users, "zms"))[0]
	if net["port"] != float64(defaultPort) {
		t.Errorf("port = %v, want %d", net["port"], defaultPort)
	}
	if net["tls"] != true || net["rejectUnauthorized"] != true {
		t.Errorf("tls/rejectUnauthorized = %v/%v, want true/true", net["tls"], net["rejectUnauthorized"])
	}
	if net["uuid"] != NetworkUUID("zms", "libera") {
		t.Errorf("uuid = %v", net["uuid"])
	}
}

// An explicit false has to survive, which is why the declaration uses
// pointers: a plain bool could not tell it apart from an absent value.
func TestRunKeepsExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	no := false
	port := 6667
	seedFile := writeJSON(t, dir, "seed.json", []User{{
		Name: "zms",
		Networks: []Network{{
			Name: "local", Host: "irc.local",
			Port: &port, TLS: &no, RejectUnauthorized: &no,
		}},
	}})

	if err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: users}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	net := networksOf(t, readUserJSON(t, users, "zms"))[0]
	if net["port"] != float64(6667) {
		t.Errorf("port = %v, want 6667", net["port"])
	}
	if net["tls"] != false || net["rejectUnauthorized"] != false {
		t.Errorf("tls/rejectUnauthorized = %v/%v, want false/false", net["tls"], net["rejectUnauthorized"])
	}
}

// Secrets arrive as paths and are read at runtime, trailing newline stripped.
func TestRunReadsSecretsByPath(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	seedFile := writeJSON(t, dir, "seed.json", []User{{
		Name: "zms",
		Networks: []Network{{
			Name: "libera", Host: "irc.libera.chat",
			NickPath:         writeSecret(t, dir, "nick", "zms"),
			PasswordPath:     writeSecret(t, dir, "pass", "server-password"),
			SASL:             "plain",
			SASLAccountPath:  writeSecret(t, dir, "account", "zms-account"),
			SASLPasswordPath: writeSecret(t, dir, "saslpass", "sasl-password"),
			Channels:         []string{"#one", "#two"},
		}},
	}})

	if err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: users}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	net := networksOf(t, readUserJSON(t, users, "zms"))[0]
	for key, want := range map[string]string{
		"nick":         "zms",
		"password":     "server-password",
		"sasl":         "plain",
		"saslAccount":  "zms-account",
		"saslPassword": "sasl-password",
	} {
		if net[key] != want {
			t.Errorf("%s = %v, want %q", key, net[key], want)
		}
	}

	channels, ok := net["channels"].([]any)
	if !ok || len(channels) != 2 {
		t.Fatalf("channels = %v", net["channels"])
	}
	first, _ := channels[0].(map[string]any)
	if first["name"] != "#one" {
		t.Errorf("first channel = %v, want #one", first)
	}
}

// SASL credentials belong to the sasl block: without sasl set they must not
// be written at all.
func TestRunSkipsSASLCredentialsWhenSASLIsOff(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	seedFile := writeJSON(t, dir, "seed.json", []User{{
		Name: "zms",
		Networks: []Network{{
			Name: "libera", Host: "irc.libera.chat",
			SASLAccountPath: writeSecret(t, dir, "account", "unused"),
		}},
	}})

	if err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: users}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	net := networksOf(t, readUserJSON(t, users, "zms"))[0]
	if _, ok := net["saslAccount"]; ok {
		t.Errorf("saslAccount written without sasl: %v", net)
	}
}

func TestRunExpandsCommandSecrets(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	seedFile := writeJSON(t, dir, "seed.json", []User{{
		Name: "zms",
		Networks: []Network{{
			Name: "libera", Host: "irc.libera.chat",
			Commands:            []string{"/msg NickServ IDENTIFY {nickpass}"},
			CommandsSecretPaths: map[string]string{"nickpass": writeSecret(t, dir, "nickpass", "hunter2")},
		}},
	}})

	if err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: users}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	net := networksOf(t, readUserJSON(t, users, "zms"))[0]
	commands, ok := net["commands"].([]any)
	if !ok || len(commands) != 1 {
		t.Fatalf("commands = %v", net["commands"])
	}
	if commands[0] != "/msg NickServ IDENTIFY hunter2" {
		t.Errorf("command = %v", commands[0])
	}
}

// A placeholder with no secret behind it would be sent to the network as
// written, which for a NickServ line leaks the literal text where a password
// belongs. It is refused instead.
func TestRunRefusesUnfilledPlaceholder(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	seedFile := writeJSON(t, dir, "seed.json", []User{{
		Name: "zms",
		Networks: []Network{{
			Name: "libera", Host: "irc.libera.chat",
			Commands: []string{"/msg NickServ IDENTIFY {missing}"},
		}},
	}})

	err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: users})
	if err == nil || !strings.Contains(err.Error(), "{missing}") {
		t.Errorf("error = %v, want the unfilled placeholder named", err)
	}
}

// A network with no commands writes no commands key, rather than an empty
// array thelounge would have to interpret.
func TestRunOmitsEmptyCommands(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	seedFile := writeJSON(t, dir, "seed.json", []User{{
		Name:     "zms",
		Networks: []Network{{Name: "libera", Host: "irc.libera.chat"}},
	}})

	if err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: users}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	net := networksOf(t, readUserJSON(t, users, "zms"))[0]
	if _, ok := net["commands"]; ok {
		t.Errorf("empty commands were written: %v", net)
	}
}

// This is the reason the tool merges rather than templates: thelounge writes
// sessions and settings into the same file, and a re-seed must not lose them.
func TestRunPreservesExistingUserState(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	if err := os.MkdirAll(users, 0o700); err != nil {
		t.Fatal(err)
	}
	existing := map[string]any{
		"password":       "$2a$hashed",
		"log":            true,
		"sessions":       map[string]any{"token": map[string]any{"ip": "10.0.0.1"}},
		"clientSettings": map[string]any{"theme": "morning", "notification": true},
		"networks":       []any{map[string]any{"name": "stale"}},
	}
	writeJSON(t, users, "zms.json", existing)

	seedFile := writeJSON(t, dir, "seed.json", []User{{
		Name:           "zms",
		Networks:       []Network{{Name: "libera", Host: "irc.libera.chat"}},
		ClientSettings: map[string]any{"theme": "default"},
	}})

	if err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: users}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data := readUserJSON(t, users, "zms")
	if data["password"] != "$2a$hashed" {
		t.Errorf("password was lost: %v", data["password"])
	}
	if _, ok := data["sessions"]; !ok {
		t.Error("sessions were lost")
	}

	settings, _ := data["clientSettings"].(map[string]any)
	if settings["theme"] != "default" {
		t.Errorf("declared setting did not win: %v", settings["theme"])
	}
	if settings["notification"] != true {
		t.Errorf("undeclared setting was dropped: %v", settings)
	}

	networks := networksOf(t, data)
	if len(networks) != 1 || networks[0]["name"] != "libera" {
		t.Errorf("networks were not replaced: %v", networks)
	}
}

// A user thelounge has never seen gets the shape it expects.
func TestRunCreatesMissingUserFile(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	seedFile := writeJSON(t, dir, "seed.json", []User{{Name: "fresh"}})

	if err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: users}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data := readUserJSON(t, users, "fresh")
	if data["password"] != "" || data["log"] != true {
		t.Errorf("new user file = %v", data)
	}
}

// The write goes through a temporary file, so no .tmp may be left behind.
func TestRunLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	seedFile := writeJSON(t, dir, "seed.json", []User{{
		Name:     "zms",
		Networks: []Network{{Name: "libera", Host: "irc.libera.chat"}},
	}})

	if err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: users}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries, err := os.ReadDir(users)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

// Seeding is idempotent: the second run must produce the same file, which is
// what makes running it on every boot safe.
func TestRunIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	seedFile := writeJSON(t, dir, "seed.json", []User{{
		Name: "zms",
		Networks: []Network{{
			Name: "libera", Host: "irc.libera.chat",
			Channels: []string{"#one"},
		}},
	}})
	cfg := Config{SeedFile: seedFile, UsersDir: users}

	if err := Run(testEnv(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(users, "zms.json"))
	if err != nil {
		t.Fatal(err)
	}

	if err := Run(testEnv(), cfg); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(users, "zms.json"))
	if err != nil {
		t.Fatal(err)
	}

	if string(first) != string(second) {
		t.Errorf("second run changed the file:\n%s\n---\n%s", first, second)
	}
}

func TestRunReportsMissingSeedFile(t *testing.T) {
	dir := t.TempDir()

	err := Run(testEnv(), Config{
		SeedFile: filepath.Join(dir, "nope.json"),
		UsersDir: filepath.Join(dir, "users"),
	})
	if err == nil || !strings.Contains(err.Error(), "nope.json") {
		t.Errorf("error = %v, want the missing file named", err)
	}
}

func TestRunReportsUnparsableSeedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Run(testEnv(), Config{SeedFile: path, UsersDir: filepath.Join(dir, "users")})
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("error = %v, want a parse failure", err)
	}
}

// A secret named but absent is a deployment error, and seeding a network
// without the credential it was told to use would silently connect wrong.
func TestRunReportsMissingSecret(t *testing.T) {
	dir := t.TempDir()
	seedFile := writeJSON(t, dir, "seed.json", []User{{
		Name: "zms",
		Networks: []Network{{
			Name: "libera", Host: "irc.libera.chat",
			NickPath: filepath.Join(dir, "absent"),
		}},
	}})

	err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: filepath.Join(dir, "users")})
	if err == nil || !strings.Contains(err.Error(), "absent") {
		t.Errorf("error = %v, want the missing secret named", err)
	}
}

// An existing user file that is not JSON is reported rather than silently
// replaced, because replacing it would destroy the sessions inside it.
func TestRunReportsUnparsableUserFile(t *testing.T) {
	dir := t.TempDir()
	users := filepath.Join(dir, "users")
	if err := os.MkdirAll(users, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(users, "zms.json"), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	seedFile := writeJSON(t, dir, "seed.json", []User{{Name: "zms"}})

	err := Run(testEnv(), Config{SeedFile: seedFile, UsersDir: users})
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("error = %v, want a parse failure", err)
	}
}
