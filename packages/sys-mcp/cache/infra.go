package cache

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"io/fs"
	"os"
)

type Vlan struct {
	VlanId          int               `json:"vlanId"`
	Subnet          string            `json:"subnet"`
	SubnetPrefix    int               `json:"subnetPrefix"`
	Domain          string            `json:"domain"`
	StaticAddresses map[string]string `json:"staticAddresses"`
	DhcpStart       string            `json:"dhcpStart"`
	DhcpStop        string            `json:"dhcpStop"`
}

type Client struct {
	Mac     string         `json:"mac"`
	Name    string         `json:"name"`
	Ip      *string        `json:"ip"`
	Blocked bool           `json:"blocked"`
	Groups  []string       `json:"groups"`
	Ports   map[string]int `json:"ports"`
	Vlan    string         `json:"vlan"`
	Kind    string         `json:"kind"`
	// PortProfile names the infra.ports key on the switch port this client is
	// wired to. Empty for anything with no cable: wireless clients, container
	// veths, bridges.
	PortProfile string `json:"portProfile"`
}

type Device struct {
	Mac  string `json:"mac"`
	Name string `json:"name"`
	Ip   string `json:"ip"`
	Type string `json:"type"`
	// PortProfile names the infra.ports key on the *upstream* switch port this
	// device plugs into — not a profile on its own ports.
	PortProfile string `json:"portProfile"`
}

// Port is a UniFi port profile. It doubles as the link identity: the switch
// names it in a port_override and whatever is plugged in names it in
// PortProfile, so the pair identifies a cable.
type Port struct {
	Name             string   `json:"name"`
	Network          string   `json:"network"`
	UnifiId          string   `json:"unifiId"`
	ResourceName     string   `json:"resourceName"`
	Trunk            bool     `json:"trunk"`
	ExcludedNetworks []string `json:"excludedNetworks"`
}

type InfraData struct {
	Vlans   map[string]Vlan   `json:"vlans"`
	Clients map[string]Client `json:"clients"`
	Devices map[string]Device `json:"devices"`
	Ports   map[string]Port   `json:"ports"`
}

type InfraCache struct {
	mu        sync.RWMutex
	data      *InfraData
	lastMtime time.Time
	flakePath string
}

func NewInfraCache(flakePath string) *InfraCache {
	return &InfraCache{flakePath: flakePath}
}

// NewInfraCacheWithData returns a pre-loaded cache for use in tests.
func NewInfraCacheWithData(data *InfraData) *InfraCache {
	c := &InfraCache{}
	c.data = data
	return c
}

// Prewarm kicks off an async eval at startup so the first tool call doesn't block.
func (c *InfraCache) Prewarm() {
	go func() { _, _ = c.refresh() }()
}

func (c *InfraCache) Get() (*InfraData, error) {
	if c.flakePath == "" {
		c.mu.RLock()
		data := c.data
		c.mu.RUnlock()
		if data != nil {
			return data, nil
		}
		return nil, fmt.Errorf("no data and no flakePath")
	}
	mtime, err := c.latestMtime()
	if err != nil {
		return nil, fmt.Errorf("mtime check: %w", err)
	}

	c.mu.RLock()
	if c.data != nil && !mtime.After(c.lastMtime) {
		data := c.data
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	return c.refresh()
}

func (c *InfraCache) refresh() (*InfraData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	out, err := exec.Command(
		"nix", "eval",
		c.flakePath+"#nixosConfigurations.server.config.infra",
		"--json",
		"--no-write-lock-file",
	).Output()
	if err != nil {
		if c.data != nil {
			return c.data, nil
		}
		return nil, fmt.Errorf("nix eval failed: %w", err)
	}

	var data InfraData
	if err := json.Unmarshal(out, &data); err != nil {
		if c.data != nil {
			return c.data, nil
		}
		return nil, fmt.Errorf("parse infra json: %w", err)
	}

	mtime, _ := c.latestMtime()
	c.data = &data
	c.lastMtime = mtime
	return c.data, nil
}

func (c *InfraCache) latestMtime() (time.Time, error) {
	var latest time.Time
	dir := filepath.Join(c.flakePath, "modules", "infra")
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if t := info.ModTime(); t.After(latest) {
			latest = t
		}
		return nil
	})
	if err != nil && os.IsNotExist(err) {
		return time.Time{}, fmt.Errorf("infra dir not found: %s", dir)
	}
	return latest, err
}
