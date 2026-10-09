// Package refresh tells Jellyfin that one library path changed.
//
// Radarr, Sonarr and Lidarr all authenticate their built-in Emby/Jellyfin
// connection with the X-Emby-Token header and the api_key query parameter.
// Jellyfin 12 removed both and accepts only
//
//	Authorization: MediaBrowser Token="<key>"
//
// so every library update those connections send comes back 401, the *arr logs
// "Unable to process notification queue" and the imported file stays invisible
// until the next scheduled scan — twelve hours away on a default schedule
// (Radarr#11658; fixed upstream in Radarr 6.4.2+, unreleased for Sonarr and
// Lidarr). This package posts the same update over the scheme Jellyfin 12
// accepts, from an *arr CustomScript hook.
//
// The update names a path rather than asking for a scan, so Jellyfin refreshes
// that one directory instead of walking the whole library, which is what makes
// this cheap enough to run on every import.
//
// An *arr hands its event to a hook through the environment and each one names
// those variables differently — radarr_movie_path, sonarr_series_path,
// lidarr_artist_path — so the names are configuration, not a table in here.
package refresh

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"homelab/app"
	"homelab/httpx"
	"homelab/secret"
)

// Jellyfin endpoints this tool uses.
const (
	// pathMediaUpdated refreshes the paths named in the request body.
	pathMediaUpdated = "/Library/Media/Updated"
	// pathSystemInfo is the cheapest authenticated route, used to answer a
	// Test event.
	pathSystemInfo = "/System/Info"
)

// TestEvent is the event type every *arr sends when the operator presses Test
// on a connection, and when configuratarr applies one. It carries no media
// path, so it is answered with a credential check instead of an update.
const TestEvent = "Test"

// updateTypeCreated is the only update type this tool sends. Jellyfin treats
// Created as "look at this path again", which is correct for an import, an
// upgrade and a rename alike: in all three the directory's contents differ from
// what Jellyfin last saw, and it is Jellyfin that decides what changed.
const updateTypeCreated = "Created"

// Config is what the tool takes on the command line.
type Config struct {
	// Base is the Jellyfin root URL.
	Base string `json:"base"`
	// APIKey is a Jellyfin API key given literally. Mutually exclusive
	// with APIKeyFile.
	APIKey string `json:"api_key"`
	// APIKeyFile is a file holding the API key.
	APIKeyFile string `json:"api_key_file"`
	// EventEnv names the environment variable holding the *arr event type.
	// Empty disables the Test-event check, which only costs the operator a
	// misleading Test result.
	EventEnv string `json:"event_env"`
	// PathEnv names the environment variables that may hold the changed
	// media path, comma-separated and in order of preference: an *arr sets
	// a different subset per event, so a hook is configured with every
	// variable that can carry the path and the first one present wins.
	//
	// One string rather than a list because modern Radarr refuses a custom
	// script with arguments ("Arguments are no longer supported for custom
	// scripts"), which leaves the environment as the only way to configure
	// a hook — and the environment layer carries scalars.
	PathEnv string `json:"path_env"`
}

// ErrNoBase reports a configuration with no Jellyfin URL.
var ErrNoBase = errors.New("jellyfin-refresh: base is required")

// ErrNoPathEnv reports a configuration that names no path variable, which
// could never refresh anything.
var ErrNoPathEnv = errors.New("jellyfin-refresh: at least one path-env is required")

// ErrNoPath reports an event whose path variables were all unset.
var ErrNoPath = errors.New("jellyfin-refresh: no media path in the environment")

// Defaults returns the configuration a hook that passes no flags gets.
//
// There is no default Base: Jellyfin runs in a different container from every
// *arr, so a loopback guess would only turn a missing flag into a connection
// refused at the far end of an import.
func Defaults() Config {
	return Config{}
}

// Validate rejects a configuration that could not do its job.
func (c Config) Validate() error {
	if c.Base == "" {
		return ErrNoBase
	}
	if len(c.PathEnvNames()) == 0 {
		return ErrNoPathEnv
	}
	return nil
}

// PathEnvNames splits PathEnv into the variable names to try, in order.
//
// Blank entries are dropped rather than tried: a trailing comma in a unit file
// would otherwise make the hook look up the empty variable name and report a
// missing path for an import that was fine.
func (c Config) PathEnvNames() []string {
	names := make([]string, 0, strings.Count(c.PathEnv, ",")+1)
	for _, name := range strings.Split(c.PathEnv, ",") {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}

// SelectPath returns the value of the first name that is set and non-empty.
//
// lookup is os.LookupEnv in production. An *arr exports its variables even for
// events they do not apply to, so a set-but-empty variable is ordinary and must
// not shadow a later one that carries the path.
func SelectPath(lookup func(string) (string, bool), names []string) string {
	for _, name := range names {
		if value, ok := lookup(name); ok && value != "" {
			return value
		}
	}
	return ""
}

// Directory reduces a media path to the directory Jellyfin should refresh.
//
// An *arr reports either the media folder or the imported file depending on the
// application and the event. Jellyfin refreshes what it is handed, so a file
// path would refresh only that file and miss the subtitles and artwork imported
// beside it. A path that does not exist is passed through unchanged: it is
// still the best description of what moved, and Jellyfin's own answer is more
// useful than a guess made here.
func Directory(path string) string {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

// update is one entry of a /Library/Media/Updated request.
type update struct {
	Path       string `json:"Path"`
	UpdateType string `json:"UpdateType"`
}

// mediaUpdated is the /Library/Media/Updated request body.
type mediaUpdated struct {
	Updates []update `json:"Updates"`
}

// Run resolves the *arr's event out of the environment and refreshes the path
// it names.
func Run(ctx context.Context, env *app.Env, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	key, err := secret.Flag("api-key", cfg.APIKey, cfg.APIKeyFile)
	if err != nil {
		return err
	}

	client, err := newClient(cfg.Base, key, env.Logger)
	if err != nil {
		return err
	}

	// The event type is read before the path because a Test event has none.
	if event, _ := os.LookupEnv(cfg.EventEnv); event == TestEvent {
		if err := client.Do(ctx, http.MethodGet, pathSystemInfo, nil, nil); err != nil {
			return fmt.Errorf("jellyfin-refresh: credential check failed: %w", err)
		}
		env.Logger.Info("credential accepted", slog.String("base", cfg.Base))
		return nil
	}

	names := cfg.PathEnvNames()
	path := SelectPath(os.LookupEnv, names)
	if path == "" {
		return fmt.Errorf("%w: tried %s", ErrNoPath, strings.Join(names, ", "))
	}
	dir := Directory(path)

	if env.DryRun {
		env.Logger.Info("dry run: not refreshing", slog.String("path", dir))
		return nil
	}

	body := mediaUpdated{Updates: []update{{Path: dir, UpdateType: updateTypeCreated}}}
	if err := client.Do(ctx, http.MethodPost, pathMediaUpdated, body, nil); err != nil {
		return fmt.Errorf("jellyfin-refresh: refresh %s: %w", dir, err)
	}

	env.Logger.Info("refreshed", slog.String("path", dir))
	return nil
}

// newClient builds the Jellyfin client with the credential in a default
// header, so the key is never part of a URL: Jellyfin logs request paths, and
// the api_key query parameter the *arr used would have written it there.
func newClient(base string, key secret.Value, logger *slog.Logger) (*httpx.Client, error) {
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("MediaBrowser Token=%q", key.Reveal()))

	return httpx.New(httpx.Config{
		BaseURL:   base,
		UserAgent: "jellyfin-refresh",
		Headers:   headers,
		Logger:    logger,
	})
}
