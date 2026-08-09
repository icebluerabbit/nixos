package tools

import (
	"strings"
	"testing"

	"sys-mcp/cache"
)

func strPtr(s string) *string { return &s }

var testData = &cache.InfraData{
	Vlans: map[string]cache.Vlan{
		"services": {
			VlanId:       140,
			Subnet:       "10.10.140.0",
			SubnetPrefix: 24,
			Domain:       "services.internal",
			StaticAddresses: map[string]string{
				"gateway": "10.10.140.1",
				"grafana": "10.10.140.9",
				"loki":    "10.10.140.8",
			},
		},
		"media": {
			VlanId:       120,
			Subnet:       "10.10.120.0",
			SubnetPrefix: 24,
			Domain:       "media.internal",
			StaticAddresses: map[string]string{
				"gateway": "10.10.120.1",
				"radarr":  "10.10.120.3",
			},
		},
	},
	Clients: map[string]cache.Client{
		"container-grafana": {
			Mac:  "62:f5:a4:5b:20:95",
			Name: "Grafana Container",
			Ip:   strPtr("10.10.140.9"),
			Ports: map[string]int{
				"default": 3000,
			},
		},
		"container-radarr": {
			Mac:  "62:f5:a4:5b:20:79",
			Name: "Radarr Container",
			Ip:   strPtr("10.10.120.3"),
			Ports: map[string]int{
				"default": 7878,
			},
		},
		"container-no-ip": {
			Mac:   "62:f5:a4:5b:20:01",
			Name:  "No IP Container",
			Ip:    nil,
			Ports: map[string]int{},
		},
	},
	Devices: map[string]cache.Device{
		"usw-pro": {
			Mac:  "aa:bb:cc:dd:ee:ff",
			Name: "USW Pro",
			Ip:   "10.10.10.5",
			Type: "static",
		},
	},
}

// --- clientVlan ---

func TestClientVlan_Found(t *testing.T) {
	c := testData.Clients["container-grafana"]
	got := clientVlan("container-grafana", c, testData)
	if got != "services" {
		t.Errorf("got %q want services", got)
	}
}

func TestClientVlan_NoIP(t *testing.T) {
	c := testData.Clients["container-no-ip"]
	got := clientVlan("container-no-ip", c, testData)
	if got != "" {
		t.Errorf("got %q want empty", got)
	}
}

func TestClientVlan_NotFound(t *testing.T) {
	c := cache.Client{Ip: strPtr("99.99.99.99")}
	got := clientVlan("unknown", c, testData)
	if got != "" {
		t.Errorf("got %q want empty", got)
	}
}

// --- clientSummary ---

func TestClientSummary_WithIP(t *testing.T) {
	c := testData.Clients["container-grafana"]
	s := clientSummary(c)
	if !strings.Contains(s, "10.10.140.9") {
		t.Errorf("missing IP in summary: %s", s)
	}
	if !strings.Contains(s, "62:f5:a4:5b:20:95") {
		t.Errorf("missing MAC in summary: %s", s)
	}
	if !strings.Contains(s, "default=3000") {
		t.Errorf("missing port in summary: %s", s)
	}
}

func TestClientSummary_NoIP(t *testing.T) {
	c := testData.Clients["container-no-ip"]
	s := clientSummary(c)
	if !strings.Contains(s, "<no ip>") {
		t.Errorf("expected <no ip> in summary: %s", s)
	}
}

// --- matchClient ---

func TestMatchClient_Name(t *testing.T) {
	c := testData.Clients["container-grafana"]
	if m := matchClient("container-grafana", c, "grafana"); m == "" {
		t.Error("expected match on name fragment")
	}
}

func TestMatchClient_IP(t *testing.T) {
	c := testData.Clients["container-grafana"]
	if m := matchClient("container-grafana", c, "10.10.140.9"); m == "" {
		t.Error("expected match on exact IP")
	}
	if m := matchClient("container-grafana", c, "140.9"); m == "" {
		t.Error("expected match on IP fragment")
	}
}

func TestMatchClient_MAC(t *testing.T) {
	c := testData.Clients["container-grafana"]
	if m := matchClient("container-grafana", c, "62:f5"); m == "" {
		t.Error("expected match on MAC fragment")
	}
}

func TestMatchClient_Port(t *testing.T) {
	c := testData.Clients["container-grafana"]
	if m := matchClient("container-grafana", c, "3000"); m == "" {
		t.Error("expected match on port number")
	}
}

func TestMatchClient_NoMatch(t *testing.T) {
	c := testData.Clients["container-grafana"]
	if m := matchClient("container-grafana", c, "radarr"); m != "" {
		t.Errorf("unexpected match: %s", m)
	}
}

// --- matchVlan ---

func TestMatchVlan_Name(t *testing.T) {
	v := testData.Vlans["services"]
	if m := matchVlan("services", v, "serv"); m == "" {
		t.Error("expected match on name fragment")
	}
}

func TestMatchVlan_Domain(t *testing.T) {
	v := testData.Vlans["services"]
	if m := matchVlan("services", v, "services.internal"); m == "" {
		t.Error("expected match on domain")
	}
}

func TestMatchVlan_Subnet(t *testing.T) {
	v := testData.Vlans["services"]
	if m := matchVlan("services", v, "10.10.140"); m == "" {
		t.Error("expected match on subnet fragment")
	}
}

func TestMatchVlan_VlanId(t *testing.T) {
	v := testData.Vlans["services"]
	if m := matchVlan("services", v, "140"); m == "" {
		t.Error("expected match on vlanId")
	}
}

func TestMatchVlan_StaticAddress(t *testing.T) {
	v := testData.Vlans["services"]
	if m := matchVlan("services", v, "10.10.140.9"); m == "" {
		t.Error("expected match on static address value")
	}
	if m := matchVlan("services", v, "grafana"); m == "" {
		t.Error("expected match on static address key")
	}
}

// --- matchDevice ---

func TestMatchDevice_Name(t *testing.T) {
	d := testData.Devices["usw-pro"]
	if m := matchDevice("usw-pro", d, "usw"); m == "" {
		t.Error("expected match on name fragment")
	}
}

func TestMatchDevice_IP(t *testing.T) {
	d := testData.Devices["usw-pro"]
	if m := matchDevice("usw-pro", d, "10.10.10.5"); m == "" {
		t.Error("expected match on IP")
	}
}

func TestMatchDevice_MAC(t *testing.T) {
	d := testData.Devices["usw-pro"]
	if m := matchDevice("usw-pro", d, "aa:bb"); m == "" {
		t.Error("expected match on MAC fragment")
	}
}
