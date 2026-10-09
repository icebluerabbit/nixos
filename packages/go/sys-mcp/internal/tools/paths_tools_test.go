package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
	"homelab/sys-mcp/internal/config"
)

// pathsServer registers resolve_path over a miniature flake on disk.
func pathsServer(t *testing.T) (s *server.MCPServer, flakeRoot string) {
	t.Helper()
	root := makePathFlake(t)
	ic := cache.NewInfraCacheWithData(testData)
	ac := &app.AppContext{
		Cfg: &config.Config{Hostname: "desktop", FlakePath: root},
		IC:  ic,
		HC: cache.NewHostsCacheWithData(cache.HostsData{
			"server":  {Containers: map[string]cache.ContainerConf{"bazarr": {}}},
			"desktop": {},
		}),
		SI: cache.NewSourceIndex(root, ic),
	}
	s = server.NewMCPServer("test", "0.0.0")
	RegisterPathTools(s, ac)
	return s, root
}

func TestRegisterPathToolsRegistersCatalog(t *testing.T) {
	s, _ := pathsServer(t)

	if names := toolNames(t, s); !has(names, "resolve_path") {
		t.Errorf("resolve_path is not registered; got %v", names)
	}
}

// The result is metadata followed by the file itself, so a caller gets the
// context and the content in one call.
func TestResolvePathToolReturnsMetadataAndContent(t *testing.T) {
	s, _ := pathsServer(t)

	out, isErr := callTool(t, s, "resolve_path",
		map[string]any{"path": "modules/containers/bazarr.nix"})
	if isErr {
		t.Fatalf("resolve_path reported an error: %s", out)
	}
	if !strings.Contains(out, `"type"`) {
		t.Errorf("metadata missing: %s", out)
	}
	if !strings.Contains(out, "# bazarr container aspect") {
		t.Errorf("file content missing: %s", out)
	}
}

// An absolute path is accepted and reduced to the flake-relative one.
func TestResolvePathToolAcceptsAbsolutePath(t *testing.T) {
	s, root := pathsServer(t)

	out, isErr := callTool(t, s, "resolve_path",
		map[string]any{"path": filepath.Join(root, "modules", "cli", "just.nix")})
	if isErr {
		t.Fatalf("resolve_path reported an error: %s", out)
	}
	if !strings.Contains(out, "# just cli aspect") {
		t.Errorf("file content missing: %s", out)
	}
}

// A container aspect can be a directory; those are listed rather than read.
func TestResolvePathToolListsDirectories(t *testing.T) {
	s, root := pathsServer(t)

	dir := filepath.Join(root, "modules", "containers", "matrix")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tuwunel.nix"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, isErr := callTool(t, s, "resolve_path",
		map[string]any{"path": "modules/containers/matrix"})
	if isErr {
		t.Fatalf("resolve_path reported an error: %s", out)
	}
	if !strings.Contains(out, "--- directory:") {
		t.Errorf("directory listing missing: %s", out)
	}
	if !strings.Contains(out, "tuwunel.nix") {
		t.Errorf("directory entry missing: %s", out)
	}
}

func TestResolvePathToolReportsMissingFile(t *testing.T) {
	s, _ := pathsServer(t)

	out, isErr := callTool(t, s, "resolve_path",
		map[string]any{"path": "modules/containers/nosuchthing.nix"})
	if !isErr {
		t.Errorf("a missing file should be an error result: %s", out)
	}
}

func TestResolvePathToolRequiresPath(t *testing.T) {
	s, _ := pathsServer(t)

	if _, isErr := callTool(t, s, "resolve_path", map[string]any{}); !isErr {
		t.Error("missing path should be an error result")
	}
}
