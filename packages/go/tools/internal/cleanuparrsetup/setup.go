// Package cleanuparrsetup completes Cleanuparr's first-run setup and exports
// its API key for downstream consumers (configuratarr).
//
// Idempotent by design: /api/auth/status is unauthenticated and the admin
// account lives in PostgreSQL rather than the container's ephemeral rootfs, so
// a reboot re-runs this harmlessly.
package cleanuparrsetup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"homelab/app"
	"homelab/clients/cleanuparr"
	"homelab/httpx"
	"homelab/secret"
)

// keyFileMode keeps the exported key readable only by its owner, matching the
// `umask 077` this replaces.
const keyFileMode = 0o600

// Config is what the tool takes on the command line.
//
// Each credential is a --x / --x-file pair so the host decides whether the
// value is a secret: Cleanuparr's admin username is already public in this
// repo, while its password lives in sops. Only paths and literals appear here
// — a resolved secret never enters Config, which is what keeps it out of the
// --help listing and out of any config file.
type Config struct {
	// Base is the Cleanuparr root URL.
	Base string `json:"base"`
	// KeyFile is where CLEANUPARR_API_KEY=… is written.
	KeyFile string `json:"key_file"`

	// AdminUser is the admin account name, given literally.
	AdminUser string `json:"admin_user"`
	// AdminUserFile is a file holding the admin account name.
	AdminUserFile string `json:"admin_user_file"`
	// AdminPassword is the admin password, given literally. Present for
	// symmetry; prefer AdminPasswordFile, since a literal lands in argv.
	AdminPassword string `json:"admin_password"`
	// AdminPasswordFile is a file holding the admin password — a sops
	// secret path, or a systemd credential under /run/host/credentials.
	AdminPasswordFile string `json:"admin_password_file"`
}

// Defaults matches the shell script this replaces, so a unit that passes no
// flags reaches the same instance and writes the same file.
func Defaults() Config {
	return Config{
		Base:    "http://127.0.0.1:11011",
		KeyFile: "/run/cleanuparr/api-key.env",
	}
}

// Credentials resolves the admin credentials from the flag pairs.
func (c Config) Credentials() (cleanuparr.Credentials, error) {
	user, err := secret.Flag("admin-user", c.AdminUser, c.AdminUserFile)
	if err != nil {
		return cleanuparr.Credentials{}, err
	}
	password, err := secret.Flag("admin-password", c.AdminPassword, c.AdminPasswordFile)
	if err != nil {
		return cleanuparr.Credentials{}, err
	}
	return cleanuparr.Credentials{
		Username: user.Reveal(),
		Password: password,
	}, nil
}

// Run performs the setup dance and writes the key file.
func Run(ctx context.Context, env *app.Env, cfg Config) error {
	creds, err := cfg.Credentials()
	if err != nil {
		return err
	}

	client, err := cleanuparr.New(httpx.Config{
		BaseURL:   cfg.Base,
		UserAgent: "cleanuparr-setup",
		Logger:    env.Logger,
	})
	if err != nil {
		return err
	}

	env.Logger.Info("waiting for cleanuparr", slog.String("base", cfg.Base))
	if err := client.WaitReady(ctx, 0); err != nil {
		return err
	}

	if err := ensureSetup(ctx, env, client, creds); err != nil {
		return err
	}

	token, err := client.Login(ctx, creds)
	if err != nil {
		return err
	}
	key, err := client.APIKey(ctx, token)
	if err != nil {
		return err
	}

	if env.DryRun {
		env.Logger.Info("dry run: not writing the key file",
			slog.String("path", cfg.KeyFile))
		return nil
	}
	if err := WriteKeyFile(cfg.KeyFile, key); err != nil {
		return err
	}

	env.Logger.Info("exported api key", slog.String("path", cfg.KeyFile))
	return nil
}

// ensureSetup creates the admin account unless setup has already been done.
func ensureSetup(
	ctx context.Context,
	env *app.Env,
	client *cleanuparr.Client,
	creds cleanuparr.Credentials,
) error {
	done, err := client.SetupCompleted(ctx)
	if err != nil {
		return err
	}
	if done {
		env.Logger.Info("first-run setup already completed")
		return nil
	}

	if env.DryRun {
		env.Logger.Info("dry run: would create the admin account")
		return nil
	}

	env.Logger.Info("completing first-run setup")
	return client.CompleteSetup(ctx, creds)
}

// WriteKeyFile writes the environment fragment configuratarr reads.
//
// The file is created 0600 and replaced atomically: configuratarr.service reads
// it as an EnvironmentFile, and a torn write would hand it half a key.
func WriteKeyFile(path string, key secret.Value) error {
	if key.IsZero() {
		return errors.New("cleanuparr-setup: refusing to write an empty api key")
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".api-key-*.env")
	if err != nil {
		return fmt.Errorf("cleanuparr-setup: create temp file in %s: %w", dir, err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // best effort on the happy path

	if err := tmp.Chmod(keyFileMode); err != nil {
		tmp.Close() //nolint:errcheck,gosec // the error below is the one that matters
		return fmt.Errorf("cleanuparr-setup: chmod %s: %w", tmp.Name(), err)
	}
	// Reveal, explicitly: %s on a secret.Value writes [REDACTED], which would
	// hand configuratarr a valid-looking file containing nothing usable.
	if _, err := fmt.Fprintf(tmp, "CLEANUPARR_API_KEY=%s\n", key.Reveal()); err != nil {
		tmp.Close() //nolint:errcheck,gosec // the error below is the one that matters
		return fmt.Errorf("cleanuparr-setup: write %s: %w", tmp.Name(), err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cleanuparr-setup: close %s: %w", tmp.Name(), err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("cleanuparr-setup: install %s: %w", path, err)
	}
	return nil
}
