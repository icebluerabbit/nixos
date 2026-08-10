package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- shared fixtures ---

var testInfra = &InfraData{
	Vlans: map[string]Vlan{
		"services": {
			VlanId:       140,
			Subnet:       "10.10.140.0",
			SubnetPrefix: 24,
			Domain:       "services.internal",
			StaticAddresses: map[string]string{
				"gateway": "10.10.140.1",
				"grafana": "10.10.140.9",
				"loki":    "10.10.140.8",
				"server":  "10.10.140.2",
			},
		},
		"trust": {
			VlanId:       20,
			Subnet:       "10.10.20.0",
			SubnetPrefix: 24,
			Domain:       "trust.internal",
			StaticAddresses: map[string]string{
				"server": "10.10.20.2",
			},
		},
	},
	Clients: map[string]Client{
		"container-grafana": {
			Mac:     "62:f5:a4:5b:20:95",
			Name:    "Grafana Container",
			Ip:      sp("10.10.140.9"),
			Blocked: false,
			Groups:  []string{},
			Ports:   map[string]int{"default": 3000},
		},
		"container-loki": {
			Mac:     "62:f5:a4:5b:20:94",
			Name:    "Loki Container",
			Ip:      sp("10.10.140.8"),
			Blocked: false,
			Groups:  []string{},
			Ports:   map[string]int{"default": 3100},
		},
		"container-no-ip": {
			Mac:     "62:f5:a4:5b:20:01",
			Name:    "No IP Container",
			Ip:      nil,
			Blocked: false,
			Groups:  []string{},
			Ports:   map[string]int{},
		},
	},
	Devices: map[string]Device{},
}

var testHosts = HostsData{
	"server": {
		Containers: map[string]ContainerConf{
			"grafana": {
				HostBridge:   "br-services",
				LocalAddress: "10.10.140.9/24",
				AutoStart:    true,
				BindMounts: map[string]BindMount{
					"/var/lib/grafana": {HostPath: "/var/lib/appdata/grafana", IsReadOnly: false},
				},
			},
			"loki": {
				HostBridge:   "br-services",
				LocalAddress: "10.10.140.8/24",
				AutoStart:    true,
				BindMounts:   map[string]BindMount{},
			},
		},
	},
	"desktop": {},
}

func sp(s string) *string { return &s }

// makeFlakeDir creates a minimal flake dir with the expected source file layout.
func makeFlakeDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dirs := []string{
		"modules/infra/clients",
		"modules/infra/networks",
		"modules/containers",
		"modules/hosts/server/containers",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// create expected source files
	files := map[string]string{
		"modules/infra/clients/container-grafana.nix": "{}",
		"modules/infra/networks/services.nix":         "{}",
		"modules/containers/grafana.nix":              "{}",
		"modules/hosts/server/containers/grafana.nix": "{}",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// loki uses a directory for its host container config
	lokiDir := filepath.Join(root, "modules/hosts/server/containers/loki")
	if err := os.MkdirAll(lokiDir, 0755); err != nil {
		t.Fatal(err)
	}
	return root
}

// --- stripCIDR ---

func TestStripCIDR_WithPrefix(t *testing.T) {
	if got := stripCIDR("10.10.140.9/24"); got != "10.10.140.9" {
		t.Errorf("got %q want 10.10.140.9", got)
	}
}

func TestStripCIDR_NoPrefix(t *testing.T) {
	if got := stripCIDR("10.10.140.9"); got != "10.10.140.9" {
		t.Errorf("got %q want 10.10.140.9", got)
	}
}

func TestStripCIDR_Empty(t *testing.T) {
	if got := stripCIDR(""); got != "" {
		t.Errorf("got %q want empty", got)
	}
}

// --- hostSSH ---

func TestHostSSH_Found(t *testing.T) {
	got := HostSSH("server", testInfra)
	// server appears in both vlans; either IP is valid
	if got == "" {
		t.Error("expected non-empty SSH target for server")
	}
}

func TestHostSSH_NotFound(t *testing.T) {
	if got := HostSSH("nonexistent", testInfra); got != "" {
		t.Errorf("got %q want empty", got)
	}
}

// --- vlanForClient ---

func TestVlanForClient_Found(t *testing.T) {
	c := testInfra.Clients["container-grafana"]
	if got := vlanForClient(c, testInfra); got != "services" {
		t.Errorf("got %q want services", got)
	}
}

func TestVlanForClient_NoIP(t *testing.T) {
	c := testInfra.Clients["container-no-ip"]
	if got := vlanForClient(c, testInfra); got != "" {
		t.Errorf("got %q want empty", got)
	}
}

// --- containerForIP ---

func TestContainerForIP_Found(t *testing.T) {
	ip := "10.10.140.9"
	name, host := containerForIP(&ip, testHosts)
	if name != "grafana" || host != "server" {
		t.Errorf("got name=%q host=%q want grafana/server", name, host)
	}
}

func TestContainerForIP_NilIP(t *testing.T) {
	name, host := containerForIP(nil, testHosts)
	if name != "" || host != "" {
		t.Errorf("expected empty results for nil IP")
	}
}

func TestContainerForIP_NotFound(t *testing.T) {
	ip := "99.99.99.99"
	name, host := containerForIP(&ip, testHosts)
	if name != "" || host != "" {
		t.Errorf("expected empty results for unknown IP")
	}
}

func TestContainerForIP_NilHosts(t *testing.T) {
	ip := "10.10.140.9"
	name, host := containerForIP(&ip, nil)
	if name != "" || host != "" {
		t.Errorf("expected empty results for nil hosts")
	}
}

// --- buildCommands ---

func TestBuildCommands_Local(t *testing.T) {
	cmds := buildCommands("grafana", "10.10.20.2", true)
	if strings.Contains(cmds.Journal, "ssh") {
		t.Error("local command should not contain ssh prefix")
	}
	if !strings.Contains(cmds.Journal, "grafana") {
		t.Error("journal command missing container name")
	}
	if !strings.Contains(cmds.Status, "container@grafana.service") {
		t.Error("status command missing unit name")
	}
}

func TestBuildCommands_Remote(t *testing.T) {
	cmds := buildCommands("grafana", "10.10.20.2", false)
	if !strings.HasPrefix(cmds.Journal, "ssh 10.10.20.2 ") {
		t.Errorf("remote journal should start with ssh prefix, got %q", cmds.Journal)
	}
	if !strings.HasPrefix(cmds.Status, "ssh 10.10.20.2 ") {
		t.Errorf("remote status should start with ssh prefix, got %q", cmds.Status)
	}
}

func TestBuildCommands_NoSSHTarget(t *testing.T) {
	// remote but no SSH target — should not prepend empty ssh prefix
	cmds := buildCommands("grafana", "", false)
	if strings.HasPrefix(cmds.Journal, "ssh ") {
		t.Error("should not prepend ssh when sshTarget is empty")
	}
}

// --- SourceIndex.Resolve ---

func makeSourceIndex(t *testing.T) (*SourceIndex, string) {
	t.Helper()
	root := makeFlakeDir(t)
	idx := &SourceIndex{flakePath: root}
	return idx, root
}

func TestSourceIndex_Resolve_AllPresent(t *testing.T) {
	idx, _ := makeSourceIndex(t)
	s := idx.Resolve("container-grafana", "grafana", "services", "server")

	if s.InfraClient == nil || *s.InfraClient != "modules/infra/clients/container-grafana.nix" {
		t.Errorf("InfraClient: got %v", s.InfraClient)
	}
	if s.InfraVlan == nil || *s.InfraVlan != "modules/infra/networks/services.nix" {
		t.Errorf("InfraVlan: got %v", s.InfraVlan)
	}
	if s.ContainerAspect == nil || *s.ContainerAspect != "modules/containers/grafana.nix" {
		t.Errorf("ContainerAspect: got %v", s.ContainerAspect)
	}
	if s.HostContainerConf == nil || *s.HostContainerConf != "modules/hosts/server/containers/grafana.nix" {
		t.Errorf("HostContainerConf: got %v", s.HostContainerConf)
	}
}

func TestSourceIndex_Resolve_DirFallback(t *testing.T) {
	idx, _ := makeSourceIndex(t)
	s := idx.Resolve("container-loki", "loki", "services", "server")

	if s.HostContainerConf == nil || *s.HostContainerConf != "modules/hosts/server/containers/loki" {
		t.Errorf("HostContainerConf dir: got %v", s.HostContainerConf)
	}
}

func TestSourceIndex_Resolve_MissingFiles(t *testing.T) {
	idx := &SourceIndex{flakePath: t.TempDir()}
	s := idx.Resolve("container-grafana", "grafana", "services", "server")

	if s.InfraClient != nil {
		t.Errorf("InfraClient should be nil when file absent, got %v", *s.InfraClient)
	}
	if s.ContainerAspect != nil {
		t.Errorf("ContainerAspect should be nil when file absent")
	}
}

func TestSourceIndex_Resolve_EmptyKeys(t *testing.T) {
	idx, _ := makeSourceIndex(t)
	s := idx.Resolve("", "", "", "")

	if s.InfraClient != nil || s.InfraVlan != nil || s.ContainerAspect != nil || s.HostContainerConf != nil {
		t.Error("all fields should be nil when keys are empty")
	}
}

func TestSourceIndex_NilSafe(t *testing.T) {
	var idx *SourceIndex
	if s := idx.Resolve("x", "y", "z", "w"); s.InfraClient != nil {
		t.Error("nil SourceIndex should return empty SourcePaths")
	}
	if s := idx.ByClientKey("x"); s.InfraClient != nil {
		t.Error("nil ByClientKey should return empty")
	}
}

// --- ResolveFromVlan ---

func TestResolveFromVlan_Type(t *testing.T) {
	e := ResolveFromVlan("services", testInfra, (*SourceIndex)(nil))
	if e.Type != "vlan" {
		t.Errorf("type: got %q want vlan", e.Type)
	}
	if e.Vlan == nil {
		t.Fatal("Vlan field nil")
	}
	if e.Vlan.Name != "services" {
		t.Errorf("name: got %q want services", e.Vlan.Name)
	}
}

func TestResolveFromVlan_ClientKeys(t *testing.T) {
	e := ResolveFromVlan("services", testInfra, (*SourceIndex)(nil))
	// grafana and loki have IPs in services staticAddresses
	keys := make(map[string]bool, len(e.Vlan.ClientKeys))
	for _, k := range e.Vlan.ClientKeys {
		keys[k] = true
	}
	if !keys["container-grafana"] {
		t.Error("ClientKeys should include container-grafana")
	}
	if !keys["container-loki"] {
		t.Error("ClientKeys should include container-loki")
	}
	// container-no-ip should not appear (no IP)
	if keys["container-no-ip"] {
		t.Error("ClientKeys should not include container-no-ip")
	}
}

func TestResolveFromVlan_NotFound(t *testing.T) {
	e := ResolveFromVlan("nonexistent", testInfra, (*SourceIndex)(nil))
	if e.Vlan != nil {
		t.Error("Vlan should be nil for unknown vlan")
	}
}

func TestResolveFromVlan_NilContainerAndHost(t *testing.T) {
	e := ResolveFromVlan("services", testInfra, (*SourceIndex)(nil))
	if e.Client != nil || e.Container != nil || e.Host != nil {
		t.Error("VLAN entity should not set client/container/host")
	}
}

// --- ResolveFromClient ---

func TestResolveFromClient_FullResolution(t *testing.T) {
	e := ResolveFromClient("container-grafana", testInfra, testHosts, (*SourceIndex)(nil), "desktop")
	if e.Type != "client" {
		t.Errorf("type: got %q want client", e.Type)
	}
	if e.Client == nil {
		t.Fatal("Client nil")
	}
	if e.Client.Name != "container-grafana" {
		t.Errorf("client name: got %q", e.Client.Name)
	}
	if e.Vlan == nil || e.Vlan.Name != "services" {
		t.Error("VLAN not resolved")
	}
	if e.Container == nil || e.Container.Name != "grafana" {
		t.Error("Container not resolved")
	}
	if e.Host == nil || e.Host.Name != "server" {
		t.Error("Host not resolved")
	}
	// SshTarget is the hostname, not an IP — ssh config resolves the rest
	if e.Host.SshTarget != "server" {
		t.Errorf("SshTarget: got %q want server", e.Host.SshTarget)
	}
}

func TestResolveFromClient_IsLocal(t *testing.T) {
	e := ResolveFromClient("container-grafana", testInfra, testHosts, (*SourceIndex)(nil), "server")
	if !e.Host.IsLocal {
		t.Error("should be local when hostname matches")
	}
}

func TestResolveFromClient_NotLocal(t *testing.T) {
	e := ResolveFromClient("container-grafana", testInfra, testHosts, (*SourceIndex)(nil), "desktop")
	if e.Host.IsLocal {
		t.Error("should not be local when hostname differs")
	}
}

func TestResolveFromClient_NoIP(t *testing.T) {
	e := ResolveFromClient("container-no-ip", testInfra, testHosts, (*SourceIndex)(nil), "desktop")
	if e.Client == nil {
		t.Fatal("Client nil")
	}
	if e.Vlan != nil {
		t.Error("VLAN should be nil for no-IP client")
	}
	if e.Container != nil || e.Host != nil {
		t.Error("Container/Host should be nil for no-IP client")
	}
}

func TestResolveFromClient_NotFound(t *testing.T) {
	e := ResolveFromClient("nonexistent", testInfra, testHosts, (*SourceIndex)(nil), "desktop")
	if e.Client != nil {
		t.Error("Client should be nil for unknown key")
	}
}

func TestResolveFromClient_NilHosts(t *testing.T) {
	e := ResolveFromClient("container-grafana", testInfra, nil, (*SourceIndex)(nil), "desktop")
	if e.Client == nil {
		t.Fatal("Client nil")
	}
	// VLAN still resolves, container/host do not
	if e.Vlan == nil {
		t.Error("VLAN should still resolve without hosts data")
	}
	if e.Container != nil || e.Host != nil {
		t.Error("Container/Host should be nil without hosts data")
	}
}

// --- ResolveFromContainer ---

func TestResolveFromContainer_FullResolution(t *testing.T) {
	e := ResolveFromContainer("grafana", "server", testInfra, testHosts, (*SourceIndex)(nil), "desktop")
	if e.Type != "container" {
		t.Errorf("type: got %q want container", e.Type)
	}
	if e.Container == nil || e.Container.Name != "grafana" {
		t.Fatal("Container not set")
	}
	if e.Container.Bridge != "br-services" {
		t.Errorf("bridge: got %q want br-services", e.Container.Bridge)
	}
	if e.Client == nil || e.Client.Name != "container-grafana" {
		t.Error("Client not resolved from localAddress")
	}
	if e.Vlan == nil || e.Vlan.Name != "services" {
		t.Error("VLAN not resolved")
	}
	if e.Host == nil || e.Host.Name != "server" {
		t.Error("Host not set")
	}
}

func TestResolveFromContainer_Commands(t *testing.T) {
	e := ResolveFromContainer("grafana", "server", testInfra, testHosts, (*SourceIndex)(nil), "desktop")
	cmds := e.Container.Commands
	if !strings.HasPrefix(cmds.Journal, "ssh ") {
		t.Errorf("remote journal should have ssh prefix: %q", cmds.Journal)
	}
	if !strings.Contains(cmds.Journal, "grafana") {
		t.Error("journal command missing container name")
	}
	if !strings.Contains(cmds.Exec, "grafana") {
		t.Error("exec command missing container name")
	}
}

func TestResolveFromContainer_BindMounts(t *testing.T) {
	e := ResolveFromContainer("grafana", "server", testInfra, testHosts, (*SourceIndex)(nil), "desktop")
	m, ok := e.Container.BindMounts["/var/lib/grafana"]
	if !ok {
		t.Fatal("bind mount /var/lib/grafana not found")
	}
	if m.HostPath != "/var/lib/appdata/grafana" {
		t.Errorf("hostPath: got %q", m.HostPath)
	}
}

func TestResolveFromContainer_NotFound(t *testing.T) {
	e := ResolveFromContainer("nonexistent", "server", testInfra, testHosts, (*SourceIndex)(nil), "desktop")
	if e.Container != nil {
		t.Error("Container should be nil for unknown name")
	}
}

func TestResolveFromContainer_UnknownHost(t *testing.T) {
	e := ResolveFromContainer("grafana", "nonexistent", testInfra, testHosts, (*SourceIndex)(nil), "desktop")
	if e.Container != nil {
		t.Error("Container should be nil for unknown host")
	}
}

// --- ResolveFromHost ---

func TestResolveFromHost_ContainerNames(t *testing.T) {
	e := ResolveFromHost("server", testInfra, testHosts, "desktop")
	if e.Type != "host" {
		t.Errorf("type: got %q want host", e.Type)
	}
	if e.Host == nil {
		t.Fatal("Host nil")
	}
	names := make(map[string]bool, len(e.Host.ContainerNames))
	for _, n := range e.Host.ContainerNames {
		names[n] = true
	}
	if !names["grafana"] || !names["loki"] {
		t.Errorf("ContainerNames: got %v, want grafana and loki", e.Host.ContainerNames)
	}
}

func TestResolveFromHost_IsLocal(t *testing.T) {
	e := ResolveFromHost("server", testInfra, testHosts, "server")
	if !e.Host.IsLocal {
		t.Error("should be local")
	}
}

func TestResolveFromHost_SshTarget(t *testing.T) {
	e := ResolveFromHost("server", testInfra, testHosts, "desktop")
	if e.Host.SshTarget != "server" {
		t.Errorf("SshTarget: got %q want server", e.Host.SshTarget)
	}
}

func TestResolveFromHost_Desktop_NoContainers(t *testing.T) {
	e := ResolveFromHost("desktop", testInfra, testHosts, "desktop")
	if e.Host == nil {
		t.Fatal("Host nil")
	}
	if len(e.Host.ContainerNames) != 0 {
		t.Errorf("desktop should have no containers, got %v", e.Host.ContainerNames)
	}
}

func TestResolveFromHost_NotFound(t *testing.T) {
	e := ResolveFromHost("nonexistent", testInfra, testHosts, "desktop")
	if e.Host != nil {
		t.Error("Host should be nil for unknown host")
	}
}

// --- summarizeAspects ---

func TestSummarizeAspects_FilterAndTruncate(t *testing.T) {
	input := []string{
		"cli.bat",
		"cli.neovim.neovim",
		"cli.neovim.plugins.blink-cmp",
		"cli.neovim.plugins.lualine",
		"hw.nvidia-rtx-4090",
		"infra.clients.container-grafana",
		"containers.grafana",
	}
	got := summarizeAspects(input)

	// infra.* and containers.* must be absent
	if _, ok := got["infra"]; ok {
		t.Error("infra group should be filtered out")
	}
	if _, ok := got["containers"]; ok {
		t.Error("containers group should be filtered out")
	}

	// cli.bat is a leaf at depth 2 — plain label
	cli := got["cli"]
	batFound, neovimFound := false, false
	for _, l := range cli {
		if l == "bat" {
			batFound = true
		}
		if strings.HasPrefix(l, "neovim (") {
			neovimFound = true
		}
	}
	if !batFound {
		t.Errorf("cli.bat should appear as plain 'bat', got %v", cli)
	}
	if !neovimFound {
		t.Errorf("cli.neovim.* should appear as 'neovim (N)', got %v", cli)
	}

	// hw.nvidia-rtx-4090 is a single leaf
	hw := got["hw"]
	if len(hw) != 1 || hw[0] != "nvidia-rtx-4090" {
		t.Errorf("hw: got %v want [nvidia-rtx-4090]", hw)
	}
}

func TestSummarizeAspects_Empty(t *testing.T) {
	if got := summarizeAspects(nil); got != nil {
		t.Errorf("nil input should return nil, got %v", got)
	}
	if got := summarizeAspects([]string{}); got != nil {
		t.Errorf("empty input should return nil, got %v", got)
	}
}

func TestSummarizeAspects_AllFiltered(t *testing.T) {
	input := []string{"infra.clients.foo", "containers.bar"}
	if got := summarizeAspects(input); got != nil {
		t.Errorf("all-filtered input should return nil, got %v", got)
	}
}

// --- ipToClientKey ---

func TestIPToClientKey(t *testing.T) {
	m := ipToClientKey(testInfra)
	if m["10.10.140.9"] != "container-grafana" {
		t.Errorf("got %q want container-grafana", m["10.10.140.9"])
	}
	if _, ok := m["container-no-ip"]; ok {
		t.Error("nil IP client should not appear in map")
	}
}
