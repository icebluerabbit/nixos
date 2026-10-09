package tools

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
	"homelab/sys-mcp/internal/config"
)

// infraServer registers the registry lookups over the shared test fixture.
func infraServer(t *testing.T) *server.MCPServer {
	t.Helper()
	s := server.NewMCPServer("test", "0.0.0")
	ac := &app.AppContext{
		Cfg: &config.Config{Hostname: "desktop", FlakePath: t.TempDir()},
		IC:  cache.NewInfraCacheWithData(testData),
		HC: cache.NewHostsCacheWithData(cache.HostsData{
			"server":  {Containers: map[string]cache.ContainerConf{"grafana": {}, "radarr": {}}},
			"desktop": {},
		}),
	}
	RegisterInfraTools(s, ac)
	return s
}

// The catalog is a contract: these names are documented in CLAUDE.md and
// called by name from the skills.
func TestRegisterInfraToolsRegistersCatalog(t *testing.T) {
	names := toolNames(t, infraServer(t))

	// get_by_ip and get_by_port are the search package's, not these.
	for _, want := range []string{
		"describe_topology", "get_host", "list_hosts", "get_container",
		"list_containers", "get_client", "list_clients", "get_device",
		"list_devices", "get_vlan", "list_vlans",
	} {
		if !has(names, want) {
			t.Errorf("tool %q is not registered; got %v", want, names)
		}
	}
}

func TestGetHostTool(t *testing.T) {
	s := infraServer(t)

	out, isErr := callTool(t, s, "get_host", map[string]any{"name": "server"})
	if isErr {
		t.Fatalf("get_host reported an error: %s", out)
	}
	if !strings.Contains(out, `"server"`) {
		t.Errorf("host missing from result: %s", out)
	}

	out, isErr = callTool(t, s, "get_host", map[string]any{"name": "nosuchhost"})
	if !isErr {
		t.Errorf("unknown host should be an error result, got: %s", out)
	}
}

func TestListHostsTool(t *testing.T) {
	out, isErr := callTool(t, infraServer(t), "list_hosts", nil)
	if isErr {
		t.Fatalf("list_hosts reported an error: %s", out)
	}
	for _, want := range []string{"server", "desktop"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q missing from list_hosts: %s", want, out)
		}
	}
}

func TestGetContainerTool(t *testing.T) {
	s := infraServer(t)

	out, isErr := callTool(t, s, "get_container", map[string]any{"name": "grafana"})
	if isErr {
		t.Fatalf("get_container reported an error: %s", out)
	}
	if !strings.Contains(out, "grafana") {
		t.Errorf("container missing from result: %s", out)
	}

	if _, isErr := callTool(t, s, "get_container", map[string]any{"name": "nope"}); !isErr {
		t.Error("unknown container should be an error result")
	}
}

func TestListContainersTool(t *testing.T) {
	out, isErr := callTool(t, infraServer(t), "list_containers", nil)
	if isErr {
		t.Fatalf("list_containers reported an error: %s", out)
	}
	if !strings.Contains(out, "grafana") || !strings.Contains(out, "radarr") {
		t.Errorf("containers missing: %s", out)
	}
}

func TestVlanTools(t *testing.T) {
	s := infraServer(t)

	out, isErr := callTool(t, s, "list_vlans", nil)
	if isErr {
		t.Fatalf("list_vlans reported an error: %s", out)
	}
	if !strings.Contains(out, "services") {
		t.Errorf("services VLAN missing: %s", out)
	}

	out, isErr = callTool(t, s, "get_vlan", map[string]any{"name": "services"})
	if isErr {
		t.Fatalf("get_vlan reported an error: %s", out)
	}
	if !strings.Contains(out, "services") {
		t.Errorf("vlan detail missing: %s", out)
	}
}

func TestClientAndDeviceTools(t *testing.T) {
	s := infraServer(t)

	if out, isErr := callTool(t, s, "list_clients", nil); isErr {
		t.Errorf("list_clients reported an error: %s", out)
	}
	if out, isErr := callTool(t, s, "list_devices", nil); isErr {
		t.Errorf("list_devices reported an error: %s", out)
	}
}

// describe_topology is the orientation call, so it must answer with no
// arguments at all.
func TestDescribeTopologyTool(t *testing.T) {
	out, isErr := callTool(t, infraServer(t), "describe_topology", nil)
	if isErr {
		t.Fatalf("describe_topology reported an error: %s", out)
	}
	if out == "" {
		t.Error("describe_topology returned nothing")
	}
}
