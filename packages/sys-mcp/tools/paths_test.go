package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sys-mcp/cache"
)

// makePathFlake builds a minimal flake dir with representative files for each path category.
func makePathFlake(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	dirs := []string{
		"modules/containers",
		"modules/infra/clients",
		"modules/infra/networks",
		"modules/infra/devices",
		"modules/infra/wlans",
		"modules/infra/portforwards",
		"modules/hosts/server/containers",
		"modules/hosts/desktop",
		"modules/cli",
		"modules/apps",
		"packages/sys-mcp",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	files := map[string]string{
		"modules/containers/bazarr.nix":                   "# bazarr container aspect",
		"modules/infra/clients/container-bazarr.nix":      "# bazarr client",
		"modules/infra/networks/media.nix":                "# media vlan",
		"modules/infra/devices/dream-machine-pro-max.nix": "# udm device",
		"modules/infra/wlans/home.nix":                    "# home wlan",
		"modules/infra/portforwards/http.nix":             "# http portforward",
		"modules/hosts/server/containers/bazarr.nix":      "# bazarr host config",
		"modules/hosts/desktop/host.nix":                  "# desktop host profile",
		"modules/cli/just.nix":                            "# just cli aspect",
		"modules/apps/chromium.nix":                       "# chromium app aspect",
		"packages/sys-mcp/main.go":                        "package main",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// makePathAC builds an AppContext with pre-loaded data and a flake dir.
func makePathAC(t *testing.T) (*testAppContext, string) {
	t.Helper()
	root := makePathFlake(t)
	ac := &testAppContext{
		flakePath: root,
		ic: cache.NewInfraCacheWithData(&cache.InfraData{
			Vlans: map[string]cache.Vlan{
				"media": {VlanId: 120, Subnet: "10.10.120.0", Domain: "media.internal",
					StaticAddresses: map[string]string{"bazarr": "10.10.120.5"}},
			},
			Clients: map[string]cache.Client{
				"container-bazarr": {
					Mac:   "62:f5:a4:5b:20:79",
					Ip:    strPtr("10.10.120.5"),
					Ports: map[string]int{"default": 6767},
				},
			},
			Devices: map[string]cache.Device{
				"dream-machine-pro-max": {Mac: "74:fa:29:50:06:e5", Ip: "10.100.10.1"},
			},
		}),
		hc: cache.NewHostsCacheWithData(cache.HostsData{
			"server": {Containers: map[string]cache.ContainerConf{"bazarr": {}}},
		}),
	}
	return ac, root
}

// testAppContext wraps the fields needed by classifyPath / enrichers without importing app.
// We call the functions directly so we need to satisfy the same interface the functions use.
// Since classifyPath takes *app.AppContext, we use makeHostAC-style real AppContext here.

// --- toRelPath ---

func TestToRelPath_AbsoluteStripped(t *testing.T) {
	got := toRelPath("/etc/nixos/modules/containers/bazarr.nix", "/etc/nixos")
	if got != "modules/containers/bazarr.nix" {
		t.Errorf("got %q", got)
	}
}

func TestToRelPath_AlreadyRelative(t *testing.T) {
	got := toRelPath("modules/containers/bazarr.nix", "/etc/nixos")
	if got != "modules/containers/bazarr.nix" {
		t.Errorf("got %q", got)
	}
}

func TestToRelPath_TrailingSlashFlake(t *testing.T) {
	got := toRelPath("/etc/nixos/modules/cli/just.nix", "/etc/nixos/")
	if got != "modules/cli/just.nix" {
		t.Errorf("got %q", got)
	}
}

// --- parseHostContainerConfig ---

func TestParseHostContainerConfig_Match(t *testing.T) {
	host, name, ok := parseHostContainerConfig("modules/hosts/server/containers/bazarr.nix")
	if !ok || host != "server" || name != "bazarr" {
		t.Errorf("got host=%q name=%q ok=%v", host, name, ok)
	}
}

func TestParseHostContainerConfig_NoMatch(t *testing.T) {
	_, _, ok := parseHostContainerConfig("modules/containers/bazarr.nix")
	if ok {
		t.Error("should not match container aspect path")
	}
}

func TestParseHostContainerConfig_HostProfile(t *testing.T) {
	_, _, ok := parseHostContainerConfig("modules/hosts/server/host.nix")
	if ok {
		t.Error("should not match host profile path")
	}
}

// --- classifyPath ---

func TestClassifyPath_Container(t *testing.T) {
	ac := makeHostAC("desktop")
	ac.IC = cache.NewInfraCacheWithData(&cache.InfraData{
		Vlans:   map[string]cache.Vlan{"media": {StaticAddresses: map[string]string{"bazarr": "10.10.120.5"}}},
		Clients: map[string]cache.Client{"container-bazarr": {Mac: "62:f5:a4:5b:20:79", Ip: strPtr("10.10.120.5")}},
		Devices: map[string]cache.Device{},
	})
	ac.HC = cache.NewHostsCacheWithData(cache.HostsData{"server": {Containers: map[string]cache.ContainerConf{"bazarr": {}}}})
	ac.Cfg.FlakePath = t.TempDir() // no files → related will be empty, that's fine

	m := classifyPath("modules/containers/bazarr.nix", ac)
	if m.Type != "container" {
		t.Errorf("Type: got %q want container", m.Type)
	}
	if m.Name != "bazarr" {
		t.Errorf("Name: got %q want bazarr", m.Name)
	}
	if m.Entity == nil {
		t.Fatal("Entity nil")
	}
	if m.Entity.IP != "10.10.120.5" {
		t.Errorf("IP: got %q", m.Entity.IP)
	}
	if m.Entity.Host != "server" {
		t.Errorf("Host: got %q want server", m.Entity.Host)
	}
	if m.Entity.Vlan != "media" {
		t.Errorf("Vlan: got %q want media", m.Entity.Vlan)
	}
}

func TestClassifyPath_InfraClient(t *testing.T) {
	ac := makeHostAC("desktop")
	ac.IC = cache.NewInfraCacheWithData(&cache.InfraData{
		Vlans:   map[string]cache.Vlan{"media": {StaticAddresses: map[string]string{"bazarr": "10.10.120.5"}}},
		Clients: map[string]cache.Client{"container-bazarr": {Mac: "62:f5:a4:5b:20:79", Ip: strPtr("10.10.120.5"), Ports: map[string]int{"default": 6767}}},
		Devices: map[string]cache.Device{},
	})
	ac.Cfg.FlakePath = t.TempDir()

	m := classifyPath("modules/infra/clients/container-bazarr.nix", ac)
	if m.Type != "infra-client" {
		t.Errorf("Type: got %q", m.Type)
	}
	if m.Entity == nil || m.Entity.MAC != "62:f5:a4:5b:20:79" {
		t.Errorf("Entity MAC: got %v", m.Entity)
	}
}

func TestClassifyPath_VLAN(t *testing.T) {
	ac := makeHostAC("desktop")
	ac.IC = cache.NewInfraCacheWithData(&cache.InfraData{
		Vlans:   map[string]cache.Vlan{"media": {VlanId: 120, Subnet: "10.10.120.0", Domain: "media.internal"}},
		Clients: map[string]cache.Client{},
		Devices: map[string]cache.Device{},
	})
	ac.Cfg.FlakePath = t.TempDir()

	m := classifyPath("modules/infra/networks/media.nix", ac)
	if m.Type != "vlan" || m.Name != "media" {
		t.Errorf("got type=%q name=%q", m.Type, m.Name)
	}
	if m.Entity == nil || m.Entity.VlanID != 120 {
		t.Errorf("VlanID: got %v", m.Entity)
	}
}

func TestClassifyPath_Device(t *testing.T) {
	ac := makeHostAC("desktop")
	ac.IC = cache.NewInfraCacheWithData(&cache.InfraData{
		Vlans:   map[string]cache.Vlan{},
		Clients: map[string]cache.Client{},
		Devices: map[string]cache.Device{"dream-machine-pro-max": {Mac: "74:fa:29:50:06:e5", Ip: "10.100.10.1"}},
	})
	ac.Cfg.FlakePath = t.TempDir()

	m := classifyPath("modules/infra/devices/dream-machine-pro-max.nix", ac)
	if m.Type != "device" || m.Name != "dream-machine-pro-max" {
		t.Errorf("got type=%q name=%q", m.Type, m.Name)
	}
	if m.Entity == nil || m.Entity.IP != "10.100.10.1" {
		t.Errorf("Entity IP: got %v", m.Entity)
	}
}

func TestClassifyPath_WLAN(t *testing.T) {
	ac := makeHostAC("desktop")
	m := classifyPath("modules/infra/wlans/home.nix", ac)
	if m.Type != "wlan" || m.Name != "home" {
		t.Errorf("got type=%q name=%q", m.Type, m.Name)
	}
	if m.Entity != nil {
		t.Error("WLAN should have no entity")
	}
}

func TestClassifyPath_PortForward(t *testing.T) {
	ac := makeHostAC("desktop")
	m := classifyPath("modules/infra/portforwards/http.nix", ac)
	if m.Type != "port-forward" || m.Name != "http" {
		t.Errorf("got type=%q name=%q", m.Type, m.Name)
	}
}

func TestClassifyPath_HostContainerConfig(t *testing.T) {
	ac := makeHostAC("desktop")
	ac.IC = cache.NewInfraCacheWithData(&cache.InfraData{
		Vlans:   map[string]cache.Vlan{},
		Clients: map[string]cache.Client{},
		Devices: map[string]cache.Device{},
	})
	ac.Cfg.FlakePath = t.TempDir()

	m := classifyPath("modules/hosts/server/containers/bazarr.nix", ac)
	if m.Type != "host-container-config" {
		t.Errorf("Type: got %q", m.Type)
	}
	if m.Name != "bazarr" {
		t.Errorf("Name: got %q", m.Name)
	}
	if m.Category != "server" {
		t.Errorf("Category (host): got %q", m.Category)
	}
}

func TestClassifyPath_HostProfile(t *testing.T) {
	ac := makeHostAC("desktop")
	m := classifyPath("modules/hosts/desktop/host.nix", ac)
	if m.Type != "host-profile" || m.Name != "desktop" {
		t.Errorf("got type=%q name=%q", m.Type, m.Name)
	}
}

func TestClassifyPath_Aspect(t *testing.T) {
	ac := makeHostAC("desktop")
	m := classifyPath("modules/cli/just.nix", ac)
	if m.Type != "aspect" || m.Category != "cli" || m.Name != "just" {
		t.Errorf("got type=%q cat=%q name=%q", m.Type, m.Category, m.Name)
	}
	if m.Entity != nil {
		t.Error("aspect should have no entity")
	}
}

func TestClassifyPath_Package(t *testing.T) {
	ac := makeHostAC("desktop")
	m := classifyPath("packages/sys-mcp/main.go", ac)
	if m.Type != "package" || m.Category != "sys-mcp" {
		t.Errorf("got type=%q cat=%q", m.Type, m.Category)
	}
}

// --- related files ---

func TestClassifyPath_ContainerRelated(t *testing.T) {
	root := makePathFlake(t)
	ac := makeHostAC("desktop")
	ac.IC = cache.NewInfraCacheWithData(&cache.InfraData{
		Vlans:   map[string]cache.Vlan{"media": {StaticAddresses: map[string]string{"bazarr": "10.10.120.5"}}},
		Clients: map[string]cache.Client{"container-bazarr": {Ip: strPtr("10.10.120.5")}},
		Devices: map[string]cache.Device{},
	})
	ac.HC = cache.NewHostsCacheWithData(cache.HostsData{"server": {Containers: map[string]cache.ContainerConf{"bazarr": {}}}})
	ac.Cfg.FlakePath = root

	m := classifyPath("modules/containers/bazarr.nix", ac)

	paths := make(map[string]string)
	for _, r := range m.Related {
		paths[r.Role] = r.Path
	}
	if p, ok := paths["infra client (MAC/IP/ports)"]; !ok || p != "modules/infra/clients/container-bazarr.nix" {
		t.Errorf("infra client related: got %q", p)
	}
	if p, ok := paths["host container config"]; !ok || p != "modules/hosts/server/containers/bazarr.nix" {
		t.Errorf("host config related: got %q", p)
	}
}

func TestClassifyPath_RelatedExcludesSelf(t *testing.T) {
	root := makePathFlake(t)
	ac := makeHostAC("desktop")
	ac.IC = cache.NewInfraCacheWithData(&cache.InfraData{
		Vlans:   map[string]cache.Vlan{},
		Clients: map[string]cache.Client{"container-bazarr": {Ip: strPtr("10.10.120.5")}},
		Devices: map[string]cache.Device{},
	})
	ac.HC = cache.NewHostsCacheWithData(cache.HostsData{"server": {Containers: map[string]cache.ContainerConf{"bazarr": {}}}})
	ac.Cfg.FlakePath = root

	self := "modules/hosts/server/containers/bazarr.nix"
	m := classifyPath(self, ac)
	for _, r := range m.Related {
		if r.Path == self {
			t.Errorf("related includes self: %q", r.Path)
		}
	}
}

// --- cache.VlanForClient (replaced the old ipToVLAN address-only lookup) ---

func TestVlanForClient_Declared(t *testing.T) {
	infra := &cache.InfraData{Vlans: map[string]cache.Vlan{}}
	if got := cache.VlanForClient(cache.Client{Vlan: "trust"}, infra); got != "trust" {
		t.Errorf("got %q want trust", got)
	}
}

func TestVlanForClient_FallsBackToStaticAddress(t *testing.T) {
	infra := &cache.InfraData{
		Vlans: map[string]cache.Vlan{
			"media": {StaticAddresses: map[string]string{"bazarr": "10.10.120.5"}},
		},
	}
	ip := "10.10.120.5"
	if got := cache.VlanForClient(cache.Client{Ip: &ip}, infra); got != "media" {
		t.Errorf("got %q want media", got)
	}
}

func TestVlanForClient_NotFound(t *testing.T) {
	infra := &cache.InfraData{Vlans: map[string]cache.Vlan{}}
	ip := "1.2.3.4"
	if got := cache.VlanForClient(cache.Client{Ip: &ip}, infra); got != "" {
		t.Errorf("got %q want empty", got)
	}
}

// --- resolve_path tool: file content in response ---

func TestResolvePath_FileContentInResponse(t *testing.T) {
	root := makePathFlake(t)
	ac := makeHostAC("desktop")
	ac.IC = cache.NewInfraCacheWithData(&cache.InfraData{
		Vlans:   map[string]cache.Vlan{},
		Clients: map[string]cache.Client{},
		Devices: map[string]cache.Device{},
	})
	ac.Cfg.FlakePath = root

	// Simulate what the tool handler does
	relPath := "modules/cli/just.nix"
	content, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		t.Fatal(err)
	}
	meta := classifyPath(relPath, ac)
	metaJSON, _ := json.Marshal(meta)
	out := strings.Join([]string{string(metaJSON), "", "--- file: " + relPath + " ---", string(content)}, "\n")

	if !strings.Contains(out, `"type":"aspect"`) {
		t.Error("missing type in JSON header")
	}
	if !strings.Contains(out, "--- file: modules/cli/just.nix ---") {
		t.Error("missing file separator")
	}
	if !strings.Contains(out, "# just cli aspect") {
		t.Error("missing file content")
	}
	// JSON must not bleed into file content section
	parts := strings.SplitN(out, "--- file:", 2)
	if strings.Contains(parts[1], `"type"`) {
		t.Error("JSON metadata leaked into file content section")
	}
}

func TestResolvePath_AbsolutePathStripped(t *testing.T) {
	root := makePathFlake(t)
	absPath := filepath.Join(root, "modules/cli/just.nix")
	rel := toRelPath(absPath, root)
	if rel != "modules/cli/just.nix" {
		t.Errorf("got %q", rel)
	}
}

// testAppContext is a placeholder to satisfy the compiler — actual tests use makeHostAC.
type testAppContext struct {
	flakePath string
	ic        *cache.InfraCache
	hc        *cache.HostsCache
}
