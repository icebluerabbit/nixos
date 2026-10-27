package tools

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
	"homelab/sys-mcp/internal/config"
)

// searchServer registers the search tools over the shared fixture. SI is a
// real SourceIndex over an empty directory: the handlers call it for every
// result, so a nil one would panic rather than return empty sources.
func searchServer(t *testing.T) *server.MCPServer {
	t.Helper()
	root := t.TempDir()
	ic := cache.NewInfraCacheWithData(testData)
	ac := &app.AppContext{
		Cfg: &config.Config{Hostname: "desktop", FlakePath: root},
		IC:  ic,
		HC: cache.NewHostsCacheWithData(cache.HostsData{
			"server":  {Containers: map[string]cache.ContainerConf{"grafana": {}, "radarr": {}}},
			"desktop": {},
		}),
		SI: cache.NewSourceIndex(root, ic),
	}
	s := server.NewMCPServer("test", "0.0.0")
	RegisterSearchTools(s, ac)
	return s
}

func TestRegisterSearchToolsRegistersCatalog(t *testing.T) {
	names := toolNames(t, searchServer(t))

	for _, want := range []string{"search", "get_by_ip", "get_by_port"} {
		if !has(names, want) {
			t.Errorf("tool %q is not registered; got %v", want, names)
		}
	}
}

// search is the tool used when the kind of the target is unknown, so it has
// to match on a name fragment, an address and a MAC alike.
func TestSearchToolMatchesEveryIdentifier(t *testing.T) {
	s := searchServer(t)

	cases := map[string]string{
		"name fragment": "grafana",
		"ip":            "10.10.140.9",
		"mac":           "62:f5:a4:5b:20:95",
	}
	for what, query := range cases {
		t.Run(what, func(t *testing.T) {
			out, isErr := callTool(t, s, "search", map[string]any{"query": query})
			if isErr {
				t.Fatalf("search reported an error: %s", out)
			}
			if !strings.Contains(out, "grafana") {
				t.Errorf("search by %s missed grafana: %s", what, out)
			}
		})
	}
}

func TestSearchToolRequiresQuery(t *testing.T) {
	if _, isErr := callTool(t, searchServer(t), "search", map[string]any{}); !isErr {
		t.Error("search without a query should be an error result")
	}
}

func TestGetByIPTool(t *testing.T) {
	s := searchServer(t)

	out, isErr := callTool(t, s, "get_by_ip", map[string]any{"ip": "10.10.120.3"})
	if isErr {
		t.Fatalf("get_by_ip reported an error: %s", out)
	}
	if !strings.Contains(out, "radarr") {
		t.Errorf("owner missing from result: %s", out)
	}

	if _, isErr := callTool(t, s, "get_by_ip", map[string]any{"ip": "10.10.99.99"}); !isErr {
		t.Error("an unowned address should be an error result")
	}
}

func TestGetByPortTool(t *testing.T) {
	s := searchServer(t)

	out, isErr := callTool(t, s, "get_by_port", map[string]any{"port": 3000})
	if isErr {
		t.Fatalf("get_by_port reported an error: %s", out)
	}
	if !strings.Contains(out, "grafana") {
		t.Errorf("port owner missing: %s", out)
	}

	if _, isErr := callTool(t, s, "get_by_port", map[string]any{"port": "not-a-number"}); !isErr {
		t.Error("a non-numeric port should be an error result")
	}
}
