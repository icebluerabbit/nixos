// Package stashbox is a client for stash-box, the GraphQL metadata service
// behind StashDB and FansDB.
//
// Both instances run the same schema, so the endpoint and API key are
// configuration rather than separate clients: a caller that follows performers
// on both constructs two of these.
//
// Queries are struct-tagged rather than generated. The schema is large and this
// package uses a corner of it — favourites and the scenes attached to them —
// so a code generator would cost a vendored schema copy and a build step to
// type four queries. The trade is that a schema change surfaces at runtime, so
// every query has a fixture test in testdata/ that fails when the shape moves.
package stashbox

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hasura/go-graphql-client"

	"homelab/secret"
)

// Config describes one stash-box instance.
type Config struct {
	// Endpoint is the GraphQL URL, e.g. https://stashdb.org/graphql.
	Endpoint string
	// APIKey authenticates as the account whose favourites are read. It is
	// sent in the ApiKey header, which is stash-box's own scheme — not a
	// bearer token.
	APIKey secret.Value
	// UserAgent identifies this client to the service.
	UserAgent string
	// Timeout bounds a single request. A favourites page is small; a scene
	// page with fingerprints is not.
	Timeout time.Duration
	// MinInterval is the floor between requests. stash-box publishes a
	// per-minute limit and Stash's own config carries a
	// maxrequestsperminute field, so a bulk crawl paces itself rather than
	// discovering the limit by being throttled.
	MinInterval time.Duration
}

// Defaults for the fields a caller does not set.
const (
	defaultTimeout     = 30 * time.Second
	defaultMinInterval = time.Second
	defaultUserAgent   = "homelab-stashbox/1.0"
	// DefaultPerPage is the page size for every paginated query. stash-box
	// caps a page at 100.
	DefaultPerPage = 100
)

// Client talks to one stash-box instance.
type Client struct {
	gql      *graphql.Client
	limiter  <-chan time.Time
	stopTick func()
}

// header injects the ApiKey header on every request.
type header struct {
	base http.RoundTripper
	key  string
	ua   string
}

func (h header) RoundTrip(r *http.Request) (*http.Response, error) {
	// Clone: RoundTrip must not modify the caller's request.
	r = r.Clone(r.Context())
	r.Header.Set("ApiKey", h.key)
	r.Header.Set("User-Agent", h.ua)
	return h.base.RoundTrip(r)
}

// New builds a client for cfg.
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("stashbox: endpoint is required")
	}
	if cfg.APIKey.IsZero() {
		return nil, fmt.Errorf("stashbox: api key is required")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.MinInterval == 0 {
		cfg.MinInterval = defaultMinInterval
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}

	httpc := &http.Client{
		Timeout: cfg.Timeout,
		Transport: header{
			base: http.DefaultTransport,
			key:  cfg.APIKey.Reveal(),
			ua:   cfg.UserAgent,
		},
	}

	tick := time.NewTicker(cfg.MinInterval)
	return &Client{
		gql:      graphql.NewClient(cfg.Endpoint, httpc),
		limiter:  tick.C,
		stopTick: tick.Stop,
	}, nil
}

// Close releases the rate limiter.
func (c *Client) Close() { c.stopTick() }

// wait blocks until the rate limiter allows another request, or ctx ends.
func (c *Client) wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.limiter:
		return nil
	}
}

// URL is a link with the site that hosts it.
type URL struct {
	URL  string `graphql:"url"`
	Site struct {
		ID   string `graphql:"id"`
		Name string `graphql:"name"`
	} `graphql:"site"`
}

// Performer is the subset of a stash-box performer this homelab stores.
type Performer struct {
	ID             string   `graphql:"id"`
	Name           string   `graphql:"name"`
	Disambiguation string   `graphql:"disambiguation"`
	Aliases        []string `graphql:"aliases"`
	Gender         string   `graphql:"gender"`
	URLs           []URL    `graphql:"urls"`
	Deleted        bool     `graphql:"deleted"`
	MergedIntoID   string   `graphql:"merged_into_id"`
	IsFavorite     bool     `graphql:"is_favorite"`
	Updated        string   `graphql:"updated"`
}

// Studio is the subset of a stash-box studio this homelab stores.
//
// Parent is the network a site belongs to. Release names carry the site, so
// the hierarchy is what connects "Tushy" to the group that owns it.
type Studio struct {
	ID      string   `graphql:"id"`
	Name    string   `graphql:"name"`
	Aliases []string `graphql:"aliases"`
	URLs    []URL    `graphql:"urls"`
	Parent  *struct {
		ID   string `graphql:"id"`
		Name string `graphql:"name"`
	} `graphql:"parent"`
	Deleted    bool   `graphql:"deleted"`
	IsFavorite bool   `graphql:"is_favorite"`
	Updated    string `graphql:"updated"`
}

// Fingerprint is a hash submitted for a scene's file. oshash is the one worth
// computing locally: it is size plus the first and last 64 KiB, no decode.
type Fingerprint struct {
	Hash        string `graphql:"hash"`
	Algorithm   string `graphql:"algorithm"`
	Duration    int    `graphql:"duration"`
	Submissions int    `graphql:"submissions"`
}

// Appearance is a performer in a scene, with the alias that scene credited
// them under. That alias is what a release name is most likely to contain.
type Appearance struct {
	Performer struct {
		ID   string `graphql:"id"`
		Name string `graphql:"name"`
	} `graphql:"performer"`
	As string `graphql:"as"`
}

// Scene is the subset of a stash-box scene this homelab stores.
type Scene struct {
	ID          string `graphql:"id"`
	Title       string `graphql:"title"`
	ReleaseDate string `graphql:"release_date"`
	Code        string `graphql:"code"`
	Duration    int    `graphql:"duration"`
	URLs        []URL  `graphql:"urls"`
	Studio      *struct {
		ID   string `graphql:"id"`
		Name string `graphql:"name"`
	} `graphql:"studio"`
	Performers   []Appearance  `graphql:"performers"`
	Fingerprints []Fingerprint `graphql:"fingerprints"`
	Tags         []struct {
		ID   string `graphql:"id"`
		Name string `graphql:"name"`
	} `graphql:"tags"`
	Deleted bool   `graphql:"deleted"`
	Updated string `graphql:"updated"`
}
