package provision

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The MAC is the whole protocol: nonce, username and password each followed
// by a NUL, then the admin flag with none. Get the layout wrong and the
// homeserver rejects every registration without saying why, so these are
// pinned against values produced by the Python implementation.
func TestRegisterMACMatchesTheSharedSecretProtocol(t *testing.T) {
	cases := []struct {
		name                      string
		nonce, username, password string
		admin                     bool
		want                      string
	}{
		{
			name: "admin", nonce: "thenonce", username: "alice", password: "hunter2", admin: true,
			want: "02268c348c50e17a354a4974478b803fc5b843d6",
		},
		{
			name: "notadmin", nonce: "thenonce", username: "bob", password: "pw", admin: false,
			want: "3f532e845c42a6d83a4ba0f494778a2797b287a0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RegisterMAC("sharedsecret", tc.nonce, tc.username, tc.password, tc.admin)
			if got != tc.want {
				t.Errorf("RegisterMAC = %s, want %s", got, tc.want)
			}
		})
	}
}

// A room ID is !abc:server and an alias is #room:server; both have to
// survive as a single path segment.
func TestEscapeMatchesPythonQuote(t *testing.T) {
	cases := map[string]string{
		"#nixos:matrix.services.internal":     "%23nixos%3Amatrix.services.internal",
		"!DOvzP91oFnuCXCsiWA:matrix.internal": "%21DOvzP91oFnuCXCsiWA%3Amatrix.internal",
		"@admin:matrix.internal":              "%40admin%3Amatrix.internal",
		"a-b_c.d~e":                           "a-b_c.d~e",
	}
	for in, want := range cases {
		if got := escape(in); got != want {
			t.Errorf("escape(%q) = %q, want %q", in, got, want)
		}
	}
}

// homeserver is a stand-in for tuwunel, recording what it was asked to do.
type homeserver struct {
	mu sync.Mutex

	// calls records "METHOD /path" for every request.
	calls []string
	// bodies records the decoded JSON body of every request, by call index.
	bodies []map[string]any

	// registered counts successful registrations by username.
	registered map[string]int
	// existing names users the homeserver already knows.
	existing map[string]bool
	// rooms maps an alias to the room ID it resolves to.
	rooms map[string]string
	// avatars records profile and room avatar state that is already set.
	avatars map[string]string
	// uploads counts media uploads.
	uploads int
	// state records m.space.child / m.space.parent writes.
	state []string
}

func newHomeserver() *homeserver {
	return &homeserver{
		registered: map[string]int{},
		existing:   map[string]bool{},
		rooms:      map[string]string{},
		avatars:    map[string]string{},
	}
}

func (h *homeserver) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		defer h.mu.Unlock()

		var body map[string]any
		if r.Body != nil {
			raw, _ := io.ReadAll(r.Body)
			if len(raw) > 0 {
				_ = json.Unmarshal(raw, &body)
			}
		}
		// Recorded in escaped form: a room ID is !abc:server, so the
		// decoded path would not match what the client actually sent.
		esc := r.URL.EscapedPath()
		h.calls = append(h.calls, r.Method+" "+esc)
		h.bodies = append(h.bodies, body)

		path := r.URL.Path
		writeJSON := func(v any) {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(v); err != nil {
				t.Errorf("encode: %v", err)
			}
		}

		switch {
		case path == "/_matrix/client/versions":
			writeJSON(map[string]any{"versions": []string{"v1.11"}})

		case path == "/_synapse/admin/v1/register" && r.Method == http.MethodGet:
			writeJSON(map[string]any{"nonce": "thenonce"})

		case path == "/_synapse/admin/v1/register":
			name, _ := body["username"].(string)
			if h.existing[name] {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"errcode":"M_USER_IN_USE","error":"User ID is not available"}`)
				return
			}
			h.registered[name]++
			h.existing[name] = true
			writeJSON(map[string]any{"user_id": "@" + name + ":matrix.test"})

		case path == "/_matrix/client/v3/login":
			name, _ := body["identifier"].(map[string]any)["user"].(string)
			writeJSON(map[string]any{"access_token": "token-" + name})

		case strings.HasPrefix(path, "/_matrix/media/v3/upload"):
			h.uploads++
			writeJSON(map[string]any{"content_uri": "mxc://matrix.test/abc"})

		case strings.HasPrefix(path, "/_matrix/client/v3/profile/"):
			if r.Method == http.MethodGet {
				writeJSON(map[string]any{"avatar_url": h.avatars[esc]})
				return
			}
			url, _ := body["avatar_url"].(string)
			h.avatars[esc] = url
			writeJSON(map[string]any{})

		case path == "/_matrix/client/v3/createRoom":
			alias, _ := body["room_alias_name"].(string)
			if _, ok := h.rooms[alias]; ok {
				w.WriteHeader(http.StatusConflict)
				_, _ = io.WriteString(w, `{"errcode":"M_ROOM_IN_USE","error":"Room alias already taken"}`)
				return
			}
			h.rooms[alias] = "!" + alias + ":matrix.test"
			writeJSON(map[string]any{"room_id": h.rooms[alias]})

		case strings.HasPrefix(path, "/_matrix/client/v3/directory/room/"):
			alias := strings.TrimPrefix(path, "/_matrix/client/v3/directory/room/")
			local := strings.TrimPrefix(strings.SplitN(alias, ":", 2)[0], "#")
			id, ok := h.rooms[local]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"errcode":"M_NOT_FOUND"}`)
				return
			}
			writeJSON(map[string]any{"room_id": id})

		case strings.Contains(path, "/state/m.room.avatar"):
			if r.Method == http.MethodGet {
				writeJSON(map[string]any{"url": h.avatars[esc]})
				return
			}
			url, _ := body["url"].(string)
			h.avatars[esc] = url
			writeJSON(map[string]any{})

		case strings.Contains(path, "/state/m.space."):
			h.state = append(h.state, path)
			writeJSON(map[string]any{"event_id": "$x"})

		default:
			writeJSON(map[string]any{})
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// port extracts the port from the stub's URL, which is what Run takes.
func port(t *testing.T, url string) string {
	t.Helper()
	parts := strings.Split(url, ":")
	return parts[len(parts)-1]
}

func (h *homeserver) called(method, path string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.calls {
		if c == method+" "+path {
			n++
		}
	}
	return n
}

// testSecrets gives every user a password.
func testSecrets() Secrets {
	return Secrets{
		RegSecret: "sharedsecret",
		PassFor:   func(u string) string { return "pw-" + u },
	}
}

func fast() Options { return Options{ReadyDelay: time.Millisecond} }

func TestRunRegistersDeclaredUsers(t *testing.T) {
	h := newHomeserver()
	url := h.serve(t)

	cfg := Config{
		Port:       port(t, url),
		ServerName: "matrix.test",
		AdminUser:  "admin",
		Users: []User{
			{Username: "admin", Admin: true},
			{Username: "hermes"},
		},
	}
	if err := Run(context.Background(), io.Discard, cfg, testSecrets(), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if h.registered["admin"] != 1 || h.registered["hermes"] != 1 {
		t.Errorf("registered = %v", h.registered)
	}

	// The MAC and the admin flag have to travel with the registration.
	h.mu.Lock()
	defer h.mu.Unlock()
	var sawAdminFlag bool
	for i, call := range h.calls {
		if call != "POST /_synapse/admin/v1/register" {
			continue
		}
		body := h.bodies[i]
		name, _ := body["username"].(string)
		admin, _ := body["admin"].(bool)
		mac, _ := body["mac"].(string)
		want := RegisterMAC("sharedsecret", "thenonce", name, "pw-"+name, admin)
		if mac != want {
			t.Errorf("%s mac = %s, want %s", name, mac, want)
		}
		if name == "admin" && admin {
			sawAdminFlag = true
		}
	}
	if !sawAdminFlag {
		t.Error("the admin user was not registered as an admin")
	}
}

// This runs on every container start, so an existing account must be a
// no-op rather than a reported failure.
func TestRunSkipsExistingUsersQuietly(t *testing.T) {
	h := newHomeserver()
	h.existing["admin"] = true
	url := h.serve(t)

	var stderr bytes.Buffer
	cfg := Config{
		Port: port(t, url), ServerName: "matrix.test", AdminUser: "admin",
		Users: []User{{Username: "admin", Admin: true}},
	}
	if err := Run(context.Background(), &stderr, cfg, testSecrets(), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if strings.Contains(stderr.String(), "register admin") {
		t.Errorf("an existing user was reported as a failure: %q", stderr.String())
	}
}

// A registration that fails for any other reason is reported, because that
// one is not expected.
func TestRunReportsRealRegistrationFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/_matrix/client/versions":
			_, _ = io.WriteString(w, `{}`)
		case r.URL.Path == "/_synapse/admin/v1/register" && r.Method == http.MethodGet:
			_, _ = io.WriteString(w, `{"nonce":"thenonce"}`)
		case r.URL.Path == "/_synapse/admin/v1/register":
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"errcode":"M_FORBIDDEN","error":"bad mac"}`)
		default:
			_, _ = io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(srv.Close)

	var stderr bytes.Buffer
	cfg := Config{
		Port: port(t, srv.URL), ServerName: "matrix.test", AdminUser: "admin",
		Users: []User{{Username: "admin", Admin: true}},
	}
	if err := Run(context.Background(), &stderr, cfg, testSecrets(), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(stderr.String(), "register admin -> 403") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// A user with no password configured cannot be created, and saying so beats
// registering it with an empty one.
func TestRunSkipsUsersWithoutAPassword(t *testing.T) {
	h := newHomeserver()
	url := h.serve(t)

	var stderr bytes.Buffer
	cfg := Config{
		Port: port(t, url), ServerName: "matrix.test", AdminUser: "admin",
		Users: []User{{Username: "ghost"}},
	}
	secrets := Secrets{RegSecret: "sharedsecret", PassFor: func(string) string { return "" }}
	if err := Run(context.Background(), &stderr, cfg, secrets, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if h.registered["ghost"] != 0 {
		t.Error("a user with no password was registered")
	}
	if !strings.Contains(stderr.String(), "no password for ghost") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunCreatesRoomsAndSpaces(t *testing.T) {
	h := newHomeserver()
	url := h.serve(t)

	cfg := Config{
		Port: port(t, url), ServerName: "matrix.test", AdminUser: "admin",
		Users: []User{{Username: "admin", Admin: true}, {Username: "hermes"}},
		Rooms: []Room{
			{Name: "Homelab", Alias: "homelab", Space: true},
			{Name: "NixOS", Alias: "nixos", Parent: "homelab", Members: []string{"hermes"}},
		},
	}
	if err := Run(context.Background(), io.Discard, cfg, testSecrets(), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.rooms) != 2 {
		t.Fatalf("rooms = %v", h.rooms)
	}

	// The space carries its creation_content; the room does not.
	var spaceCreated bool
	for i, call := range h.calls {
		if call != "POST /_matrix/client/v3/createRoom" {
			continue
		}
		alias, _ := h.bodies[i]["room_alias_name"].(string)
		_, hasCreation := h.bodies[i]["creation_content"]
		if alias == "homelab" && hasCreation {
			spaceCreated = true
		}
		if alias == "nixos" && hasCreation {
			t.Error("a plain room was created as a space")
		}
	}
	if !spaceCreated {
		t.Error("the space was not created with creation_content")
	}

	// Both sides of the hierarchy are written, which is what makes the
	// child show up inside the space.
	var child, parent bool
	for _, s := range h.state {
		if strings.Contains(s, "m.space.child") {
			child = true
		}
		if strings.Contains(s, "m.space.parent") {
			parent = true
		}
	}
	if !child || !parent {
		t.Errorf("hierarchy state = %v", h.state)
	}
}

// createRoom only auto-joins the creator the first time, so the admin join
// has to happen on every run or a re-run leaves the admin outside.
func TestRunAlwaysJoinsTheAdmin(t *testing.T) {
	h := newHomeserver()
	h.rooms["nixos"] = "!nixos:matrix.test" // already created by an earlier run
	url := h.serve(t)

	cfg := Config{
		Port: port(t, url), ServerName: "matrix.test", AdminUser: "admin",
		Users: []User{{Username: "admin", Admin: true}},
		Rooms: []Room{{Name: "NixOS", Alias: "nixos"}},
	}
	if err := Run(context.Background(), io.Discard, cfg, testSecrets(), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if n := h.called(http.MethodPost, "/_matrix/client/v3/join/"+escape("!nixos:matrix.test")); n == 0 {
		t.Error("the admin was not joined to an existing room")
	}
}

// Members are invited and then join with their own token, which is what
// makes a private room actually contain them.
func TestRunInvitesAndJoinsMembers(t *testing.T) {
	h := newHomeserver()
	url := h.serve(t)

	cfg := Config{
		Port: port(t, url), ServerName: "matrix.test", AdminUser: "admin",
		Users: []User{{Username: "admin", Admin: true}, {Username: "hermes"}},
		Rooms: []Room{{Name: "NixOS", Alias: "nixos", Members: []string{"hermes"}}},
	}
	if err := Run(context.Background(), io.Discard, cfg, testSecrets(), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	roomID := escape("!nixos:matrix.test")
	if n := h.called(http.MethodPost, "/_matrix/client/v3/rooms/"+roomID+"/invite"); n != 1 {
		t.Errorf("invites = %d, want 1", n)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	var invited string
	for i, call := range h.calls {
		if strings.HasSuffix(call, "/invite") {
			invited, _ = h.bodies[i]["user_id"].(string)
		}
	}
	if invited != "@hermes:matrix.test" {
		t.Errorf("invited = %q, want @hermes:matrix.test", invited)
	}
}

// An avatar already in place is left alone, so a person who changed theirs
// keeps it across restarts.
func TestRunLeavesExistingAvatarsAlone(t *testing.T) {
	h := newHomeserver()
	profile := "/_matrix/client/v3/profile/" + escape("@admin:matrix.test") + "/avatar_url"
	h.avatars[profile] = "mxc://matrix.test/already"
	url := h.serve(t)

	avatar := filepath.Join(t.TempDir(), "avatar.png")
	if err := os.WriteFile(avatar, []byte("not really a png"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Port: port(t, url), ServerName: "matrix.test", AdminUser: "admin",
		Users: []User{{Username: "admin", Admin: true, AvatarFile: avatar}},
	}
	if err := Run(context.Background(), io.Discard, cfg, testSecrets(), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.uploads != 0 {
		t.Errorf("uploads = %d, want none when an avatar is already set", h.uploads)
	}
	if h.avatars[profile] != "mxc://matrix.test/already" {
		t.Errorf("the existing avatar was replaced: %q", h.avatars[profile])
	}
}

func TestRunSetsMissingAvatars(t *testing.T) {
	h := newHomeserver()
	url := h.serve(t)

	avatar := filepath.Join(t.TempDir(), "avatar.png")
	if err := os.WriteFile(avatar, []byte("not really a png"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Port: port(t, url), ServerName: "matrix.test", AdminUser: "admin",
		Users: []User{{Username: "admin", Admin: true, AvatarFile: avatar}},
		Rooms: []Room{{Name: "NixOS", Alias: "nixos", IconFile: avatar}},
	}
	if err := Run(context.Background(), io.Discard, cfg, testSecrets(), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.uploads != 2 {
		t.Errorf("uploads = %d, want 2 (one profile, one room)", h.uploads)
	}
}

// A declared avatar file that is not there is skipped rather than failing
// the provisioning run.
func TestRunToleratesMissingAvatarFile(t *testing.T) {
	h := newHomeserver()
	url := h.serve(t)

	cfg := Config{
		Port: port(t, url), ServerName: "matrix.test", AdminUser: "admin",
		Users: []User{{Username: "admin", Admin: true, AvatarFile: "/no/such/avatar.png"}},
	}
	if err := Run(context.Background(), io.Discard, cfg, testSecrets(), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.uploads != 0 {
		t.Errorf("uploads = %d, want none", h.uploads)
	}
}

// The unit starts alongside the homeserver rather than after it, so the
// first checks routinely fail.
func TestRunWaitsForTheHomeserver(t *testing.T) {
	var mu sync.Mutex
	refusals := 2

	h := newHomeserver()
	inner := h.serve(t)

	// A proxy in front of the stub that refuses the first few checks.
	gate := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if r.URL.Path == "/_matrix/client/versions" && refusals > 0 {
			refusals--
			mu.Unlock()
			hj, ok := w.(http.Hijacker)
			if !ok {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			conn, _, err := hj.Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		mu.Unlock()

		req, err := http.NewRequestWithContext(r.Context(), r.Method, inner+r.URL.RequestURI(), r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		req.Header = r.Header.Clone()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	t.Cleanup(gate.Close)

	cfg := Config{
		Port: port(t, gate.URL), ServerName: "matrix.test", AdminUser: "admin",
		Users: []User{{Username: "admin", Admin: true}},
	}
	if err := Run(context.Background(), io.Discard, cfg, testSecrets(), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if h.registered["admin"] != 1 {
		t.Errorf("registered = %v after the homeserver came up", h.registered)
	}
}

// A homeserver that never comes up stops the run when the context is
// cancelled rather than waiting forever.
func TestRunStopsWaitingWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := Config{Port: "1", ServerName: "matrix.test", AdminUser: "admin"}

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, io.Discard, cfg, testSecrets(), Options{ReadyDelay: time.Hour})
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Run returned nil for a cancelled context")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run ignored its cancelled context")
	}
}

// A room whose parent does not resolve is left unlinked rather than failing
// the whole run.
func TestRunSkipsHierarchyWithAMissingParent(t *testing.T) {
	h := newHomeserver()
	url := h.serve(t)

	cfg := Config{
		Port: port(t, url), ServerName: "matrix.test", AdminUser: "admin",
		Users: []User{{Username: "admin", Admin: true}},
		Rooms: []Room{{Name: "NixOS", Alias: "nixos", Parent: "nosuchspace"}},
	}
	if err := Run(context.Background(), io.Discard, cfg, testSecrets(), fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.state) != 0 {
		t.Errorf("hierarchy was written for a missing parent: %v", h.state)
	}
}

func TestEnvSecrets(t *testing.T) {
	t.Setenv("TUWUNEL_REG_SECRET", "s3cret")
	t.Setenv("TUWUNEL_HOME_ASSISTANT_PASS", "ha-pass")

	got := EnvSecrets()
	if got.RegSecret != "s3cret" {
		t.Errorf("RegSecret = %q", got.RegSecret)
	}
	// A dash in the username becomes an underscore in the variable name.
	if pw := got.PassFor("home-assistant"); pw != "ha-pass" {
		t.Errorf("PassFor(home-assistant) = %q, want ha-pass", pw)
	}
	if pw := got.PassFor("nobody"); pw != "" {
		t.Errorf("PassFor(nobody) = %q, want empty", pw)
	}
}

func TestOptionDefaults(t *testing.T) {
	got := Options{}.withDefaults()

	if got.ReadyDelay != DefaultReadyDelay {
		t.Errorf("ReadyDelay = %v, want %v", got.ReadyDelay, DefaultReadyDelay)
	}
	if got.Client == nil {
		t.Error("no HTTP client")
	}
}
