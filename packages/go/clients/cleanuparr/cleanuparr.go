// Package cleanuparr talks to a Cleanuparr instance's HTTP API.
//
// Cleanuparr mints its API key with the admin account during first-run setup
// and only serves it from the API — there is no config file or DB row to
// preseed, and every non-auth /api/* path answers 403 "Setup required" until
// setup completes. That shape is why this client exposes the setup dance as
// distinct steps rather than one call.
//
// https://github.com/Cleanuparr/Cleanuparr
package cleanuparr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"homelab/httpx"
	"homelab/secret"
)

// Paths this client uses. /health and /api/auth/* are unauthenticated;
// everything else needs the bearer token from Login.
const (
	pathHealth        = "/health"
	pathAuthStatus    = "/api/auth/status"
	pathSetupAccount  = "/api/auth/setup/account"
	pathSetupComplete = "/api/auth/setup/complete"
	pathLogin         = "/api/auth/login"
	pathAPIKey        = "/api/account/api-key"
)

// Client is a Cleanuparr API client.
type Client struct {
	http *httpx.Client
}

// Credentials are the admin account's.
//
// They are carried in a struct rather than as loose arguments so a caller
// cannot transpose them. Password is a secret.Value, so it cannot reach a log
// record or an error however this struct is handled — but note that the struct
// is also the JSON login payload, which is why it marshals through
// credentialsJSON below rather than directly.
type Credentials struct {
	Username string
	Password secret.Value
}

// credentialsJSON is the wire shape. secret.Value marshals to [REDACTED] by
// design, so sending Credentials to the API needs an explicit Reveal — the
// same deliberate step every other exposure requires.
type credentialsJSON struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// MarshalJSON reveals the password for the request body, and only there.
func (c Credentials) MarshalJSON() ([]byte, error) {
	return json.Marshal(credentialsJSON{
		Username: c.Username,
		Password: c.Password.Reveal(),
	})
}

// ErrNoCredentials reports empty admin credentials.
var ErrNoCredentials = errors.New("cleanuparr: username and password are required")

// ErrNoToken reports a login that returned no access token.
var ErrNoToken = errors.New("cleanuparr: login returned no access token")

// ErrNoAPIKey reports an authenticated key fetch that returned nothing.
var ErrNoAPIKey = errors.New("cleanuparr: no API key in response")

// Valid reports whether both fields are set.
func (c Credentials) Valid() bool {
	return c.Username != "" && !c.Password.IsZero()
}

// New builds a client for the instance at baseURL.
func New(cfg httpx.Config) (*Client, error) {
	c, err := httpx.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{http: c}, nil
}

// WaitReady blocks until the instance answers its health check, or ctx ends.
func (c *Client) WaitReady(ctx context.Context, interval time.Duration) error {
	return c.http.WaitReady(ctx, pathHealth, interval)
}

// SetupCompleted reports whether first-run setup has already been done.
//
// The endpoint is unauthenticated and the account lives in PostgreSQL rather
// than the container's ephemeral rootfs, so this is what makes a re-run after a
// reboot harmless.
func (c *Client) SetupCompleted(ctx context.Context) (bool, error) {
	var status struct {
		SetupCompleted bool `json:"setupCompleted"`
	}
	if err := c.http.Do(ctx, http.MethodGet, pathAuthStatus, nil, &status); err != nil {
		return false, fmt.Errorf("cleanuparr: auth status: %w", err)
	}
	return status.SetupCompleted, nil
}

// CompleteSetup creates the admin account and finishes first-run setup.
func (c *Client) CompleteSetup(ctx context.Context, creds Credentials) error {
	if !creds.Valid() {
		return ErrNoCredentials
	}
	if err := c.http.Do(ctx, http.MethodPost, pathSetupAccount, creds, nil); err != nil {
		return fmt.Errorf("cleanuparr: create admin account: %w", err)
	}
	if err := c.http.Do(ctx, http.MethodPost, pathSetupComplete, nil, nil); err != nil {
		return fmt.Errorf("cleanuparr: complete setup: %w", err)
	}
	return nil
}

// Login exchanges the admin credentials for an access token.
func (c *Client) Login(ctx context.Context, creds Credentials) (secret.Value, error) {
	if !creds.Valid() {
		return secret.Value{}, ErrNoCredentials
	}

	var out struct {
		Tokens struct {
			AccessToken string `json:"accessToken"`
		} `json:"tokens"`
	}
	if err := c.http.Do(ctx, http.MethodPost, pathLogin, creds, &out); err != nil {
		return secret.Value{}, fmt.Errorf("cleanuparr: login: %w", err)
	}
	if out.Tokens.AccessToken == "" {
		return secret.Value{}, ErrNoToken
	}
	return secret.New(out.Tokens.AccessToken), nil
}

// APIKey fetches the admin account's API key using a token from Login.
func (c *Client) APIKey(ctx context.Context, token secret.Value) (secret.Value, error) {
	var out struct {
		APIKey string `json:"apiKey"`
	}
	authed := c.http.WithBearer(token.Reveal())
	if err := authed.Do(ctx, http.MethodGet, pathAPIKey, nil, &out); err != nil {
		return secret.Value{}, fmt.Errorf("cleanuparr: fetch api key: %w", err)
	}
	if out.APIKey == "" {
		return secret.Value{}, ErrNoAPIKey
	}
	return secret.New(out.APIKey), nil
}
