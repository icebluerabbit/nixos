package cache

import (
	"os"
	"path/filepath"
	"testing"
)

// makeFullFlakeDir extends makeFlakeDir with wlans, portforwards, and devices dirs/files.
func makeFullFlakeDir(t *testing.T) string {
	t.Helper()
	root := makeFlakeDir(t)
	extras := []string{
		"modules/infra/wlans",
		"modules/infra/portforwards",
		"modules/infra/devices",
	}
	for _, d := range extras {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"modules/infra/wlans/home.nix":                    "{}",
		"modules/infra/wlans/rtorrent-peer.nix":           "{}",
		"modules/infra/portforwards/http.nix":             "{}",
		"modules/infra/portforwards/https.nix":            "{}",
		"modules/infra/devices/dream-machine-pro-max.nix": "{}",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// makeTestIndex returns a SourceIndex with pre-populated reverse maps (no InfraCache needed).
// ic == nil + lastData != nil → ensure() no-ops, maps are trusted as-is.
func makeTestIndex(t *testing.T) (*SourceIndex, string) {
	t.Helper()
	root := makeFullFlakeDir(t)
	idx := &SourceIndex{
		flakePath:    root,
		lastData:     &InfraData{}, // non-nil sentinel — ensures ensure() skips rebuild
		macToClient:  map[string]string{"62:f5:a4:5b:20:95": "container-grafana"},
		macToDevice:  map[string]string{"74:fa:29:50:06:e5": "dream-machine-pro-max"},
		vlanIDToName: map[int]string{140: "services"},
		wlanFiles:    scanInfraDir(root, "modules/infra/wlans"),
		pfFiles:      scanInfraDir(root, "modules/infra/portforwards"),
	}
	return idx, root
}

// --- kebabNorm ---

func TestKebabNorm_Plain(t *testing.T) {
	if got := kebabNorm("home"); got != "home" {
		t.Errorf("got %q want home", got)
	}
}

func TestKebabNorm_Spaces(t *testing.T) {
	if got := kebabNorm("rTorrent Peer"); got != "rtorrent-peer" {
		t.Errorf("got %q want rtorrent-peer", got)
	}
}

func TestKebabNorm_MultiSpace(t *testing.T) {
	if got := kebabNorm("Home  Network"); got != "home-network" {
		t.Errorf("got %q want home-network", got)
	}
}

func TestKebabNorm_LeadingTrailingPunct(t *testing.T) {
	if got := kebabNorm("_foo_"); got != "foo" {
		t.Errorf("got %q want foo", got)
	}
}

func TestKebabNorm_Empty(t *testing.T) {
	if got := kebabNorm(""); got != "" {
		t.Errorf("got %q want empty", got)
	}
}

// --- scanInfraDir ---

func TestScanInfraDir_Found(t *testing.T) {
	root := makeFullFlakeDir(t)
	m := scanInfraDir(root, "modules/infra/wlans")
	if m["home"] != "modules/infra/wlans/home.nix" {
		t.Errorf("home entry: got %q", m["home"])
	}
	if m["rtorrent-peer"] != "modules/infra/wlans/rtorrent-peer.nix" {
		t.Errorf("rtorrent-peer entry: got %q", m["rtorrent-peer"])
	}
}

func TestScanInfraDir_Missing(t *testing.T) {
	m := scanInfraDir(t.TempDir(), "modules/infra/wlans")
	if len(m) != 0 {
		t.Errorf("expected empty map for missing dir, got %v", m)
	}
}

// --- RelIfExists / RelIfExistsDir ---

func TestRelIfExists_File(t *testing.T) {
	root := makeFullFlakeDir(t)
	got := RelIfExists(root, "modules/infra/wlans/home.nix")
	if got != "modules/infra/wlans/home.nix" {
		t.Errorf("got %q", got)
	}
}

func TestRelIfExists_Dir(t *testing.T) {
	root := makeFullFlakeDir(t)
	// dirs should return empty
	if got := RelIfExists(root, "modules/infra/wlans"); got != "" {
		t.Errorf("dir should not match RelIfExists, got %q", got)
	}
}

func TestRelIfExists_Missing(t *testing.T) {
	if got := RelIfExists(t.TempDir(), "nope.nix"); got != "" {
		t.Errorf("got %q want empty", got)
	}
}

func TestRelIfExistsDir_Dir(t *testing.T) {
	root := makeFullFlakeDir(t)
	got := RelIfExistsDir(root, "modules/infra/wlans")
	if got != "modules/infra/wlans" {
		t.Errorf("got %q", got)
	}
}

func TestRelIfExistsDir_File(t *testing.T) {
	root := makeFullFlakeDir(t)
	// files should return empty
	if got := RelIfExistsDir(root, "modules/infra/wlans/home.nix"); got != "" {
		t.Errorf("file should not match RelIfExistsDir, got %q", got)
	}
}

// --- ByClientKey ---

func TestByClientKey_Found(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByClientKey("container-grafana")
	if s.InfraClient == nil || *s.InfraClient != "modules/infra/clients/container-grafana.nix" {
		t.Errorf("InfraClient: got %v", s.InfraClient)
	}
	if s.InfraVlan != nil || s.InfraDevice != nil {
		t.Error("unexpected non-nil fields")
	}
}

func TestByClientKey_Missing(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByClientKey("container-nonexistent")
	if s.InfraClient != nil {
		t.Errorf("expected nil, got %v", *s.InfraClient)
	}
}

func TestByClientKey_Empty(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByClientKey("")
	if s.InfraClient != nil {
		t.Error("empty key should return empty SourcePaths")
	}
}

// --- ByDeviceKey ---

func TestByDeviceKey_Found(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByDeviceKey("dream-machine-pro-max")
	if s.InfraDevice == nil || *s.InfraDevice != "modules/infra/devices/dream-machine-pro-max.nix" {
		t.Errorf("InfraDevice: got %v", s.InfraDevice)
	}
}

func TestByDeviceKey_Missing(t *testing.T) {
	idx, _ := makeTestIndex(t)
	if s := idx.ByDeviceKey("nonexistent"); s.InfraDevice != nil {
		t.Error("expected nil InfraDevice for unknown key")
	}
}

// --- ByVLANName ---

func TestByVLANName_Found(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByVLANName("services")
	if s.InfraVlan == nil || *s.InfraVlan != "modules/infra/networks/services.nix" {
		t.Errorf("InfraVlan: got %v", s.InfraVlan)
	}
}

func TestByVLANName_Missing(t *testing.T) {
	idx, _ := makeTestIndex(t)
	if s := idx.ByVLANName("nonexistent"); s.InfraVlan != nil {
		t.Error("expected nil for unknown VLAN name")
	}
}

func TestByVLANName_Empty(t *testing.T) {
	idx, _ := makeTestIndex(t)
	if s := idx.ByVLANName(""); s.InfraVlan != nil {
		t.Error("empty name should return empty SourcePaths")
	}
}

// --- ByVLANID ---

func TestByVLANID_Found(t *testing.T) {
	idx, _ := makeTestIndex(t)
	id := int64(140)
	s := idx.ByVLANID(&id)
	if s.InfraVlan == nil || *s.InfraVlan != "modules/infra/networks/services.nix" {
		t.Errorf("InfraVlan via ID: got %v", s.InfraVlan)
	}
}

func TestByVLANID_Nil(t *testing.T) {
	idx, _ := makeTestIndex(t)
	if s := idx.ByVLANID(nil); s.InfraVlan != nil {
		t.Error("nil ID should return empty SourcePaths")
	}
}

func TestByVLANID_Unknown(t *testing.T) {
	idx, _ := makeTestIndex(t)
	id := int64(999)
	if s := idx.ByVLANID(&id); s.InfraVlan != nil {
		t.Error("unknown ID should return empty SourcePaths")
	}
}

// --- ByMAC ---

func TestByMAC_Client(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByMAC("62:f5:a4:5b:20:95")
	if s.InfraClient == nil || *s.InfraClient != "modules/infra/clients/container-grafana.nix" {
		t.Errorf("InfraClient via MAC: got %v", s.InfraClient)
	}
	if s.InfraDevice != nil {
		t.Error("InfraDevice should be nil for client MAC")
	}
}

func TestByMAC_ClientCaseInsensitive(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByMAC("62:F5:A4:5B:20:95")
	if s.InfraClient == nil {
		t.Error("MAC lookup should be case-insensitive")
	}
}

func TestByMAC_Device(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByMAC("74:fa:29:50:06:e5")
	if s.InfraDevice == nil || *s.InfraDevice != "modules/infra/devices/dream-machine-pro-max.nix" {
		t.Errorf("InfraDevice via MAC: got %v", s.InfraDevice)
	}
	if s.InfraClient != nil {
		t.Error("InfraClient should be nil for device MAC")
	}
}

func TestByMAC_Unknown(t *testing.T) {
	idx, _ := makeTestIndex(t)
	if s := idx.ByMAC("aa:bb:cc:dd:ee:ff"); s.InfraClient != nil || s.InfraDevice != nil {
		t.Error("unknown MAC should return empty SourcePaths")
	}
}

func TestByMAC_Empty(t *testing.T) {
	idx, _ := makeTestIndex(t)
	if s := idx.ByMAC(""); s.InfraClient != nil {
		t.Error("empty MAC should return empty SourcePaths")
	}
}

// --- ByWLAN ---

func TestByWLAN_Exact(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByWLAN("home")
	if s.InfraWlan == nil || *s.InfraWlan != "modules/infra/wlans/home.nix" {
		t.Errorf("InfraWlan: got %v", s.InfraWlan)
	}
}

func TestByWLAN_KebabNorm(t *testing.T) {
	idx, _ := makeTestIndex(t)
	// "rTorrent Peer" → "rtorrent-peer" → matches rtorrent-peer.nix
	s := idx.ByWLAN("rTorrent Peer")
	if s.InfraWlan == nil || *s.InfraWlan != "modules/infra/wlans/rtorrent-peer.nix" {
		t.Errorf("InfraWlan via kebab norm: got %v", s.InfraWlan)
	}
}

func TestByWLAN_NotFound(t *testing.T) {
	idx, _ := makeTestIndex(t)
	if s := idx.ByWLAN("nonexistent"); s.InfraWlan != nil {
		t.Error("expected nil for unknown WLAN")
	}
}

// --- ByPortForward ---

func TestByPortForward_Found(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByPortForward("http")
	if s.InfraPortForward == nil || *s.InfraPortForward != "modules/infra/portforwards/http.nix" {
		t.Errorf("InfraPortForward: got %v", s.InfraPortForward)
	}
}

func TestByPortForward_NotFound(t *testing.T) {
	idx, _ := makeTestIndex(t)
	if s := idx.ByPortForward("nonexistent"); s.InfraPortForward != nil {
		t.Error("expected nil for unknown port forward")
	}
}

// --- ByContainer ---

func TestByContainer_FileExists(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByContainer("grafana", "server")
	if s.ContainerAspect == nil || *s.ContainerAspect != "modules/containers/grafana.nix" {
		t.Errorf("ContainerAspect: got %v", s.ContainerAspect)
	}
	if s.HostContainerConf == nil || *s.HostContainerConf != "modules/hosts/server/containers/grafana.nix" {
		t.Errorf("HostContainerConf: got %v", s.HostContainerConf)
	}
}

func TestByContainer_DirFallback(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.ByContainer("loki", "server")
	// loki has a dir, not a .nix file (from makeFlakeDir)
	if s.HostContainerConf == nil || *s.HostContainerConf != "modules/hosts/server/containers/loki" {
		t.Errorf("HostContainerConf dir: got %v", s.HostContainerConf)
	}
}

// --- Resolve (full entity) ---

func TestResolve_FullEntity(t *testing.T) {
	idx, _ := makeTestIndex(t)
	s := idx.Resolve("container-grafana", "grafana", "services", "server")
	if s.InfraClient == nil {
		t.Error("InfraClient nil")
	}
	if s.InfraVlan == nil {
		t.Error("InfraVlan nil")
	}
	if s.ContainerAspect == nil {
		t.Error("ContainerAspect nil")
	}
	if s.HostContainerConf == nil {
		t.Error("HostContainerConf nil")
	}
}

func TestResolve_NilSafe(t *testing.T) {
	var idx *SourceIndex
	s := idx.Resolve("x", "y", "z", "w")
	if s.InfraClient != nil || s.InfraVlan != nil {
		t.Error("nil SourceIndex Resolve should return empty")
	}
}
