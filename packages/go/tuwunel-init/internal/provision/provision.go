// Package provision declaratively creates tuwunel's users, rooms and spaces.
//
// Matrix has no declarative configuration for any of this: users are created
// through the Synapse-compatible shared-secret registration API, and rooms
// get their IDs minted server-side at createRoom. So the declaration lives in
// Nix and this reconciles it after the homeserver comes up.
//
// Everything here is idempotent, because it runs on every container start. A
// user that already exists answers "User ID is not available", an alias that
// already exists answers in kind, and both are skipped rather than treated as
// failures.
package provision

import (
	"bytes"
	"context"
	"crypto/hmac"
	//nolint:gosec // G505: SHA-1 is what the shared-secret registration API specifies
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultReadyDelay is how long to wait between checks for the homeserver.
const DefaultReadyDelay = 2 * time.Second

// avatarContentType is what the uploader declares. Every declared avatar in
// this repo is a PNG, which is the assumption the original made too.
const avatarContentType = "image/png"

// userInUse is the fragment tuwunel's registration answers with when the
// user already exists. Matched case-insensitively on the error body, which
// is what makes a re-run a no-op rather than a failure.
const userInUse = "available"

// User is one declared account.
type User struct {
	Username   string `json:"username"`
	Admin      bool   `json:"admin"`
	AvatarFile string `json:"avatarFile"`
}

// Room is one declared room or space.
type Room struct {
	Name     string   `json:"name"`
	Alias    string   `json:"alias"`
	Space    bool     `json:"space"`
	Parent   string   `json:"parent"`
	IconFile string   `json:"iconFile"`
	Members  []string `json:"members"`
}

// Config is what the tool takes on the command line.
type Config struct {
	Port       string
	ServerName string
	AdminUser  string
	Users      []User
	Rooms      []Room
}

// Secrets are the credentials, which arrive by environment rather than argv
// so they never show up in the homeserver's process list.
type Secrets struct {
	// RegSecret is the shared registration secret.
	RegSecret string
	// PassFor returns a user's password, or "" when none is configured.
	PassFor func(username string) string
}

// EnvSecrets reads the credentials from the environment the unit sets.
func EnvSecrets() Secrets {
	return Secrets{
		RegSecret: os.Getenv("TUWUNEL_REG_SECRET"),
		PassFor:   envPassword,
	}
}

// envPassword maps a username onto the variable holding its password.
func envPassword(username string) string {
	key := "TUWUNEL_" + strings.ToUpper(strings.ReplaceAll(username, "-", "_")) + "_PASS"
	return os.Getenv(key)
}

// Options tune the readiness wait and the HTTP client.
type Options struct {
	ReadyDelay time.Duration
	Client     *http.Client
}

// withDefaults fills in whatever the caller left unset.
func (o Options) withDefaults() Options {
	if o.ReadyDelay <= 0 {
		o.ReadyDelay = DefaultReadyDelay
	}
	if o.Client == nil {
		o.Client = http.DefaultClient
	}
	return o
}

// client talks to one homeserver.
type client struct {
	base    string
	api     string
	synapse string
	http    *http.Client
	stderr  io.Writer
}

// result is one API answer. An HTTP error is data here rather than a
// failure: idempotent re-runs hit user-in-use, alias-in-use and
// already-joined, and all three are expected.
type result struct {
	Body   map[string]any
	Status int
	// ErrBody is the raw body of a non-2xx response.
	ErrBody string
}

// failed reports whether the call did not succeed.
func (r result) failed() bool { return r.Status >= http.StatusBadRequest }

// str reads a string field out of a successful answer.
func (r result) str(key string) string {
	v, _ := r.Body[key].(string)
	return v
}

// Run provisions everything the declaration names.
func Run(ctx context.Context, stderr io.Writer, cfg Config, secrets Secrets, opts Options) error {
	opts = opts.withDefaults()

	base := "http://localhost:" + cfg.Port
	c := &client{
		base:    base,
		api:     base + "/_matrix/client/v3",
		synapse: base + "/_synapse/admin/v1",
		http:    opts.Client,
		stderr:  stderr,
	}

	if err := c.waitReady(ctx, opts.ReadyDelay); err != nil {
		return err
	}

	// Users first: the room pass logs in as them.
	for _, u := range cfg.Users {
		c.register(ctx, u, secrets)
	}
	for _, u := range cfg.Users {
		c.setUserAvatar(ctx, cfg, u, secrets)
	}

	adminToken := c.login(ctx, cfg.AdminUser, secrets)

	// Pass 1: every room exists and has its members.
	for _, room := range cfg.Rooms {
		c.createRoom(ctx, cfg, room, adminToken, secrets)
	}
	// Pass 2: the hierarchy, once every room has an ID to point at.
	for _, room := range cfg.Rooms {
		c.linkSpace(ctx, cfg, room, adminToken)
	}
	return nil
}

// waitReady blocks until the homeserver answers, because the unit starts
// alongside it rather than after it.
func (c *client) waitReady(ctx context.Context, delay time.Duration) error {
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			c.base+"/_matrix/client/versions", http.NoBody)
		if err != nil {
			return err
		}
		resp, err := c.http.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

// req performs one JSON call.
func (c *client) req(ctx context.Context, method, url, token string, body any) result {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return result{Status: http.StatusInternalServerError, ErrBody: err.Error()}
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return result{Status: http.StatusInternalServerError, ErrBody: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return result{Status: http.StatusServiceUnavailable, ErrBody: err.Error()}
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return result{Status: resp.StatusCode, ErrBody: err.Error()}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return result{Status: resp.StatusCode, ErrBody: string(raw)}
	}

	out := result{Status: resp.StatusCode, Body: map[string]any{}}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.Body)
	}
	return out
}

// RegisterMAC is the HMAC the shared-secret registration API expects:
// SHA-1 over nonce, username and password, each followed by a NUL, then the
// admin flag with no trailing NUL.
//
// Exported because the byte layout is the whole protocol — get it wrong and
// the homeserver rejects every registration with no clue why.
func RegisterMAC(secret, nonce, username, password string, admin bool) string {
	//nolint:gosec // G401: SHA-1 is what the registration API specifies
	mac := hmac.New(sha1.New, []byte(secret))
	for _, part := range []string{nonce, username, password} {
		_, _ = mac.Write([]byte(part))
		_, _ = mac.Write([]byte{0})
	}
	if admin {
		_, _ = mac.Write([]byte("admin"))
	} else {
		_, _ = mac.Write([]byte("notadmin"))
	}
	return hex.EncodeToString(mac.Sum(nil))
}

// register creates one account, skipping one that already exists.
func (c *client) register(ctx context.Context, u User, secrets Secrets) {
	password := secrets.PassFor(u.Username)
	if password == "" {
		fmt.Fprintf(c.stderr, "init: no password for %s, skipping\n", u.Username) //nolint:errcheck // reporting only
		return
	}

	nonce := c.req(ctx, http.MethodGet, c.synapse+"/register", "", nil).str("nonce")
	if nonce == "" {
		fmt.Fprintf(c.stderr, "init: no register nonce, skipping %s\n", u.Username) //nolint:errcheck // reporting only
		return
	}

	res := c.req(ctx, http.MethodPost, c.synapse+"/register", "", map[string]any{
		"nonce":         nonce,
		"username":      u.Username,
		"password":      password,
		"admin":         u.Admin,
		"mac":           RegisterMAC(secrets.RegSecret, nonce, u.Username, password, u.Admin),
		"inhibit_login": true,
	})

	// "User ID is not available" means the account is already there, which
	// is the normal answer on every run after the first.
	if res.failed() && !strings.Contains(strings.ToLower(res.ErrBody), userInUse) {
		fmt.Fprintf(c.stderr, "init: register %s -> %d %s\n", u.Username, res.Status, res.ErrBody) //nolint:errcheck // reporting only
	}
}

// login returns an access token for a declared user, or "".
func (c *client) login(ctx context.Context, username string, secrets Secrets) string {
	return c.req(ctx, http.MethodPost, c.api+"/login", "", map[string]any{
		"type":       "m.login.password",
		"identifier": map[string]any{"type": "m.id.user", "user": username},
		"password":   secrets.PassFor(username),
	}).str("access_token")
}

// setUserAvatar gives a user its declared picture, leaving an existing one
// alone so a person who changed theirs keeps it.
func (c *client) setUserAvatar(ctx context.Context, cfg Config, u User, secrets Secrets) {
	if u.AvatarFile == "" {
		return
	}
	token := c.login(ctx, u.Username, secrets)
	if token == "" {
		return
	}

	profile := c.api + "/profile/" + escape("@"+u.Username+":"+cfg.ServerName) + "/avatar_url"
	if c.req(ctx, http.MethodGet, profile, "", nil).str("avatar_url") != "" {
		return
	}
	if mxc := c.upload(ctx, u.AvatarFile, token); mxc != "" {
		c.req(ctx, http.MethodPut, profile, token, map[string]any{"avatar_url": mxc})
	}
}

// createRoom creates one room or space, joins the admin to it, sets its icon
// and invites its members.
func (c *client) createRoom(ctx context.Context, cfg Config, room Room, adminToken string, secrets Secrets) {
	body := map[string]any{
		"name":            room.Name,
		"room_alias_name": room.Alias,
		"preset":          "private_chat",
	}
	if room.Space {
		body["creation_content"] = map[string]any{"type": "m.space"}
	}
	c.req(ctx, http.MethodPost, c.api+"/createRoom", adminToken, body)

	roomID := c.resolveID(ctx, cfg, room.Alias)
	if roomID == "" {
		return
	}

	// createRoom only auto-joins the creator the first time, so membership
	// is ensured separately on every run.
	c.req(ctx, http.MethodPost, c.api+"/join/"+escape(roomID), adminToken, nil)

	if room.IconFile != "" {
		avatar := c.api + "/rooms/" + escape(roomID) + "/state/m.room.avatar"
		if c.req(ctx, http.MethodGet, avatar, "", nil).str("url") == "" {
			if mxc := c.upload(ctx, room.IconFile, adminToken); mxc != "" {
				c.req(ctx, http.MethodPut, avatar, adminToken, map[string]any{"url": mxc})
			}
		}
	}

	for _, member := range room.Members {
		c.req(ctx, http.MethodPost, c.api+"/rooms/"+escape(roomID)+"/invite", adminToken,
			map[string]any{"user_id": "@" + member + ":" + cfg.ServerName})
		if token := c.login(ctx, member, secrets); token != "" {
			c.req(ctx, http.MethodPost, c.api+"/join/"+escape(roomID), token, nil)
		}
	}
}

// linkSpace wires one room into its parent space, from both sides.
func (c *client) linkSpace(ctx context.Context, cfg Config, room Room, adminToken string) {
	if room.Parent == "" {
		return
	}
	childID := c.resolveID(ctx, cfg, room.Alias)
	parentID := c.resolveID(ctx, cfg, room.Parent)
	if childID == "" || parentID == "" {
		return
	}

	c.req(ctx, http.MethodPut,
		c.api+"/rooms/"+escape(parentID)+"/state/m.space.child/"+escape(childID),
		adminToken, map[string]any{"via": []string{cfg.ServerName}})

	c.req(ctx, http.MethodPut,
		c.api+"/rooms/"+escape(childID)+"/state/m.space.parent/"+escape(parentID),
		adminToken, map[string]any{"via": []string{cfg.ServerName}, "canonical": true})
}

// resolveID turns a declared alias localpart into the room ID Matrix minted
// for it.
func (c *client) resolveID(ctx context.Context, cfg Config, localpart string) string {
	alias := "#" + localpart + ":" + cfg.ServerName
	return c.req(ctx, http.MethodGet, c.api+"/directory/room/"+escape(alias), "", nil).str("room_id")
}

// upload sends an image and returns its mxc:// URI, or "".
func (c *client) upload(ctx context.Context, path, token string) string {
	data, err := os.ReadFile(path) //nolint:gosec // the path comes from the host's own declaration
	if err != nil {
		return ""
	}

	url := c.base + "/_matrix/media/v3/upload?filename=" + escape(filepath.Base(path))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", avatarContentType)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	if resp.StatusCode >= http.StatusBadRequest {
		fmt.Fprintf(c.stderr, "init: upload failed -> %d %s\n", resp.StatusCode, string(raw)) //nolint:errcheck // reporting only
		return ""
	}

	var body struct {
		ContentURI string `json:"content_uri"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return body.ContentURI
}

// escape percent-encodes a path segment, leaving only the unreserved set —
// what Python's quote(s, safe="") did. A room ID is !abc:server and an alias
// is #room:server; both have to survive as one segment.
func escape(s string) string {
	var b strings.Builder
	for i := range len(s) {
		ch := s[i]
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9',
			ch == '-', ch == '_', ch == '.', ch == '~':
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}
