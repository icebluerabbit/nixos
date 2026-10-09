package graph

import (
	"strings"
	"testing"

	"homelab/topology/internal/infra"
)

func ptr(s string) *string { return &s }

// logicalInput is a miniature of what the server dumps: two VLANs the host
// bridges onto, one bridge with no uplink at all, and one whose uplink is not
// a VLAN subinterface.
func logicalInput() infra.Input {
	return infra.Input{
		Infra: infra.Infra{
			Vlans: map[string]infra.Vlan{
				"media":    {VlanID: 120, Subnet: "10.0.120.0", SubnetPrefix: 24},
				"services": {VlanID: 140, Subnet: "10.0.140.0", SubnetPrefix: 24},
			},
			Clients: map[string]infra.Client{
				"server-bridge-media":    {IP: ptr("10.0.120.2"), Vlan: "media", Kind: "virtual"},
				"server-bridge-services": {IP: ptr("10.0.140.2"), Vlan: "services", Kind: "virtual"},
				"nas":                    {IP: ptr("10.0.120.10"), Vlan: "media", Kind: "physical"},
				"tv":                     {IP: ptr("10.0.120.11"), Vlan: "media", Kind: "physical"},
				"veth-jellyfin":          {IP: ptr("10.0.120.12"), Vlan: "media", Kind: "virtual"},
			},
		},
		Bonds: map[string]infra.Bond{
			"bond0": {Interfaces: []string{"enp1s0", "enp2s0"}},
		},
		Bridges: map[string]infra.Bridge{
			"br-media":    {Interfaces: []string{"vlan-media"}},
			"br-services": {Interfaces: []string{"vlan-services"}},
			"br-vpn0":     {Interfaces: nil},
			"br-weird":    {Interfaces: []string{"vlan-nosuchvlan"}},
		},
		Containers: map[string]infra.Container{
			"jellyfin":  {HostBridge: "br-media", LocalAddress: "10.0.120.20/24"},
			"navidrome": {HostBridge: "br-media", LocalAddress: "10.0.120.21/24"},
			"caddy":     {HostBridge: "br-services", LocalAddress: "10.0.140.20/24"},
			// A container with no bridge is not on a VLAN and is skipped.
			"nowhere": {},
		},
	}
}

// The bond is the one uplink everything past the trunk hangs off, so it is
// named even though the logical tables do not print it.
func TestBuildLogicalBond(t *testing.T) {
	l := BuildLogical(logicalInput(), "server")

	if l.Host != "server" {
		t.Errorf("host: got %q want server", l.Host)
	}
	if l.Bond != "bond0" {
		t.Errorf("bond: got %q want bond0", l.Bond)
	}
	if strings.Join(l.NICs, ",") != "enp1s0,enp2s0" {
		t.Errorf("nics: got %v want [enp1s0 enp2s0]", l.NICs)
	}
}

// Busiest VLAN first, name as the tiebreak.
func TestBuildLogicalGroupOrder(t *testing.T) {
	l := BuildLogical(logicalInput(), "server")

	var names []string
	for _, g := range l.Groups {
		names = append(names, g.Name)
	}
	if strings.Join(names, ",") != "media,services" {
		t.Errorf("groups: got %v want [media services]", names)
	}
}

// A group carries the VLAN's identity, the host's own address on it, its
// containers with the prefix stripped, and the real hardware sharing it.
func TestBuildLogicalGroupContents(t *testing.T) {
	l := BuildLogical(logicalInput(), "server")

	media := l.Groups[0]
	if media.ID != 120 || media.Subnet != "10.0.120.0" || media.Prefix != 24 {
		t.Errorf("media identity: got %d %s/%d want 120 10.0.120.0/24",
			media.ID, media.Subnet, media.Prefix)
	}
	if media.Bridge != "br-media" || media.Iface != "vlan-media" {
		t.Errorf("media interfaces: got %s on %s want br-media on vlan-media",
			media.Bridge, media.Iface)
	}
	if media.HostIP != "10.0.120.2" {
		t.Errorf("media host ip: got %q want 10.0.120.2", media.HostIP)
	}

	// Containers are named in sorted order with the CIDR prefix dropped.
	var containers []string
	for _, c := range media.Containers {
		containers = append(containers, c.Name+"="+c.IP)
	}
	want := "jellyfin=10.0.120.20,navidrome=10.0.120.21"
	if got := strings.Join(containers, ","); got != want {
		t.Errorf("media containers: got %q want %q", got, want)
	}

	// Only physical clients: the bridge and the veths are already drawn.
	if got := strings.Join(media.Clients, ","); got != "nas,tv" {
		t.Errorf("media clients: got %q want nas,tv", got)
	}
}

// A bridge with no uplink, and one whose uplink names no VLAN, are both
// counted rather than drawn.
func TestBuildLogicalOrphans(t *testing.T) {
	l := BuildLogical(logicalInput(), "server")

	if got := strings.Join(l.Orphans, ","); got != "br-vpn0,br-weird" {
		t.Errorf("orphans: got %q want br-vpn0,br-weird", got)
	}
}

// A VLAN the host bridges but runs nothing on still gets a row: the bridge
// exists, and omitting it would read as "not configured".
func TestBuildLogicalKeepsEmptyVlan(t *testing.T) {
	in := logicalInput()
	delete(in.Containers, "caddy")

	l := BuildLogical(in, "server")

	if len(l.Groups) != 2 {
		t.Fatalf("groups: got %d want 2", len(l.Groups))
	}
	if l.Groups[1].Name != "services" || len(l.Groups[1].Containers) != 0 {
		t.Errorf("services group: got %q with %d containers, want services with 0",
			l.Groups[1].Name, len(l.Groups[1].Containers))
	}
}

// A bridge whose VLAN has no registered host client leaves the address blank
// rather than inventing one.
func TestBuildLogicalWithoutHostAddress(t *testing.T) {
	in := logicalInput()
	delete(in.Infra.Clients, "server-bridge-media")

	l := BuildLogical(in, "server")

	if l.Groups[0].HostIP != "" {
		t.Errorf("media host ip: got %q want empty", l.Groups[0].HostIP)
	}
}

// An address with no prefix is already bare and must survive untouched.
func TestStripCIDR(t *testing.T) {
	if got := stripCIDR("10.0.120.20/24"); got != "10.0.120.20" {
		t.Errorf("with prefix: got %q want 10.0.120.20", got)
	}
	if got := stripCIDR("10.0.120.20"); got != "10.0.120.20" {
		t.Errorf("without prefix: got %q want 10.0.120.20", got)
	}
}
