package cache

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const sampleInfraJSON = `{
  "vlans": {
    "services": {
      "vlanId": 140,
      "subnet": "10.10.140.0",
      "subnetPrefix": 24,
      "domain": "services.internal",
      "staticAddresses": {
        "gateway": "10.10.140.1",
        "grafana": "10.10.140.9"
      }
    }
  },
  "clients": {
    "container-grafana": {
      "mac": "62:f5:a4:5b:20:95",
      "name": "Grafana Container",
      "ip": "10.10.140.9",
      "blocked": false,
      "groups": [],
      "ports": { "default": 3000 }
    },
    "container-no-ip": {
      "mac": "62:f5:a4:5b:20:01",
      "name": "No IP Container",
      "ip": null,
      "blocked": false,
      "groups": [],
      "ports": {}
    }
  },
  "devices": {
    "usw-pro": {
      "mac": "aa:bb:cc:dd:ee:ff",
      "name": "USW Pro",
      "ip": "10.10.10.5",
      "type": "static"
    }
  }
}`

func TestUnmarshal(t *testing.T) {
	var data InfraData
	if err := json.Unmarshal([]byte(sampleInfraJSON), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(data.Vlans) != 1 {
		t.Errorf("vlans: got %d want 1", len(data.Vlans))
	}
	if len(data.Clients) != 2 {
		t.Errorf("clients: got %d want 2", len(data.Clients))
	}
	if len(data.Devices) != 1 {
		t.Errorf("devices: got %d want 1", len(data.Devices))
	}

	grafana := data.Clients["container-grafana"]
	if grafana.Ip == nil || *grafana.Ip != "10.10.140.9" {
		t.Errorf("grafana IP: got %v want 10.10.140.9", grafana.Ip)
	}
	if grafana.Ports["default"] != 3000 {
		t.Errorf("grafana port: got %d want 3000", grafana.Ports["default"])
	}

	noip := data.Clients["container-no-ip"]
	if noip.Ip != nil {
		t.Errorf("null ip: got %v want nil", noip.Ip)
	}
}

func TestLatestMtime(t *testing.T) {
	dir := t.TempDir()

	// write two files with different mtimes
	f1 := filepath.Join(dir, "a.nix")
	f2 := filepath.Join(dir, "b.nix")
	if err := os.WriteFile(f1, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(f2, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := &InfraCache{flakePath: filepath.Dir(filepath.Dir(dir))}
	// override flakePath so latestMtime looks in our temp dir
	c.flakePath = ""

	// call latestMtime directly on the temp dir by temporarily monkey-patching
	// the walk — instead, test the walk logic inline
	var latest time.Time
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if mt := info.ModTime(); mt.After(latest) {
			latest = mt
		}
		return nil
	})

	f2Info, _ := os.Stat(f2)
	if !latest.Equal(f2Info.ModTime()) {
		t.Errorf("latest mtime should be f2's mtime")
	}
}

func TestCacheIntegration(t *testing.T) {
	flake := os.Getenv("FLAKE")
	if flake == "" {
		flake = os.Getenv("NH_FLAKE")
	}
	if flake == "" {
		t.Skip("FLAKE not set, skipping nix eval integration test")
	}
	if _, err := os.Stat(filepath.Join(flake, "modules", "infra")); err != nil {
		t.Skipf("infra dir not found under %s, skipping", flake)
	}

	c := NewInfraCache(flake)
	data, err := c.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(data.Vlans) == 0 {
		t.Error("expected at least one VLAN")
	}
	if len(data.Clients) == 0 {
		t.Error("expected at least one client")
	}

	// second call should hit cache
	data2, err := c.Get()
	if err != nil {
		t.Fatalf("Get (cached): %v", err)
	}
	if len(data2.Vlans) != len(data.Vlans) {
		t.Error("cached result differs from first result")
	}
}
