package resolve

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

// fast keeps the retry behaviour without its wall clock: the real budget is
// two minutes, which is right in a container and wrong in a test.
func fast() Options {
	return Options{Retries: 3, RetryDelay: time.Millisecond}
}

// directory is a stand-in for the Matrix directory endpoint.
type directory struct {
	mu sync.Mutex
	// rooms maps an alias to the room ID it resolves to.
	rooms map[string]string
	// failures counts how many lookups fail before answering, per alias.
	failures map[string]int
	// paths records the raw request paths, to assert the escaping.
	paths []string
}

func (d *directory) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.paths = append(d.paths, r.URL.EscapedPath())

		alias := strings.TrimPrefix(r.URL.Path, directoryPath)
		if n := d.failures[alias]; n > 0 {
			d.failures[alias] = n - 1
			w.WriteHeader(http.StatusNotFound)
			return
		}
		roomID, ok := d.rooms[alias]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"room_id": roomID}); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func (d *directory) requestPaths() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.paths...)
}

// envLines reads back what was appended to the .env.
func envLines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

// envFile returns a path to a .env holding what the activation script wrote.
func envFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hermes.env")
	if err := os.WriteFile(path, []byte("EXISTING=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunResolvesHomeAndFreeRooms(t *testing.T) {
	d := &directory{rooms: map[string]string{
		"#hermes:matrix.test": "!home:matrix.test",
		"#one:matrix.test":    "!one:matrix.test",
		"#two:matrix.test":    "!two:matrix.test",
	}}
	env := envFile(t)

	var stdout, stderr bytes.Buffer
	cfg := Config{
		Homeserver:  d.serve(t),
		EnvFile:     env,
		HomeAlias:   "#hermes:matrix.test",
		FreeAliases: []string{"#one:matrix.test", "#two:matrix.test"},
	}
	if err := Run(context.Background(), &stdout, &stderr, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	lines := envLines(t, env)
	want := []string{
		"EXISTING=value",
		HomeRoomKey + "=!home:matrix.test",
		FreeResponseKey + "=!one:matrix.test,!two:matrix.test",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("env =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}

	// Whatever is appended is echoed, so the journal shows what the gateway
	// will start with.
	for _, line := range want[1:] {
		if !strings.Contains(stdout.String(), line) {
			t.Errorf("stdout is missing %q: %s", line, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// A room alias is #room:server, and both characters have to be
// percent-encoded for the directory endpoint to see a single path segment.
func TestRunEscapesTheAlias(t *testing.T) {
	d := &directory{rooms: map[string]string{"#hermes:matrix.test": "!home:matrix.test"}}
	env := envFile(t)

	cfg := Config{
		Homeserver: d.serve(t),
		EnvFile:    env,
		HomeAlias:  "#hermes:matrix.test",
	}
	if err := Run(context.Background(), io.Discard, io.Discard, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	paths := d.requestPaths()
	if len(paths) == 0 {
		t.Fatal("no request was made")
	}
	want := directoryPath + "%23hermes%3Amatrix.test"
	if paths[0] != want {
		t.Errorf("path = %q, want %q", paths[0], want)
	}
}

func TestEscapeAliasMatchesPythonQuote(t *testing.T) {
	// The right-hand sides came from urllib.parse.quote(alias, safe="").
	cases := map[string]string{
		"#hermes:matrix.example.com": "%23hermes%3Amatrix.example.com",
		"#a-b_c.d~e":                 "%23a-b_c.d~e",
		"#room with space:h":         "%23room%20with%20space%3Ah",
		"#ünïcode:h":                 "%23%C3%BCn%C3%AFcode%3Ah",
	}
	for alias, want := range cases {
		if got := escapeAlias(alias); got != want {
			t.Errorf("escapeAlias(%q) = %q, want %q", alias, got, want)
		}
	}
}

// tuwunel creates these rooms at its own startup, so the first lookups
// routinely 404 while it works through them.
func TestRunRetriesWhileTheRoomIsCreated(t *testing.T) {
	d := &directory{
		rooms:    map[string]string{"#hermes:matrix.test": "!home:matrix.test"},
		failures: map[string]int{"#hermes:matrix.test": 2},
	}
	env := envFile(t)

	cfg := Config{Homeserver: d.serve(t), EnvFile: env, HomeAlias: "#hermes:matrix.test"}
	if err := Run(context.Background(), io.Discard, io.Discard, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := envLines(t, env); len(got) != 2 || !strings.HasPrefix(got[1], HomeRoomKey+"=") {
		t.Errorf("env = %v", got)
	}
}

// An unresolved home room is not fatal: the gateway runs, and only
// unattended output has nowhere to go.
func TestRunToleratesUnresolvedHomeRoom(t *testing.T) {
	d := &directory{rooms: map[string]string{}}
	env := envFile(t)

	var stdout, stderr bytes.Buffer
	cfg := Config{Homeserver: d.serve(t), EnvFile: env, HomeAlias: "#missing:matrix.test"}
	if err := Run(context.Background(), &stdout, &stderr, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := envLines(t, env); len(got) != 1 {
		t.Errorf("env was written to: %v", got)
	}
	if !strings.Contains(stderr.String(), "could not resolve #missing:matrix.test") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q", stdout.String())
	}
}

// Same for the free-response rooms: an @mention stays required, which is a
// working gateway, not a broken one.
func TestRunToleratesUnresolvedFreeRooms(t *testing.T) {
	d := &directory{rooms: map[string]string{}}
	env := envFile(t)

	var stderr bytes.Buffer
	cfg := Config{
		Homeserver:  d.serve(t),
		EnvFile:     env,
		HomeAlias:   NoHomeRoom,
		FreeAliases: []string{"#one:matrix.test"},
	}
	if err := Run(context.Background(), io.Discard, &stderr, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := envLines(t, env); len(got) != 1 {
		t.Errorf("env was written to: %v", got)
	}
	if !strings.Contains(stderr.String(), "@mention stays required") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// The resolvable ones still get written when a sibling cannot be found.
func TestRunKeepsTheFreeRoomsItCouldResolve(t *testing.T) {
	d := &directory{rooms: map[string]string{"#two:matrix.test": "!two:matrix.test"}}
	env := envFile(t)

	cfg := Config{
		Homeserver:  d.serve(t),
		EnvFile:     env,
		HomeAlias:   NoHomeRoom,
		FreeAliases: []string{"#one:matrix.test", "#two:matrix.test"},
	}
	if err := Run(context.Background(), io.Discard, io.Discard, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	lines := envLines(t, env)
	if len(lines) != 2 || lines[1] != FreeResponseKey+"=!two:matrix.test" {
		t.Errorf("env = %v", lines)
	}
}

// The placeholder means no home room was declared, so nothing is looked up.
func TestRunSkipsHomeRoomPlaceholder(t *testing.T) {
	d := &directory{rooms: map[string]string{}}
	env := envFile(t)

	cfg := Config{Homeserver: d.serve(t), EnvFile: env, HomeAlias: NoHomeRoom}
	if err := Run(context.Background(), io.Discard, io.Discard, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if paths := d.requestPaths(); len(paths) != 0 {
		t.Errorf("the placeholder was looked up: %v", paths)
	}
	if got := envLines(t, env); len(got) != 1 {
		t.Errorf("env = %v", got)
	}
}

// A body that is not what the endpoint promises is treated as unresolved
// rather than written through as a room ID.
func TestRunIgnoresMalformedResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"not_a_room":"!x:y"}`)
	}))
	t.Cleanup(srv.Close)
	env := envFile(t)

	var stderr bytes.Buffer
	cfg := Config{Homeserver: srv.URL, EnvFile: env, HomeAlias: "#hermes:matrix.test"}
	if err := Run(context.Background(), io.Discard, &stderr, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := envLines(t, env); len(got) != 1 {
		t.Errorf("env was written to: %v", got)
	}
	if !strings.Contains(stderr.String(), "could not resolve") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// A homeserver that is not there is the same situation as a room that does
// not exist: retried, then given up on without failing the unit.
func TestRunToleratesUnreachableHomeserver(t *testing.T) {
	env := envFile(t)

	var stderr bytes.Buffer
	cfg := Config{Homeserver: "http://127.0.0.1:1", EnvFile: env, HomeAlias: "#hermes:matrix.test"}
	if err := Run(context.Background(), io.Discard, &stderr, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(stderr.String(), "could not resolve") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// An env file that cannot be written is a real failure: the gateway would
// start without the rooms and nothing would say why.
func TestRunReportsUnwritableEnvFile(t *testing.T) {
	d := &directory{rooms: map[string]string{"#hermes:matrix.test": "!home:matrix.test"}}

	cfg := Config{
		Homeserver: d.serve(t),
		EnvFile:    filepath.Join(t.TempDir(), "no-such-dir", "hermes.env"),
		HomeAlias:  "#hermes:matrix.test",
	}
	err := Run(context.Background(), io.Discard, io.Discard, cfg, fast())

	if err == nil || !strings.Contains(err.Error(), "hermes.env") {
		t.Errorf("error = %v, want the env file named", err)
	}
}

// A cancelled context stops the retry loop rather than sitting out the
// budget.
func TestRunStopsRetryingWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	env := envFile(t)

	cfg := Config{Homeserver: "http://127.0.0.1:1", EnvFile: env, HomeAlias: "#hermes:matrix.test"}

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, io.Discard, io.Discard, cfg, Options{Retries: 1000, RetryDelay: time.Hour})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run ignored its cancelled context")
	}
}

// The defaults are what the container runs with.
func TestOptionDefaults(t *testing.T) {
	got := Options{}.withDefaults()

	if got.Retries != DefaultRetries || got.RetryDelay != DefaultRetryDelay {
		t.Errorf("retries = %d x %v, want %d x %v",
			got.Retries, got.RetryDelay, DefaultRetries, DefaultRetryDelay)
	}
	if got.Client == nil {
		t.Error("no HTTP client")
	}
}
