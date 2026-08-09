package cache

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const sampleHostsJSON = `{
  "server": {
    "containers": {
      "grafana": {
        "hostBridge":   "br-services",
        "localAddress": "10.10.140.9/24",
        "autoStart":    true,
        "bindMounts": {
          "/var/lib/grafana": {
            "hostPath":   "/var/lib/appdata/grafana",
            "isReadOnly": false
          }
        }
      },
      "loki": {
        "hostBridge":   "br-services",
        "localAddress": "10.10.140.8/24",
        "autoStart":    true,
        "bindMounts":   {}
      }
    },
    "aspects": ["security.sops", "containers.grafana"]
  },
  "desktop": {}
}`

func TestHostsUnmarshal(t *testing.T) {
	var data HostsData
	if err := json.Unmarshal([]byte(sampleHostsJSON), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(data) != 2 {
		t.Errorf("hosts: got %d want 2", len(data))
	}
	if len(data["server"].Containers) != 2 {
		t.Errorf("server containers: got %d want 2", len(data["server"].Containers))
	}
	if len(data["desktop"].Containers) != 0 {
		t.Errorf("desktop containers: got %d want 0", len(data["desktop"].Containers))
	}

	g := data["server"].Containers["grafana"]
	if g.HostBridge != "br-services" {
		t.Errorf("bridge: got %q want br-services", g.HostBridge)
	}
	if g.LocalAddress != "10.10.140.9/24" {
		t.Errorf("localAddress: got %q want 10.10.140.9/24", g.LocalAddress)
	}
	if !g.AutoStart {
		t.Error("autoStart: want true")
	}
	if len(g.BindMounts) != 1 {
		t.Errorf("bindMounts: got %d want 1", len(g.BindMounts))
	}
	m := g.BindMounts["/var/lib/grafana"]
	if m.HostPath != "/var/lib/appdata/grafana" {
		t.Errorf("mount hostPath: got %q", m.HostPath)
	}
	if m.IsReadOnly {
		t.Error("mount isReadOnly: want false")
	}
}

func TestHostsLatestMtime(t *testing.T) {
	dir := t.TempDir()
	subHosts := filepath.Join(dir, "modules", "hosts")
	subConts := filepath.Join(dir, "modules", "containers")
	_ = os.MkdirAll(subHosts, 0755)
	_ = os.MkdirAll(subConts, 0755)

	_ = os.WriteFile(filepath.Join(subHosts, "server.nix"), []byte("a"), 0644)
	time.Sleep(10 * time.Millisecond)
	f2 := filepath.Join(subConts, "grafana.nix")
	_ = os.WriteFile(f2, []byte("b"), 0644)

	c := &HostsCache{flakePath: dir}
	latest, err := c.latestMtime()
	if err != nil {
		t.Fatalf("latestMtime: %v", err)
	}

	f2Info, _ := os.Stat(f2)
	if !latest.Equal(f2Info.ModTime()) {
		t.Errorf("latest mtime should be f2's mtime")
	}
}

func TestHostsLatestMtime_MissingDirs(t *testing.T) {
	// latestMtime should not error when dirs are absent
	c := &HostsCache{flakePath: t.TempDir()}
	_, err := c.latestMtime()
	if err != nil {
		t.Errorf("unexpected error for missing dirs: %v", err)
	}
}

func TestHostsLatestMtime_WalkLogic(t *testing.T) {
	// Inline walk mirrors what latestMtime does — verify the logic is correct.
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.nix")
	f2 := filepath.Join(dir, "b.nix")
	_ = os.WriteFile(f1, []byte("a"), 0644)
	time.Sleep(10 * time.Millisecond)
	_ = os.WriteFile(f2, []byte("b"), 0644)

	var latest time.Time
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, _ := d.Info()
		if t := info.ModTime(); t.After(latest) {
			latest = t
		}
		return nil
	})

	f2Info, _ := os.Stat(f2)
	if !latest.Equal(f2Info.ModTime()) {
		t.Error("walk should return f2 as latest")
	}
}

func TestHostsCacheIntegration(t *testing.T) {
	flake := os.Getenv("FLAKE")
	if flake == "" {
		flake = os.Getenv("NH_FLAKE")
	}
	if flake == "" {
		t.Skip("FLAKE not set, skipping nix eval integration test")
	}
	if _, err := os.Stat(filepath.Join(flake, "modules", "hosts")); err != nil {
		t.Skipf("hosts dir not found under %s", flake)
	}

	c := NewHostsCache(flake)
	data, err := c.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected at least one host")
	}

	// second call should hit cache
	data2, err := c.Get()
	if err != nil {
		t.Fatalf("Get (cached): %v", err)
	}
	if len(data2) != len(data) {
		t.Error("cached result differs from first result")
	}
}
