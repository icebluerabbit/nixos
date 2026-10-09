// Package watch imports .torrent files dropped in watch/<category>/.
//
// qBittorrent has its own watch-directory feature, but it files everything
// into one category. Here the subdirectory names the category, which is what
// makes a hand-drop land in the same place an *arr grab would.
//
// A hand-dropped torrent enters the client with nothing in front of it: no
// autobrr grab, no *arr grab. A cross-seed inject looks exactly the same from
// qBittorrent's side, so the absence of an upstream event cannot tell the two
// apart. ReportURL, when set, says so explicitly at the moment of import.
package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"homelab/app"
	"homelab/clients/qbittorrent"
	"homelab/httpx"
)

// Timeouts. An upload carries a file and is given longer than the provenance
// ping, which carries one small JSON object.
const (
	AddTimeout    = 15 * time.Second
	ReportTimeout = 10 * time.Second
)

// backupDirPerm is what the backup directory gets when it has to be created.
const backupDirPerm = 0o755

// Event is the name the provenance ping is recorded under.
const Event = "watch_added"

// Config is what the tool takes on the command line.
type Config struct {
	// API is the qBittorrent WebUI root.
	API string `json:"api"`
	// WatchDir holds one subdirectory per category.
	WatchDir string `json:"watch_dir"`
	// BackupDir is where an imported .torrent is moved to. Moving rather
	// than deleting is what keeps the import idempotent: the file is gone
	// from the watch directory, so the next pass does not see it, and it
	// is still on disk if the add turns out to have been wrong.
	BackupDir string `json:"backup_dir"`
	// Categories is the comma-separated list of subdirectories to scan.
	// Named explicitly rather than discovered, so a stray directory under
	// the watch root does not mint a category.
	Categories string `json:"categories"`
	// ReportURL receives one provenance ping per import. Empty disables
	// reporting.
	ReportURL string `json:"report_url"`
}

// Defaults returns the configuration before flags and the environment.
func Defaults() Config { return Config{} }

// CategoryList splits the configured list, dropping empties so that a
// trailing comma or an empty configuration does not produce a category named
// "", which would glob the watch root itself.
func (c Config) CategoryList() []string {
	var out []string
	for _, name := range strings.Split(c.Categories, ",") {
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// Run imports every .torrent waiting under each category.
func Run(ctx context.Context, env *app.Env, cfg Config) error {
	client, err := qbittorrent.New(httpx.Config{
		BaseURL:  cfg.API,
		Timeout:  AddTimeout,
		RetryMax: -1,
		Logger:   env.Logger,
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.BackupDir, backupDirPerm); err != nil {
		return fmt.Errorf("create %s: %w", cfg.BackupDir, err)
	}

	reporter := &reporter{url: cfg.ReportURL, logger: env.Logger}

	for _, category := range cfg.CategoryList() {
		matches, err := filepath.Glob(filepath.Join(cfg.WatchDir, category, "*.torrent"))
		if err != nil {
			return fmt.Errorf("scan %s: %w", category, err)
		}
		for _, path := range matches {
			importOne(ctx, env, client, reporter, cfg, category, path)
		}
	}
	return nil
}

// importOne handles one file. A failure here is logged and skipped rather
// than returned: one bad .torrent must not stop the rest of the drop, and the
// file stays in the watch directory so the next pass tries again.
func importOne(
	ctx context.Context,
	env *app.Env,
	client *qbittorrent.Client,
	rep *reporter,
	cfg Config,
	category, path string,
) {
	name := filepath.Base(path)

	content, err := os.ReadFile(path) //nolint:gosec // the path came from the configured watch directory
	if err != nil {
		env.Logger.Error("read failed", slog.String("path", path), slog.Any("error", err))
		return
	}

	if env.DryRun {
		env.Logger.Info("would add", slog.String("name", name),
			slog.String("category", category))
		return
	}

	accepted, err := client.AddTorrentFile(ctx, name, content, qbittorrent.AddOptions{
		Category: category,
		// The category's save path is the whole point of filing by
		// subdirectory, and only automatic management applies it.
		AutoTMM: true,
	})
	if err != nil {
		env.Logger.Error("add failed", slog.String("path", path), slog.Any("error", err))
		return
	}
	if !accepted {
		env.Logger.Warn("add rejected", slog.String("path", path))
		return
	}

	if err := move(path, filepath.Join(cfg.BackupDir, name)); err != nil {
		// The torrent is in the client. Leaving the file where it is
		// would re-add it next pass, which qBittorrent refuses, so this
		// is noisy rather than harmful.
		env.Logger.Error("backup failed", slog.String("path", path), slog.Any("error", err))
		return
	}

	env.Logger.Info("added", slog.String("name", name), slog.String("category", category))
	rep.send(ctx, name, category)
}

// rename is a seam. The fallback below runs only when a rename crosses a
// filesystem boundary, which a test cannot arrange portably.
var rename = os.Rename

// move relocates a file, falling back to copy-and-remove across filesystems.
// The watch directory and the export directory are often separate mounts, and
// rename cannot cross one.
func move(src, dst string) error {
	if err := rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// copyFile writes src's contents to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // src is the file just imported
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only

	out, err := os.Create(dst) //nolint:gosec // dst is under the configured backup directory
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck,gosec // the copy already failed
		return err
	}
	return out.Close()
}

// reporter sends the provenance ping.
type reporter struct {
	url    string
	logger *slog.Logger
}

// send records one import. Best-effort by construction: the import already
// succeeded, and a recorder being down must not turn that into a failure.
func (r *reporter) send(ctx context.Context, name, category string) {
	if r.url == "" {
		return
	}

	body, err := json.Marshal(map[string]string{
		"event":    Event,
		"name":     name,
		"category": category,
	})
	if err != nil {
		r.logger.Warn("report failed", slog.String("name", name), slog.Any("error", err))
		return
	}

	ctx, cancel := context.WithTimeout(ctx, ReportTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		r.logger.Warn("report failed", slog.String("name", name), slog.Any("error", err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.logger.Warn("report failed", slog.String("name", name), slog.Any("error", err))
		return
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body
	_, _ = io.Copy(io.Discard, resp.Body)
}
