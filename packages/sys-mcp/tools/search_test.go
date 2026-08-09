package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

// testData and strPtr are declared in infra_test.go (same package).

// --- searchResult JSON ---

func TestSearchResult_JSON(t *testing.T) {
	r := searchResult{
		Type:      "client",
		Name:      "container-grafana",
		Summary:   "IP: 10.10.140.9 | MAC: 62:f5:a4:5b:20:95 | ports: default=3000",
		MatchedOn: "name:container-grafana",
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, want := range []string{"type", "name", "summary", "matched_on"} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing field %q: %s", want, s)
		}
	}
}

// --- matchVlan (all fields) ---

func TestMatchVlan_NoMatch(t *testing.T) {
	v := testData.Vlans["services"]
	if m := matchVlan("services", v, "zzznomatch"); m != "" {
		t.Errorf("unexpected match: %s", m)
	}
}

// --- get_by_ip logic: device path ---

func TestGetByIP_DevicePath(t *testing.T) {
	ip := "10.10.10.5"
	d := testData.Devices["usw-pro"]
	if d.Ip != ip {
		t.Skipf("fixture IP changed")
	}
	// client lookup should miss
	for _, c := range testData.Clients {
		if c.Ip != nil && *c.Ip == ip {
			t.Error("IP should not match a client")
		}
	}
	// device lookup should hit
	found := false
	for _, dev := range testData.Devices {
		if dev.Ip == ip {
			found = true
		}
	}
	if !found {
		t.Error("device not found by IP")
	}
}

// --- get_by_ip logic: vlan-address path ---

func TestGetByIP_VlanAddressPath(t *testing.T) {
	// gateway IPs are in staticAddresses but not in any client
	ip := "10.10.140.1"

	for _, c := range testData.Clients {
		if c.Ip != nil && *c.Ip == ip {
			t.Fatal("gateway IP should not be a client IP in test data")
		}
	}
	for _, d := range testData.Devices {
		if d.Ip == ip {
			t.Fatal("gateway IP should not be a device IP in test data")
		}
	}

	found := false
	for _, v := range testData.Vlans {
		for addrName, addr := range v.StaticAddresses {
			if addr == ip {
				found = true
				if addrName != "gateway" {
					t.Errorf("expected addrName=gateway, got %q", addrName)
				}
			}
		}
	}
	if !found {
		t.Error("gateway IP not found in any vlan staticAddresses")
	}
}

// --- get_by_port logic ---

func TestGetByPort_Found(t *testing.T) {
	port := 3000
	var matches []string
	for name, c := range testData.Clients {
		for _, p := range c.Ports {
			if p == port {
				matches = append(matches, name)
			}
		}
	}
	if len(matches) == 0 {
		t.Error("expected at least one client with port 3000")
	}
	found := false
	for _, m := range matches {
		if m == "container-grafana" {
			found = true
		}
	}
	if !found {
		t.Error("container-grafana should match port 3000")
	}
}

func TestGetByPort_NotFound(t *testing.T) {
	port := 9999
	for _, c := range testData.Clients {
		for _, p := range c.Ports {
			if p == port {
				t.Errorf("unexpected match for port %d", port)
			}
		}
	}
}

// --- search aggregation: queries produce at least one match of the expected type ---

func TestMatchAll_ProduceSummary(t *testing.T) {
	queries := []struct {
		q    string
		want string // at least one match of this type expected
	}{
		{"container-grafana", "client"}, // exact client key
		{"services", "vlan"},            // exact vlan name
		{"usw", "device"},               // device name fragment
		{"3000", "client"},              // port number
	}
	for _, tc := range queries {
		t.Run(tc.q, func(t *testing.T) {
			types := map[string]bool{}
			for name, c := range testData.Clients {
				if matchClient(name, c, tc.q) != "" {
					types["client"] = true
				}
			}
			for name, v := range testData.Vlans {
				if matchVlan(name, v, tc.q) != "" {
					types["vlan"] = true
				}
			}
			for name, d := range testData.Devices {
				if matchDevice(name, d, tc.q) != "" {
					types["device"] = true
				}
			}
			if len(types) == 0 {
				t.Errorf("query %q produced no matches", tc.q)
			}
			if !types[tc.want] {
				t.Errorf("query %q: want match of type %q, got types %v", tc.q, tc.want, types)
			}
		})
	}
}

func TestMatchAll_NoMatch(t *testing.T) {
	q := "zzznomatchxxx"
	for name, c := range testData.Clients {
		if m := matchClient(name, c, q); m != "" {
			t.Errorf("unexpected client match: %s", m)
		}
	}
	for name, v := range testData.Vlans {
		if m := matchVlan(name, v, q); m != "" {
			t.Errorf("unexpected vlan match: %s", m)
		}
	}
	for name, d := range testData.Devices {
		if m := matchDevice(name, d, q); m != "" {
			t.Errorf("unexpected device match: %s", m)
		}
	}
}
