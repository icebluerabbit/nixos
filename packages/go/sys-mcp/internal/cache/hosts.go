package cache

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ContainerConf is one container as its host declares it.
type ContainerConf struct {
	HostBridge   string               `json:"hostBridge"`
	LocalAddress string               `json:"localAddress"`
	AutoStart    bool                 `json:"autoStart"`
	BindMounts   map[string]BindMount `json:"bindMounts"`
}

// BindMount is one host path a container sees.
type BindMount struct {
	HostPath   string `json:"hostPath"`
	IsReadOnly bool   `json:"isReadOnly"`
}

// HostEval is what one host's configuration yields: its containers and the
// den aspects it composes.
type HostEval struct {
	Containers map[string]ContainerConf `json:"containers"`
	Aspects    []string                 `json:"aspects"`
}

// HostsData is every host in the flake, keyed by hostname: its containers and
// its active aspects.
type HostsData map[string]HostEval

// HostsCache holds the evaluated host configurations, refreshed when the
// flake's files change.
type HostsCache struct {
	mu        sync.RWMutex
	data      HostsData
	lastMtime time.Time
	flakePath string
}

// NewHostsCache returns a cache that evaluates the flake at flakePath.
func NewHostsCache(flakePath string) *HostsCache {
	return &HostsCache{flakePath: flakePath}
}

// NewHostsCacheWithData returns a pre-loaded cache for use in tests.
func NewHostsCacheWithData(data HostsData) *HostsCache {
	c := &HostsCache{}
	c.data = data
	return c
}

// Prewarm starts the first evaluation in the background, so the first tool
// call does not pay for it.
func (c *HostsCache) Prewarm() {
	go func() { _, _ = c.refresh() }()
}

// Get returns the host data, re-evaluating when the flake has changed.
func (c *HostsCache) Get() (HostsData, error) {
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

// hostsApply extracts containers and active aspects per host.
const hostsApply = `cfgs: builtins.mapAttrs (host: cfg: {
  containers = builtins.mapAttrs (name: c: {
    hostBridge    = if c.hostBridge   != null then c.hostBridge   else "";
    localAddress  = if c.localAddress != null then c.localAddress else "";
    autoStart     = c.autoStart;
    bindMounts    = builtins.mapAttrs (_: m: { inherit (m) hostPath isReadOnly; }) c.bindMounts;
  }) cfg.config.containers;
  aspects = cfg.config.aspects or [];
}) cfgs`

func (c *HostsCache) refresh() (HostsData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	//nolint:gosec // the flake path is this tool's own configuration
	out, err := exec.Command(
		"nix", "eval",
		c.flakePath+"#nixosConfigurations",
		"--apply", hostsApply,
		"--json",
		"--no-write-lock-file",
	).Output()
	if err != nil {
		if c.data != nil {
			return c.data, nil
		}
		return nil, fmt.Errorf("nix eval failed: %w", err)
	}

	var data HostsData
	if err := json.Unmarshal(out, &data); err != nil {
		if c.data != nil {
			return c.data, nil
		}
		return nil, fmt.Errorf("parse hosts json: %w", err)
	}

	mtime, _ := c.latestMtime()
	c.data = data
	c.lastMtime = mtime
	return c.data, nil
}

func (c *HostsCache) latestMtime() (time.Time, error) {
	var latest time.Time
	for _, sub := range [][]string{{"modules", "hosts"}, {"modules", "containers"}} {
		dir := filepath.Join(append([]string{c.flakePath}, sub...)...) //nolint:gosec // rooted at the configured flake
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
		if err != nil && !os.IsNotExist(err) {
			return time.Time{}, err
		}
	}
	return latest, nil
}
