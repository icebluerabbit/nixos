package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	hlapp "homelab/app"
)

// syncBuffer collects stdout across the goroutine boundary. The server
// writes its frames from its own goroutine while the test polls for them, so
// a bare bytes.Buffer would be a data race rather than a synchronisation.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func testEnv(stdout io.Writer) *hlapp.Env {
	return &hlapp.Env{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Stdout: stdout,
	}
}

// withFlake points the configuration at a throwaway flake directory. The
// caches evaluate it in the background and are allowed to fail; what matters
// here is that the server answers regardless.
func withFlake(t *testing.T) {
	t.Helper()
	t.Setenv("SYS_MCP_FLAKE_PATH", t.TempDir())
	t.Setenv("SYS_MCP_HOSTNAME", "desktop")
	t.Setenv("UNIFI_URL", "")
	t.Setenv("UNIFI_API_KEY", "")
}

// Run refuses to start without a flake path rather than serving a catalog
// that would answer every question wrongly.
func TestRunRequiresConfiguration(t *testing.T) {
	t.Setenv("SYS_MCP_FLAKE_PATH", "")
	t.Setenv("FLAKE", "")
	t.Setenv("NH_FLAKE", "")

	if err := Run(context.Background(), testEnv(io.Discard)); err == nil {
		t.Fatal("Run started with no flake path configured")
	}
}

// stdout is the protocol: the handshake and the tool catalog are written
// there and nothing else may be.
func TestRunServesTheToolCatalogOverStdio(t *testing.T) {
	withFlake(t)

	stdin, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer stdinWriter.Close()

	// Run reads os.Stdin directly, as an MCP client attaches it.
	realStdin := os.Stdin
	os.Stdin = stdin
	t.Cleanup(func() { os.Stdin = realStdin })

	var stdout syncBuffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, testEnv(&stdout)) }()

	frames := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	}
	for _, f := range frames {
		if _, err := io.WriteString(stdinWriter, f+"\n"); err != nil {
			t.Fatalf("write frame: %v", err)
		}
	}

	// Wait for the tools/list reply rather than a fixed sleep.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(stdout.String(), `"tools"`) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("Run() = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}

	out := stdout.String()
	if !strings.Contains(out, `"tools"`) {
		t.Fatalf("no tool catalog on stdout: %s", out)
	}

	// Every line on stdout must be a JSON-RPC frame; a stray log line there
	// would corrupt the session.
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Errorf("non-JSON on stdout: %q", line)
			continue
		}
		if frame["jsonrpc"] != "2.0" {
			t.Errorf("not a JSON-RPC frame: %q", line)
		}
	}

	// The catalog must carry the registry tools; UniFi is unconfigured here
	// and correctly absent.
	for _, want := range []string{"get_host", "list_containers", "resolve_path"} {
		if !strings.Contains(out, want) {
			t.Errorf("tool %q missing from the catalog", want)
		}
	}
	if strings.Contains(out, "list_unifi_devices") {
		t.Error("UniFi tools registered without a URL or key")
	}
}
