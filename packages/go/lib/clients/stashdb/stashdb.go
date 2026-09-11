// Package stashdb queries StashDB, the community metadata database that this
// homelab's Stash instance already identifies its library against.
//
// StashDB is a stash-box GraphQL server. Two properties shape this client:
//
//   - Authentication is an `ApiKey` header carrying the key verbatim. It is
//     not `Authorization: Bearer` — that is rejected.
//   - A GraphQL server answers 200 for a failed query and reports the failure
//     in an `errors` array. httpx therefore sees success on a query that did
//     nothing, so every response is checked for `errors` here. Getting this
//     wrong yields empty results that look like "no match" rather than a
//     failure.
//
// Search is fuzzy full-text and low precision: `searchScenes("Tushy")` returns
// over a thousand hits, led by an unrelated title that merely contains the
// word. A caller deciding whether a release is a given scene needs to
// corroborate with studio, performers and date rather than trust a title hit.
//
// https://stashdb.org/graphql
package stashdb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"homelab/httpx"
	"homelab/secret"
)

// DefaultEndpoint is the public StashDB GraphQL endpoint.
const DefaultEndpoint = "https://stashdb.org/graphql"

// DefaultLimit bounds a search when the caller does not.
const DefaultLimit = 10

// Config describes a StashDB client.
type Config struct {
	// Endpoint is the GraphQL URL. Empty means DefaultEndpoint.
	Endpoint string
	// APIKey authenticates every request. Required.
	APIKey secret.Value
	// Timeout bounds a single attempt. Zero means httpx's default.
	Timeout time.Duration
	// Logger receives retry diagnostics. Nil discards them.
	Logger *slog.Logger
}

// Client is a StashDB API client.
type Client struct {
	http *httpx.Client
}

// ErrNoAPIKey reports a Config with no key.
var ErrNoAPIKey = errors.New("stashdb: APIKey is required")

// ErrUnauthorized reports a key StashDB refused. It is separated from other
// failures because it is a deployment fault, not a transient one — retrying
// will never fix it.
var ErrUnauthorized = errors.New("stashdb: not authorized")

// New builds a client for cfg.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey.IsZero() {
		return nil, ErrNoAPIKey
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	c, err := httpx.New(httpx.Config{
		BaseURL:   endpoint,
		Timeout:   cfg.Timeout,
		UserAgent: "homelab-stashdb",
		// Revealed once, here. The header set is not exposed again, so the
		// key does not travel through any other call in this package.
		Headers: http.Header{"Apikey": []string{cfg.APIKey.Reveal()}},
		Logger:  cfg.Logger,
	})
	if err != nil {
		return nil, err
	}
	return &Client{http: c}, nil
}

// Performer is a performer credited on a scene.
type Performer struct {
	ID string `json:"id"`
	// Name is the performer's primary name on StashDB.
	Name string `json:"name"`
	// Disambiguation separates performers who share a name.
	Disambiguation string `json:"disambiguation"`
}

// Scene is one StashDB scene, flattened from the GraphQL shape.
type Scene struct {
	ID string
	// Title as StashDB records it, which is often not the release title.
	Title string
	// Code is the studio's own scene identifier where one is known. It is
	// the highest-signal join to a release name when present, and frequently
	// absent — StashDB returns null for most older scenes.
	Code string
	// Studio name, empty when the scene has none recorded.
	Studio string
	// ReleaseDate in YYYY-MM-DD, as StashDB stores it. Kept as a string
	// because StashDB records partial dates and a time.Time cannot.
	ReleaseDate string
	// Duration in seconds, zero when unknown. Useful corroboration against
	// a torrent's file table.
	Duration int
	// Performers credited on the scene.
	Performers []Performer
	// URLs are the scene's links, studio page first where one exists.
	URLs []string
}

// searchScenesQuery is kept whole rather than built by concatenation so it can
// be pasted into StashDB's GraphQL playground unchanged when a field is added.
const searchScenesQuery = `query SearchScenes($term: String!, $limit: Int!) {
  searchScenes(term: $term, limit: $limit) {
    count
    scenes {
      id
      title
      code
      release_date
      duration
      studio { name }
      performers { performer { id name disambiguation } }
      urls { url }
    }
  }
}`

const findSceneQuery = `query FindScene($id: ID!) {
  findScene(id: $id) {
    id
    title
    code
    release_date
    duration
    studio { name }
    performers { performer { id name disambiguation } }
    urls { url }
  }
}`

const searchPerformersQuery = `query SearchPerformers($term: String!, $limit: Int!) {
  searchPerformers(term: $term, limit: $limit) {
    count
    performers { id name disambiguation }
  }
}`

// SearchScenes runs StashDB's full-text scene search.
//
// It returns the matches and the server's total count, which is usually far
// larger than the page returned. limit of zero means DefaultLimit.
//
// The search is fuzzy: treat a result as a candidate, not an identification.
func (c *Client) SearchScenes(ctx context.Context, term string, limit int) ([]Scene, int, error) {
	if strings.TrimSpace(term) == "" {
		return nil, 0, errors.New("stashdb: search term is required")
	}
	if limit <= 0 {
		limit = DefaultLimit
	}

	var out struct {
		SearchScenes struct {
			Count  int         `json:"count"`
			Scenes []wireScene `json:"scenes"`
		} `json:"searchScenes"`
	}
	vars := map[string]any{"term": term, "limit": limit}
	if err := c.query(ctx, searchScenesQuery, vars, &out); err != nil {
		return nil, 0, err
	}

	scenes := make([]Scene, 0, len(out.SearchScenes.Scenes))
	for _, s := range out.SearchScenes.Scenes {
		scenes = append(scenes, s.scene())
	}
	return scenes, out.SearchScenes.Count, nil
}

// SearchPerformers runs StashDB's full-text performer search.
func (c *Client) SearchPerformers(ctx context.Context, term string, limit int) ([]Performer, error) {
	if strings.TrimSpace(term) == "" {
		return nil, errors.New("stashdb: search term is required")
	}
	if limit <= 0 {
		limit = DefaultLimit
	}

	var out struct {
		SearchPerformers struct {
			Performers []Performer `json:"performers"`
		} `json:"searchPerformers"`
	}
	vars := map[string]any{"term": term, "limit": limit}
	if err := c.query(ctx, searchPerformersQuery, vars, &out); err != nil {
		return nil, err
	}
	return out.SearchPerformers.Performers, nil
}

// ErrNotFound reports a scene id StashDB does not know.
var ErrNotFound = errors.New("stashdb: scene not found")

// Scene fetches one scene by id.
func (c *Client) Scene(ctx context.Context, id string) (*Scene, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("stashdb: scene id is required")
	}

	var out struct {
		FindScene *wireScene `json:"findScene"`
	}
	if err := c.query(ctx, findSceneQuery, map[string]any{"id": id}, &out); err != nil {
		return nil, err
	}
	if out.FindScene == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	scene := out.FindScene.scene()
	return &scene, nil
}

// wireScene mirrors StashDB's nested GraphQL shape. Scene is the flat form
// callers actually want; keeping them separate means the schema's nesting does
// not leak into every consumer.
type wireScene struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Code        string `json:"code"`
	ReleaseDate string `json:"release_date"`
	Duration    int    `json:"duration"`
	Studio      *struct {
		Name string `json:"name"`
	} `json:"studio"`
	Performers []struct {
		Performer Performer `json:"performer"`
	} `json:"performers"`
	URLs []struct {
		URL string `json:"url"`
	} `json:"urls"`
}

func (w wireScene) scene() Scene {
	s := Scene{
		ID:          w.ID,
		Title:       w.Title,
		Code:        w.Code,
		ReleaseDate: w.ReleaseDate,
		Duration:    w.Duration,
	}
	if w.Studio != nil {
		s.Studio = w.Studio.Name
	}
	for _, p := range w.Performers {
		s.Performers = append(s.Performers, p.Performer)
	}
	for _, u := range w.URLs {
		s.URLs = append(s.URLs, u.URL)
	}
	return s
}

// graphQLError is one entry of a GraphQL response's errors array.
type graphQLError struct {
	Message string   `json:"message"`
	Path    []string `json:"path"`
}

// query posts a GraphQL operation and decodes data into out.
//
// Variables rather than string interpolation: a release title carrying a quote
// or a backslash must not be able to alter the query.
func (c *Client) query(ctx context.Context, operation string, vars map[string]any, out any) error {
	body := map[string]any{"query": operation, "variables": vars}

	var envelope struct {
		Data   any            `json:"data"`
		Errors []graphQLError `json:"errors"`
	}
	envelope.Data = out

	if err := c.http.Do(ctx, http.MethodPost, "", body, &envelope); err != nil {
		return fmt.Errorf("stashdb: %w", err)
	}
	return graphQLFailure(envelope.Errors)
}

// graphQLFailure turns an errors array into one error, mapping the
// authorization case to a sentinel so a bad key is distinguishable from a
// transient failure.
func graphQLFailure(errs []graphQLError) error {
	if len(errs) == 0 {
		return nil
	}

	messages := make([]string, 0, len(errs))
	unauthorized := false
	for _, e := range errs {
		messages = append(messages, e.Message)
		if strings.Contains(strings.ToLower(e.Message), "not authorized") {
			unauthorized = true
		}
	}
	if unauthorized {
		return ErrUnauthorized
	}
	return fmt.Errorf("stashdb: %s", strings.Join(messages, "; "))
}
