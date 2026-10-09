// Package reclaim hands stranded Soulseek album folders back to Lidarr.
//
// A soularr run only imports what it grabbed in that same run: it tags each
// downloaded album with the Lidarr metadata it already has, then calls
// DownloadedAlbumsScan on that one folder. An album stranded by a crash, an
// NFS stall or an earlier whole-list sweep therefore sits in the download
// directory forever — real music nothing will ever look at again.
//
// This walks that directory and replays the second half of the flow, the
// scan, once per album folder. It cannot replay the first half: the album
// identity soularr had at grab time is gone, so Lidarr has to recognise each
// folder from its existing tags and name alone. Well-tagged rips import; the
// rest stay put and are reported.
//
// The parent directory is not a valid scan target — Lidarr finds no audio
// directly inside it and answers "Failed to import" in about 13ms. One
// command per album folder is the only shape that works.
//
// Nothing here moves or deletes anything. Lidarr does the moving, under its
// own import rules.
package reclaim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"homelab/app"
)

// Environment variables. These names are the contract with the container
// unit in modules/containers/soularr/soularr.nix.
const (
	EnvLidarrURL      = "LIDARR_URL"
	EnvLidarrKeyFile  = "LIDARR_KEY_FILE"
	EnvDownloadDir    = "DOWNLOAD_DIR"
	EnvMinAgeMinutes  = "RECLAIM_MIN_AGE_MINUTES"
	EnvMaxFolders     = "RECLAIM_MAX_FOLDERS"
	EnvCommandTimeout = "RECLAIM_COMMAND_TIMEOUT"
)

// Defaults for the tunables.
const (
	// DefaultMinAgeMinutes keeps the scan off an album that may still be
	// landing: slskd writes completed files into the directory one at a
	// time, so a fresh folder is not necessarily a whole album.
	DefaultMinAgeMinutes = 120
	// DefaultMaxFolders is no cap.
	DefaultMaxFolders = 0
	// DefaultCommandTimeout bounds the wait for one Lidarr command.
	DefaultCommandTimeout = 120
)

// pollInterval is how often a running command is re-checked.
const pollInterval = 2 * time.Second

// failedImports is soularr's own denylist directory. Nothing retries what
// is in there, so neither does this.
const failedImports = "failed_imports"

// audioSuffixes are what makes a folder worth handing over. Anything else
// is artwork or an nfo left behind.
var audioSuffixes = map[string]bool{
	".flac": true, ".mp3": true, ".m4a": true, ".ogg": true, ".opus": true,
	".wav": true, ".aac": true, ".wma": true, ".alac": true, ".ape": true,
}

// settledStatuses are the Lidarr command states that mean it is over.
var settledStatuses = map[string]bool{
	"completed": true, "failed": true, "aborted": true, "cancelled": true,
}

// Config is what the tool reads from the environment.
type Config struct {
	LidarrURL      string
	KeyFile        string
	DownloadDir    string
	MinAge         time.Duration
	MaxFolders     int
	CommandTimeout time.Duration
}

// Options tune the poll loop and the HTTP client, for tests.
type Options struct {
	PollInterval time.Duration
	Client       *http.Client
	// Now is the clock the age check uses.
	Now func() time.Time
}

// withDefaults fills in whatever the caller left unset.
func (o Options) withDefaults() Options {
	if o.PollInterval <= 0 {
		o.PollInterval = pollInterval
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 30 * time.Second}
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return o
}

// Load reads the configuration from the environment.
func Load(logger *slog.Logger) (Config, error) {
	cfg := Config{
		LidarrURL:   strings.TrimRight(os.Getenv(EnvLidarrURL), "/"),
		KeyFile:     os.Getenv(EnvLidarrKeyFile),
		DownloadDir: os.Getenv(EnvDownloadDir),
	}
	for name, value := range map[string]string{
		EnvLidarrURL:     cfg.LidarrURL,
		EnvLidarrKeyFile: cfg.KeyFile,
		EnvDownloadDir:   cfg.DownloadDir,
	} {
		if value == "" {
			return Config{}, fmt.Errorf("reclaim: %s is not set", name)
		}
	}

	cfg.MinAge = time.Duration(envInt(logger, EnvMinAgeMinutes, DefaultMinAgeMinutes)) * time.Minute
	cfg.MaxFolders = envInt(logger, EnvMaxFolders, DefaultMaxFolders)
	cfg.CommandTimeout = time.Duration(envInt(logger, EnvCommandTimeout, DefaultCommandTimeout)) * time.Second
	return cfg, nil
}

// envInt reads an integer setting, falling back to the default rather than
// failing: a typo in a unit's environment should not stop the sweep.
func envInt(logger *slog.Logger, name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		logger.Warn("ignoring non-integer setting",
			slog.String("name", name),
			slog.String("value", raw),
			slog.Int("using", fallback),
		)
		return fallback
	}
	return n
}

// Run hands every eligible folder to Lidarr, one at a time.
func Run(ctx context.Context, env *app.Env, cfg Config, opts Options) error {
	opts = opts.withDefaults()

	rawKey, err := os.ReadFile(cfg.KeyFile) //nolint:gosec // the path is the unit's own bound secret
	if err != nil {
		return fmt.Errorf("read %s: %w", cfg.KeyFile, err)
	}
	key := strings.TrimSpace(string(rawKey))

	folders, err := candidates(env.Logger, cfg, opts)
	if err != nil {
		return err
	}

	if cfg.MaxFolders > 0 && len(folders) > cfg.MaxFolders {
		env.Logger.Info("capping this run",
			slog.Int("cap", cfg.MaxFolders),
			slog.Int("eligible", len(folders)),
		)
		folders = folders[:cfg.MaxFolders]
	}

	env.Logger.Info("folders to hand to Lidarr",
		slog.Int("count", len(folders)),
		slog.String("dir", cfg.DownloadDir),
	)

	imported, failed := 0, 0
	for _, folder := range folders {
		name := filepath.Base(folder)

		ok, detail, err := scanFolder(ctx, opts, cfg, key, folder)
		switch {
		case err != nil:
			failed++
			env.Logger.Error("scan failed", slog.String("folder", name), slog.Any("error", err))
		case ok:
			imported++
			env.Logger.Info("imported", slog.String("folder", name))
		default:
			// Lidarr reports a scan that matched nothing as a completed
			// command with result "unsuccessful". That is a normal outcome
			// here, not an error: the folder stays where it is.
			failed++
			env.Logger.Info("no match", slog.String("folder", name), slog.String("detail", detail))
		}
	}

	env.Logger.Info("done",
		slog.Int("imported", imported),
		slog.Int("left_in_place", failed),
		slog.Int("attempted", len(folders)),
	)
	return nil
}

// candidates returns the album folders worth handing over, longest-stranded
// first, so a capped run works through the oldest backlog.
func candidates(logger *slog.Logger, cfg Config, opts Options) ([]string, error) {
	entries, err := os.ReadDir(cfg.DownloadDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", cfg.DownloadDir, err)
	}

	type candidate struct {
		path  string
		mtime time.Time
	}
	var found []candidate

	now := opts.Now()
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == failedImports {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		age := now.Sub(info.ModTime())
		if age < cfg.MinAge {
			logger.Info("skipping folder",
				slog.String("folder", entry.Name()),
				slog.String("reason", "too fresh"),
				slog.Int("age_seconds", int(age.Seconds())),
			)
			continue
		}

		path := filepath.Join(cfg.DownloadDir, entry.Name())
		if !hasAudio(path) {
			logger.Info("skipping folder",
				slog.String("folder", entry.Name()),
				slog.String("reason", "no audio"),
			)
			continue
		}
		found = append(found, candidate{path: path, mtime: info.ModTime()})
	}

	sort.Slice(found, func(i, j int) bool { return found[i].mtime.Before(found[j].mtime) })

	paths := make([]string, 0, len(found))
	for _, c := range found {
		paths = append(paths, c.path)
	}
	return paths, nil
}

// hasAudio reports whether a folder holds an audio file at any depth.
// Multi-disc rips nest their tracks in per-disc subfolders.
func hasAudio(folder string) bool {
	found := false
	_ = filepath.WalkDir(folder, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // an unreadable entry is simply not audio
		}
		if audioSuffixes[strings.ToLower(filepath.Ext(d.Name()))] {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// command is the part of a Lidarr command this cares about.
type command struct {
	ID      int    `json:"id"`
	Status  string `json:"status"`
	Result  string `json:"result"`
	Message string `json:"message"`
}

// scanFolder fires DownloadedAlbumsScan for one folder and waits for it to
// settle, reporting whether Lidarr imported it and what it said.
func scanFolder(
	ctx context.Context, opts Options, cfg Config, key, folder string,
) (imported bool, detail string, err error) {
	cmd, err := apiRequest(ctx, opts, key, http.MethodPost, cfg.LidarrURL+"/api/v1/command",
		map[string]any{"name": "DownloadedAlbumsScan", "path": folder})
	if err != nil {
		return false, "", err
	}

	deadline := opts.Now().Add(cfg.CommandTimeout)
	status := cmd.Status
	if status == "" {
		status = "queued"
	}

	for !settledStatuses[status] {
		if opts.Now().After(deadline) {
			return false, fmt.Sprintf("timed out after %ds (command %d)",
				int(cfg.CommandTimeout.Seconds()), cmd.ID), nil
		}

		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		case <-time.After(opts.PollInterval):
		}

		cmd, err = apiRequest(ctx, opts, key, http.MethodGet,
			fmt.Sprintf("%s/api/v1/command/%d", cfg.LidarrURL, cmd.ID), nil)
		if err != nil {
			return false, "", err
		}
		status = cmd.Status
		if status == "" {
			status = "queued"
		}
	}

	result := cmd.Result
	if result == "" {
		result = "no-result"
	}
	detail = status + "/" + result
	if cmd.Message != "" {
		detail += " — " + cmd.Message
	}
	return status == "completed" && cmd.Result == "successful", detail, nil
}

// apiRequest performs one Lidarr call: POST when given a payload, else GET.
func apiRequest(ctx context.Context, opts Options, key, method, url string, payload any) (command, error) {
	var body *bytes.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return command{}, fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, body)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, http.NoBody)
	}
	if err != nil {
		return command{}, err
	}
	req.Header.Set("X-Api-Key", key)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := opts.Client.Do(req)
	if err != nil {
		return command{}, err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	var cmd command
	if err := json.NewDecoder(resp.Body).Decode(&cmd); err != nil {
		return command{}, fmt.Errorf("decode %s: %w", url, err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return cmd, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return cmd, nil
}
