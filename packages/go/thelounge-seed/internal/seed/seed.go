// Package seed merges declared IRC networks into thelounge's per-user JSON.
//
// thelounge owns users/<name>.json and rewrites it at runtime, so the declared
// set is merged back in on every boot rather than templated once. Only the
// networks key is replaced: sessions, clientSettings and every other key the
// server put there are preserved, because losing them would log the user out
// and drop their preferences.
//
// Nicks and passwords are read from bind-mounted sops files at runtime, so
// they are named here by path and never enter the Nix store.
package seed

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"homelab/app"
)

// Environment variables. These names are the contract with the container
// unit in modules/containers/thelounge.nix, which sets the first and relies
// on the default for the second.
const (
	// EnvSeedFile names the JSON file describing the declared users.
	EnvSeedFile = "THELOUNGE_SEED_FILE"
	// EnvUsersDir names thelounge's per-user state directory.
	EnvUsersDir = "THELOUNGE_USERS_DIR"
)

// DefaultUsersDir is where thelounge keeps its per-user files.
const DefaultUsersDir = "/var/lib/thelounge/users"

// Defaults for a network the declaration leaves unset. They match what a
// modern IRC network expects, so a declaration only states what differs.
const (
	defaultPort               = 6697
	defaultTLS                = true
	defaultRejectUnauthorized = true
)

// filePerm is what a written user file gets. thelounge runs as its own user
// and the file holds credentials, so it is not world-readable.
const filePerm = 0o600

// dirPerm is what a created users directory gets.
const dirPerm = 0o700

// ErrNoSeedFile reports a run with nothing to seed from.
var ErrNoSeedFile = errors.New("seed: " + EnvSeedFile + " is not set")

// Config is what the tool reads from the environment.
type Config struct {
	// SeedFile is the declared users, as JSON.
	SeedFile string
	// UsersDir is thelounge's per-user state directory.
	UsersDir string
}

// Load reads the configuration from the environment.
func Load() (Config, error) {
	cfg := Config{
		SeedFile: os.Getenv(EnvSeedFile),
		UsersDir: os.Getenv(EnvUsersDir),
	}
	if cfg.SeedFile == "" {
		return Config{}, ErrNoSeedFile
	}
	if cfg.UsersDir == "" {
		cfg.UsersDir = DefaultUsersDir
	}
	return cfg, nil
}

// User is one declared thelounge user.
type User struct {
	Name           string         `json:"name"`
	Networks       []Network      `json:"networks"`
	ClientSettings map[string]any `json:"clientSettings"`
}

// Network is one declared IRC network.
//
// The pointer fields are how a declaration says nothing rather than saying
// false: port, tls and rejectUnauthorized all default to on, so an absent
// value and an explicit false have to stay distinguishable.
type Network struct {
	Name               string   `json:"name"`
	Host               string   `json:"host"`
	Port               *int     `json:"port"`
	TLS                *bool    `json:"tls"`
	RejectUnauthorized *bool    `json:"rejectUnauthorized"`
	Channels           []string `json:"channels"`

	// Paths to secrets, read at runtime.
	NickPath         string `json:"nickPath"`
	PasswordPath     string `json:"passwordPath"`
	SASL             string `json:"sasl"`
	SASLAccountPath  string `json:"saslAccountPath"`
	SASLPasswordPath string `json:"saslPasswordPath"`

	// Commands are sent on connect, with {name} placeholders filled from
	// CommandsSecretPaths.
	Commands            []string          `json:"commands"`
	CommandsSecretPaths map[string]string `json:"commandsSecretPaths"`
}

// Run seeds every declared user.
func Run(env *app.Env, cfg Config) error {
	users, err := readUsers(cfg.SeedFile)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.UsersDir, dirPerm); err != nil {
		return fmt.Errorf("create %s: %w", cfg.UsersDir, err)
	}

	for _, u := range users {
		networks, err := buildNetworks(u)
		if err != nil {
			return err
		}
		if err := writeUser(cfg.UsersDir, u, networks); err != nil {
			return err
		}
		env.Logger.Info("seeded user",
			slog.String("user", u.Name),
			slog.Int("networks", len(networks)),
		)
	}
	return nil
}

// readUsers parses the declared users.
func readUsers(path string) ([]User, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is the unit's own seed file
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var users []User
	if err := json.Unmarshal(raw, &users); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return users, nil
}

// buildNetworks turns one user's declaration into the objects thelounge
// stores, reading every secret named by path.
func buildNetworks(u User) ([]map[string]any, error) {
	networks := make([]map[string]any, 0, len(u.Networks))
	for _, n := range u.Networks {
		channels := make([]map[string]any, 0, len(n.Channels))
		for _, c := range n.Channels {
			channels = append(channels, map[string]any{"name": c})
		}

		net := map[string]any{
			// Derived from the user and the network name so that a re-run
			// never creates a second network: thelounge keys scrollback by
			// uuid, and a fresh one would orphan it.
			"uuid":               NetworkUUID(u.Name, n.Name),
			"name":               n.Name,
			"host":               n.Host,
			"port":               valueOr(n.Port, defaultPort),
			"tls":                valueOr(n.TLS, defaultTLS),
			"rejectUnauthorized": valueOr(n.RejectUnauthorized, defaultRejectUnauthorized),
			"channels":           channels,
		}

		for _, s := range []struct {
			key  string
			path string
		}{
			{"nick", n.NickPath},
			{"password", n.PasswordPath},
		} {
			if s.path == "" {
				continue
			}
			value, err := readSecret(s.path)
			if err != nil {
				return nil, err
			}
			net[s.key] = value
		}

		if n.SASL != "" {
			net["sasl"] = n.SASL
			for _, s := range []struct {
				key  string
				path string
			}{
				{"saslAccount", n.SASLAccountPath},
				{"saslPassword", n.SASLPasswordPath},
			} {
				if s.path == "" {
					continue
				}
				value, err := readSecret(s.path)
				if err != nil {
					return nil, err
				}
				net[s.key] = value
			}
		}

		commands, err := expandCommands(n)
		if err != nil {
			return nil, err
		}
		if len(commands) > 0 {
			net["commands"] = commands
		}

		networks = append(networks, net)
	}
	return networks, nil
}

// NetworkUUID is the identity thelounge keys a network's scrollback by.
//
// Version 5 over the URL namespace, from the user and network name, so the
// same declaration always yields the same uuid on every host and every run.
func NetworkUUID(user, network string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL,
		[]byte(fmt.Sprintf("thelounge/%s/%s", user, network))).String()
}

// expandCommands fills the {name} placeholders in a network's commands from
// the secrets it names.
func expandCommands(n Network) ([]string, error) {
	if len(n.Commands) == 0 {
		return nil, nil
	}

	replacements := make([]string, 0, 2*len(n.CommandsSecretPaths))
	for key, path := range n.CommandsSecretPaths {
		value, err := readSecret(path)
		if err != nil {
			return nil, err
		}
		replacements = append(replacements, "{"+key+"}", value)
	}
	replacer := strings.NewReplacer(replacements...)

	commands := make([]string, 0, len(n.Commands))
	for _, c := range n.Commands {
		expanded := replacer.Replace(c)
		// A placeholder with no secret behind it would otherwise be sent to
		// the network verbatim, which for a NickServ line means leaking the
		// literal text where a password belongs.
		if i := strings.IndexByte(expanded, '{'); i >= 0 {
			return nil, fmt.Errorf(
				"network %q: command has an unfilled placeholder: %s",
				n.Name, expanded[i:],
			)
		}
		commands = append(commands, expanded)
	}
	return commands, nil
}

// readSecret reads a credential from the file the declaration names.
func readSecret(path string) (string, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is named by the host's own declaration
	if err != nil {
		return "", fmt.Errorf("read secret %s: %w", path, err)
	}
	return strings.TrimSpace(string(raw)), nil
}

// writeUser merges the networks into the user's file, preserving everything
// else already in it.
func writeUser(dir string, u User, networks []map[string]any) error {
	path := filepath.Join(dir, u.Name+".json")

	data, err := readUserFile(path)
	if err != nil {
		return err
	}
	data["networks"] = networks

	if len(u.ClientSettings) > 0 {
		settings, _ := data["clientSettings"].(map[string]any)
		if settings == nil {
			settings = map[string]any{}
		}
		for k, v := range u.ClientSettings {
			settings[k] = v
		}
		data["clientSettings"] = settings
	}

	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}

	// Written beside the target and renamed over it: thelounge may read this
	// file at any moment, and a half-written one would lose the user.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, encoded, filePerm); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s: %w", tmp, err)
	}
	return nil
}

// readUserFile returns the user's existing state, or the shape thelounge
// expects for a user it has not seen yet.
func readUserFile(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is inside the configured users dir
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{"password": "", "log": true}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return data, nil
}

// valueOr returns what the declaration said, or the default when it said
// nothing.
func valueOr[T any](v *T, fallback T) T {
	if v == nil {
		return fallback
	}
	return *v
}
