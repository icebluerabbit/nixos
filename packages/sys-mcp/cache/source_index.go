package cache

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

// SourceIndex maps entity identifiers to their Nix source file paths.
// It is built from InfraCache data and filesystem scans, and stays in sync
// with InfraCache via pointer-equality staleness detection — no independent
// mtime polling needed since InfraCache already watches modules/infra/.
type SourceIndex struct {
	ic        *InfraCache
	flakePath string

	mu           sync.RWMutex
	lastData     *InfraData        // pointer from last build; same ptr = no change
	macToClient  map[string]string // lowercase MAC → infra client key
	macToDevice  map[string]string // lowercase MAC → infra device key
	vlanIDToName map[int]string    // vlan ID → vlan name
	wlanFiles    map[string]string // filename stem → relative path
	pfFiles      map[string]string // filename stem → relative path
}

func NewSourceIndex(flakePath string, ic *InfraCache) *SourceIndex {
	return &SourceIndex{
		ic:        ic,
		flakePath: flakePath,
	}
}

// ensure rebuilds if InfraCache has refreshed since last build.
// Hot path: single pointer comparison under RLock.
func (idx *SourceIndex) ensure() error {
	if idx.ic == nil {
		return nil
	}
	data, err := idx.ic.Get()
	if err != nil {
		return err
	}

	idx.mu.RLock()
	fresh := idx.lastData == data
	idx.mu.RUnlock()
	if fresh {
		return nil
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()
	if idx.lastData == data { // double-check under write lock
		return nil
	}

	macToClient := make(map[string]string, len(data.Clients))
	for name, c := range data.Clients {
		if c.Mac != "" {
			macToClient[strings.ToLower(c.Mac)] = name
		}
	}

	macToDevice := make(map[string]string, len(data.Devices))
	for name, d := range data.Devices {
		if d.Mac != "" {
			macToDevice[strings.ToLower(d.Mac)] = name
		}
	}

	vlanIDToName := make(map[int]string, len(data.Vlans))
	for name, v := range data.Vlans {
		if v.VlanId != 0 {
			vlanIDToName[v.VlanId] = name
		}
	}

	idx.macToClient = macToClient
	idx.macToDevice = macToDevice
	idx.vlanIDToName = vlanIDToName
	idx.wlanFiles = scanInfraDir(idx.flakePath, "modules/infra/wlans")
	idx.pfFiles = scanInfraDir(idx.flakePath, "modules/infra/portforwards")
	idx.lastData = data
	return nil
}

// --- Lookup methods ---
// Methods using only file-existence checks (RelIfExists) need no reverse maps
// and skip ensure(). Methods using reverse maps or dir-scan results call ensure().

// Resolve returns all source paths for a fully-identified entity.
// Used by entity.go to replace the old resolveSources function.
func (idx *SourceIndex) Resolve(clientKey, containerName, vlanName, hostName string) SourcePaths {
	if idx == nil {
		return SourcePaths{}
	}
	s := SourcePaths{}
	if clientKey != "" {
		if p := RelIfExists(idx.flakePath, "modules/infra/clients/"+clientKey+".nix"); p != "" {
			s.InfraClient = &p
		}
	}
	if vlanName != "" {
		if p := RelIfExists(idx.flakePath, "modules/infra/networks/"+vlanName+".nix"); p != "" {
			s.InfraVlan = &p
		}
	}
	if containerName != "" {
		if p := ContainerAspectPath(idx.flakePath, containerName); p != "" {
			s.ContainerAspect = &p
		}
		if hostName != "" {
			base := "modules/hosts/" + hostName + "/containers/" + containerName
			if p := RelIfExistsFileOrDir(idx.flakePath, base); p != "" {
				s.HostContainerConf = &p
			}
		}
	}
	return s
}

func (idx *SourceIndex) ByClientKey(key string) SourcePaths {
	if idx == nil {
		return SourcePaths{}
	}
	s := SourcePaths{}
	if key == "" {
		return s
	}
	if p := RelIfExists(idx.flakePath, "modules/infra/clients/"+key+".nix"); p != "" {
		s.InfraClient = &p
	}
	return s
}

func (idx *SourceIndex) ByDeviceKey(key string) SourcePaths {
	if idx == nil {
		return SourcePaths{}
	}
	s := SourcePaths{}
	if key == "" {
		return s
	}
	if p := RelIfExists(idx.flakePath, "modules/infra/devices/"+key+".nix"); p != "" {
		s.InfraDevice = &p
	}
	return s
}

func (idx *SourceIndex) ByVLANName(name string) SourcePaths {
	if idx == nil {
		return SourcePaths{}
	}
	s := SourcePaths{}
	if name == "" {
		return s
	}
	if p := RelIfExists(idx.flakePath, "modules/infra/networks/"+name+".nix"); p != "" {
		s.InfraVlan = &p
	}
	return s
}

func (idx *SourceIndex) ByVLANID(id *int64) SourcePaths {
	if idx == nil || id == nil {
		return SourcePaths{}
	}
	if err := idx.ensure(); err != nil {
		return SourcePaths{}
	}
	idx.mu.RLock()
	name := idx.vlanIDToName[int(*id)]
	idx.mu.RUnlock()
	return idx.ByVLANName(name)
}

// ByMAC resolves source paths for any entity identified by MAC.
// Returns InfraClient if the MAC belongs to a registered client,
// InfraDevice if it belongs to a registered device, empty if unknown.
func (idx *SourceIndex) ByMAC(mac string) SourcePaths {
	if idx == nil {
		return SourcePaths{}
	}
	s := SourcePaths{}
	if mac == "" {
		return s
	}
	if err := idx.ensure(); err != nil {
		return s
	}
	lower := strings.ToLower(mac)
	idx.mu.RLock()
	clientKey := idx.macToClient[lower]
	deviceKey := idx.macToDevice[lower]
	idx.mu.RUnlock()

	if clientKey != "" {
		if p := RelIfExists(idx.flakePath, "modules/infra/clients/"+clientKey+".nix"); p != "" {
			s.InfraClient = &p
		}
	}
	if deviceKey != "" {
		if p := RelIfExists(idx.flakePath, "modules/infra/devices/"+deviceKey+".nix"); p != "" {
			s.InfraDevice = &p
		}
	}
	return s
}

func (idx *SourceIndex) ByContainer(containerName, hostName string) SourcePaths {
	if idx == nil {
		return SourcePaths{}
	}
	return idx.Resolve("", containerName, "", hostName)
}

func (idx *SourceIndex) ByWLAN(name string) SourcePaths {
	if idx == nil {
		return SourcePaths{}
	}
	s := SourcePaths{}
	if err := idx.ensure(); err != nil {
		return s
	}
	idx.mu.RLock()
	p := idx.wlanFiles[name]
	if p == "" {
		p = idx.wlanFiles[kebabNorm(name)]
	}
	idx.mu.RUnlock()
	if p != "" {
		s.InfraWlan = &p
	}
	return s
}

func (idx *SourceIndex) ByPortForward(name string) SourcePaths {
	if idx == nil {
		return SourcePaths{}
	}
	s := SourcePaths{}
	if err := idx.ensure(); err != nil {
		return s
	}
	idx.mu.RLock()
	p := idx.pfFiles[name]
	if p == "" {
		p = idx.pfFiles[kebabNorm(name)]
	}
	idx.mu.RUnlock()
	if p != "" {
		s.InfraPortForward = &p
	}
	return s
}

// --- package-level helpers (used by entity.go and source_index methods) ---

func RelIfExists(flakePath, rel string) string {
	info, err := os.Stat(filepath.Join(flakePath, rel))
	if err == nil && !info.IsDir() {
		return rel
	}
	return ""
}

func RelIfExistsDir(flakePath, rel string) string {
	info, err := os.Stat(filepath.Join(flakePath, rel))
	if err == nil && info.IsDir() {
		return rel
	}
	return ""
}

// RelIfExistsFileOrDir accepts either form an aspect may take: a single
// `<name>.nix`, or a `<name>/` directory that import-tree walks. Both are
// normal — of the container aspects, 13 are directories — so a check that
// only looks for the file reports a missing source for every one of them.
func RelIfExistsFileOrDir(flakePath, rel string) string {
	if p := RelIfExists(flakePath, rel+".nix"); p != "" {
		return p
	}
	return RelIfExistsDir(flakePath, rel)
}

// ContainerAspectPath resolves a container's aspect source, which may be:
//
//	modules/containers/<name>.nix
//	modules/containers/<name>/
//	modules/containers/<group>/<name>.nix   ← directory named for the grouping
//
// The third form is why this cannot be a plain existence check: `tuwunel` lives
// in `modules/containers/matrix/`, so neither the file nor a same-named
// directory exists for it.
func ContainerAspectPath(flakePath, name string) string {
	base := "modules/containers/" + name
	if p := RelIfExistsFileOrDir(flakePath, base); p != "" {
		return p
	}
	entries, err := os.ReadDir(filepath.Join(flakePath, "modules/containers"))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rel := "modules/containers/" + e.Name() + "/" + name + ".nix"
		if p := RelIfExists(flakePath, rel); p != "" {
			return p
		}
	}
	return ""
}

func scanInfraDir(flakePath, relDir string) map[string]string {
	m := make(map[string]string)
	entries, err := os.ReadDir(filepath.Join(flakePath, relDir))
	if err != nil {
		return m
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".nix") {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), ".nix")
		m[stem] = relDir + "/" + e.Name()
	}
	return m
}

// kebabNorm converts "rTorrent Peer" → "rtorrent-peer"
func kebabNorm(s string) string {
	var b strings.Builder
	prev := '-'
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prev = r
		} else if prev != '-' {
			b.WriteRune('-')
			prev = '-'
		}
	}
	return strings.Trim(b.String(), "-")
}
