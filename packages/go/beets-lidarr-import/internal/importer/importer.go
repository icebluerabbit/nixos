// Package importer hands one finished album from beets to Lidarr.
//
// beets calls this once it has written an album into its output directory.
// Lidarr does the rest: it links the files into the artist folder, renames
// them to the configured format and records the extra files alongside. That
// is what keeps a CD rip and a torrent landing identically, with Lidarr as
// the only writer into the library.
//
// It asks Lidarr what it found and then tells it to import that, rather than
// asking it to scan a folder and decide for itself.
//
// The difference matters. An unattended DownloadedAlbumsScan scores the files
// against the releases Lidarr holds and refuses anything under 80%, and the
// release MBID carries weight 5.0 of roughly 23 — ten times any other field
// (CloseAlbumMatchSpecification, DistanceCalculator). A rip tagged with a
// real MusicBrainz release that Lidarr's metadata mirror happens not to hold
// therefore fails every time, however correct the rip is. The ManualImport
// command builds its decisions directly and never runs that specification, so
// identification still comes from Lidarr — it is acted on rather than
// second-guessed.
package importer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"homelab/app"
	"homelab/clients/arr"
	"homelab/httpx"
)

// Environment names this reads. They are Lidarr's own, not this tool's, and
// the beets container sets them on the unit the hook inherits — so they stay
// as they are rather than moving under a HOMELAB_ prefix.
const (
	EnvURL     = "LIDARR_URL"
	EnvAPIKey  = "LIDARR_API_KEY" //nolint:gosec // the variable's name, not a key
	EnvMode    = "LIDARR_IMPORT_MODE"
	EnvTimeout = "LIDARR_IMPORT_TIMEOUT"
	EnvPoll    = "LIDARR_IMPORT_POLL"
)

// Defaults for the settings the container does not set.
const (
	// DefaultMode leaves the rip where beets put it and, with Lidarr's
	// copyUsingHardlinks enabled on the same filesystem, links rather than
	// duplicates it — so the rip survives as its own tree at no extra disk
	// cost. That mirrors how a torrent stays in the download directory
	// while Lidarr links it into the library.
	DefaultMode    = arr.ImportCopy
	DefaultTimeout = 300 * time.Second
	DefaultPoll    = 2 * time.Second
)

// callTimeout bounds one Lidarr call. The identification walks a folder and
// is the slow one.
const callTimeout = 30 * time.Second

// ErrNoAlbum reports an invocation with no album directory.
var ErrNoAlbum = errors.New("--album is required")

// Config is what the tool takes on the command line.
type Config struct {
	// Album is the directory beets has just written.
	Album string `json:"album"`
}

// Defaults returns the configuration before flags and the environment.
func Defaults() Config { return Config{} }

// Settings are the parts that arrive through the environment.
type Settings struct {
	URL     string
	APIKey  string
	Mode    string
	Timeout time.Duration
	Poll    time.Duration
}

// LoadSettings reads the environment, failing on anything missing or
// unreadable rather than importing against a half-configured Lidarr.
func LoadSettings() (Settings, error) {
	url, err := required(EnvURL)
	if err != nil {
		return Settings{}, err
	}
	key, err := required(EnvAPIKey)
	if err != nil {
		return Settings{}, err
	}
	timeout, err := seconds(EnvTimeout, DefaultTimeout)
	if err != nil {
		return Settings{}, err
	}
	poll, err := seconds(EnvPoll, DefaultPoll)
	if err != nil {
		return Settings{}, err
	}

	mode := os.Getenv(EnvMode)
	if mode == "" {
		mode = DefaultMode
	}

	return Settings{
		// Every trailing slash goes: the API path is appended to this,
		// and "http://lidarr:8686///api/v1" is a 404 that reads like a
		// wrong endpoint.
		URL:     strings.TrimRight(url, "/"),
		APIKey:  key,
		Mode:    mode,
		Timeout: timeout,
		Poll:    poll,
	}, nil
}

func required(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is not set", key)
	}
	return value, nil
}

// seconds reads a duration given as a plain number of seconds, which is what
// the environment has always carried here.
func seconds(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is not a number: %q", key, raw)
	}
	return time.Duration(value * float64(time.Second)), nil
}

// Run imports one album.
func Run(ctx context.Context, env *app.Env, cfg Config) error {
	if cfg.Album == "" {
		return ErrNoAlbum
	}
	settings, err := LoadSettings()
	if err != nil {
		return err
	}

	client, err := arr.New(arr.Lidarr, httpx.Config{
		BaseURL: settings.URL,
		Timeout: callTimeout,
		// No retries. The poll loop is the retry: a blip during it is
		// one missed read out of many, and a blip before it means the
		// album stays in the output directory for the next pass.
		RetryMax: -1,
		Logger:   env.Logger,
	}, settings.APIKey)
	if err != nil {
		return err
	}

	found, err := client.ManualImport(ctx, cfg.Album)
	if err != nil {
		return fmt.Errorf("could not reach Lidarr at %s: %w", settings.URL, err)
	}
	if len(found) == 0 {
		//nolint:staticcheck // Lidarr is a proper noun, not a sentence opener
		return fmt.Errorf("Lidarr found no importable files in %s", cfg.Album)
	}

	// Every file has to be placed. A file Lidarr could not resolve has no
	// track to import it as, and importing the rest would leave the album
	// silently short. Left where beets put it rather than imported
	// somewhere wrong: an album Lidarr does not track is a decision for a
	// person.
	if unresolved := unplaced(found); len(unresolved) > 0 {
		return unplacedError(cfg.Album, unresolved, len(found))
	}

	if env.DryRun {
		env.Logger.Info("would import",
			slog.String("album", cfg.Album),
			slog.Int("files", len(found)),
			slog.String("mode", settings.Mode))
		return nil
	}

	queued, err := client.RunCommand(ctx, arr.NewManualImport(found, settings.Mode))
	if err != nil {
		//nolint:staticcheck // Lidarr is a proper noun, not a sentence opener
		return fmt.Errorf("Lidarr refused the import of %s: %w", cfg.Album, err)
	}
	if queued.ID == 0 {
		//nolint:staticcheck // Lidarr is a proper noun, not a sentence opener
		return fmt.Errorf("Lidarr rejected the request for %s", cfg.Album)
	}

	env.Logger.Info("queued import",
		slog.Int64("command", queued.ID),
		slog.Int("files", len(found)),
		slog.String("album", cfg.Album))

	return await(ctx, env, client, cfg.Album, queued.ID, settings)
}

// unplaced returns the files Lidarr could not fully resolve.
func unplaced(items []arr.ManualImportItem) []arr.ManualImportItem {
	var out []arr.ManualImportItem
	for _, item := range items {
		if !item.Placed() {
			out = append(out, item)
		}
	}
	return out
}

// unplacedError explains which files Lidarr could not place, and why.
func unplacedError(album string, items []arr.ManualImportItem, total int) error {
	lines := []string{fmt.Sprintf(
		"Lidarr could not place %d of %d files in %s; not imported",
		len(items), total, album)}
	for _, item := range items {
		name := filepath.Base(item.Path)
		if item.Path == "" {
			name = "?"
		}
		lines = append(lines, fmt.Sprintf("  %s: %s", name,
			strings.Join(item.Reasons(), "; ")))
	}
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}

// await waits for the import to finish, and says what it did.
//
// The command is asynchronous and completes successfully even when nothing
// was linked — that shows up as an unsuccessful result rather than as a
// failure. Polling for it is the difference between knowing an album was
// imported and assuming it was.
func await(
	ctx context.Context,
	env *app.Env,
	client *arr.Client,
	album string,
	command int64,
	settings Settings,
) error {
	deadline, cancel := context.WithTimeout(ctx, settings.Timeout)
	defer cancel()

	ticker := time.NewTicker(settings.Poll)
	defer ticker.Stop()

	for {
		status, err := client.CommandStatus(deadline, command)
		if err != nil {
			if deadline.Err() != nil {
				return fmt.Errorf("gave up waiting %s for command %d (%s)",
					settings.Timeout, command, album)
			}
			return fmt.Errorf("lost contact with Lidarr while awaiting %d: %w",
				command, err)
		}

		switch {
		case status.Status == arr.CommandCompleted &&
			status.Result == arr.CommandSuccessful:
			env.Logger.Info("imported", slog.String("album", album))
			return nil

		case status.Status == arr.CommandCompleted:
			// Nothing was linked, so the album stays visible in the
			// output directory as one Lidarr could not place.
			//nolint:staticcheck // Lidarr is a proper noun, not a sentence opener
			return fmt.Errorf("Lidarr imported nothing from %s", album)

		case status.Finished():
			return fmt.Errorf("import of %s %s: %s",
				album, status.Status, status.Detail())
		}

		select {
		case <-ticker.C:
		case <-deadline.Done():
			return fmt.Errorf("gave up waiting %s for command %d (%s)",
				settings.Timeout, command, album)
		}
	}
}
