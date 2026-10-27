package tools

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
	"homelab/sys-mcp/internal/config"
)

// provisionData is the shared fixture plus the DHCP pool boundaries the
// allocators need: statics live below dhcpStart, and the tool refuses to hand
// out an address at or above it.
func provisionData() *cache.InfraData {
	data := *testData
	vlans := make(map[string]cache.Vlan, len(testData.Vlans))
	for name, v := range testData.Vlans {
		v.DhcpStart = strings.Replace(v.Subnet, ".0", ".100", 1)
		v.DhcpStop = strings.Replace(v.Subnet, ".0", ".200", 1)
		vlans[name] = v
	}
	data.Vlans = vlans
	return &data
}

// provisionServer registers the allocators over that fixture.
func provisionServer(t *testing.T) *server.MCPServer {
	t.Helper()
	root := t.TempDir()
	ic := cache.NewInfraCacheWithData(provisionData())
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
	RegisterProvisionTools(s, ac)
	return s
}

func TestRegisterProvisionToolsRegistersCatalog(t *testing.T) {
	names := toolNames(t, provisionServer(t))

	for _, want := range []string{"next_free_ip", "next_free_mac", "validate_container"} {
		if !has(names, want) {
			t.Errorf("tool %q is not registered; got %v", want, names)
		}
	}
}

// The services VLAN has statics at .1, .8 and .9, so the next allocation is
// .10: one past the highest, never a reused gap.
func TestNextFreeIPTool(t *testing.T) {
	s := provisionServer(t)

	out, isErr := callTool(t, s, "next_free_ip", map[string]any{"vlan": "services"})
	if isErr {
		t.Fatalf("next_free_ip reported an error: %s", out)
	}
	if !strings.Contains(out, "10.10.140.10") {
		t.Errorf("want 10.10.140.10 in result: %s", out)
	}
}

func TestNextFreeIPToolRejectsUnknownVlan(t *testing.T) {
	if _, isErr := callTool(t, provisionServer(t), "next_free_ip", map[string]any{"vlan": "nope"}); !isErr {
		t.Error("unknown VLAN should be an error result")
	}
}

func TestNextFreeIPToolRequiresVlan(t *testing.T) {
	if _, isErr := callTool(t, provisionServer(t), "next_free_ip", map[string]any{}); !isErr {
		t.Error("missing vlan should be an error result")
	}
}

// The MAC is derived, not stored: OUI plus vlanId + index, with the bridge
// MAC alongside it.
func TestNextFreeMACTool(t *testing.T) {
	out, isErr := callTool(t, provisionServer(t), "next_free_mac", map[string]any{"vlan": "services"})
	if isErr {
		t.Fatalf("next_free_mac reported an error: %s", out)
	}
	if !strings.Contains(out, containerMACOUI) {
		t.Errorf("container OUI missing: %s", out)
	}
	if !strings.Contains(out, bridgeMACOUI) {
		t.Errorf("bridge OUI missing: %s", out)
	}
}

func TestNextFreeMACToolRejectsUnknownVlan(t *testing.T) {
	if _, isErr := callTool(t, provisionServer(t), "next_free_mac", map[string]any{"vlan": "nope"}); !isErr {
		t.Error("unknown VLAN should be an error result")
	}
}

// validate_container reports per-check results rather than failing outright,
// so a fully-wired container and a missing one both answer.
func TestValidateContainerTool(t *testing.T) {
	s := provisionServer(t)

	out, isErr := callTool(t, s, "validate_container", map[string]any{"name": "grafana"})
	if isErr {
		t.Fatalf("validate_container reported an error: %s", out)
	}
	if !strings.Contains(out, "check") {
		t.Errorf("no checks in result: %s", out)
	}

	out, _ = callTool(t, s, "validate_container", map[string]any{"name": "nosuchcontainer"})
	if out == "" {
		t.Error("validate_container returned nothing for an unknown container")
	}
}

func TestValidateContainerToolRequiresName(t *testing.T) {
	if _, isErr := callTool(t, provisionServer(t), "validate_container", map[string]any{}); !isErr {
		t.Error("missing name should be an error result")
	}
}
