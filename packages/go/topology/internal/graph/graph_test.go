package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"homelab/topology/internal/infra"
)

// testdata/infra.json is a real `nix eval` dump. Refresh it with:
//
//	nix eval .#nixosConfigurations.server.config.infra --json > testdata/infra.json
func loadInfra(t *testing.T) infra.Infra {
	t.Helper()
	raw, err := os.ReadFile("testdata/infra.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var reg infra.Infra
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return reg
}

// The edge list is the whole point of this tool and the thing that can
// silently drift from sys-mcp's copy of the same join. Assert every cable by
// hand: a wrong diagram should fail here rather than get committed.
func TestPhysicalEdges(t *testing.T) {
	g := BuildPhysical(loadInfra(t))

	want := []string{
		"internet: → dream-machine-pro-max/wan",
		"usw-flex-2-5g-8:1 → macbookpro-work-1/eth0 {work}",
		"usw-flex-2-5g-8:2 → hue-hub/eth0 {iot}",
		"usw-flex-2-5g-8:3 → playstation5/eth0 {game}",
		"usw-flex-2-5g-8:4 → desktop/enp9s0 {trust}",
		"usw-pro-hd-24-poe:1 → u7-pro-xg/uplink [trunk] {iot, trust, work}",
		"usw-pro-hd-24-poe:2 → server/enp9s0 {mgmt}",
		"usw-pro-hd-24-poe:3 → server-ipmi/eth0 {mgmt}",
		"usw-pro-hd-24-poe:9 → usw-flex-2-5g-8/port9 [trunk] {game, iot, trust, work}",
		// One edge for the LAG, carrying its member count.
		"usw-pro-hd-24-poe:25 → server/bond0 [trunk] [lag2] {all but mgmt}",
		"usw-pro-hd-24-poe:27 → dream-machine-pro-max/uplink [trunk] [upstream] {all}",
		"usw-pro-hd-24-poe:28 → unas-pro/eth0 {storage}",
	}

	var got []string
	for _, e := range g.Edges {
		s := fmt.Sprintf("%s:%s → %s/%s", e.From, e.FromPort, e.To, e.ToPort)
		if e.Trunk {
			s += " [trunk]"
		}
		if e.LAG > 0 {
			s += fmt.Sprintf(" [lag%d]", e.LAG)
		}
		if e.Upstream {
			s += " [upstream]"
		}
		// The rendered label says only "trunk", so assert the carried set here
		// instead. It is the fact that matters and the one that silently
		// changes when a VLAN is added.
		switch {
		case e.Trunk && len(e.Excluded) == 0:
			s += " {all}"
		case e.Trunk && len(e.Excluded) < len(e.Carried):
			s += " {all but " + strings.Join(e.Excluded, ", ") + "}"
		case e.Trunk:
			s += " {" + strings.Join(e.Carried, ", ") + "}"
		case e.Network != "":
			s += " {" + e.Network + "}"
		}
		got = append(got, s)
	}

	sort.Strings(want)
	sort.Strings(got)

	if len(got) != len(want) {
		t.Errorf("edge count: got %d want %d", len(got), len(want))
	}
	for i := range max(len(got), len(want)) {
		g, w := "", ""
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if g != w {
			t.Errorf("edge %d:\n  got  %s\n  want %s", i, g, w)
		}
	}
}

// A port with no profile has no peer to pair with. Both cases are real: Pro HD
// 26 is a LAG member, and any unconfigured port has neither.
func TestPortsWithoutProfileHaveNoEdge(t *testing.T) {
	reg := loadInfra(t)
	for _, e := range BuildPhysical(reg).Edges {
		if e.From != "internet" && e.Profile == "" {
			t.Errorf("edge %s:%s has no profile", e.From, e.FromPort)
		}
	}
}

// Wireless clients and container veths have no cable and must not appear.
func TestNoUncabledNodes(t *testing.T) {
	reg := loadInfra(t)
	g := BuildPhysical(reg)
	for _, n := range g.Nodes {
		if c, ok := reg.Clients[n.ID]; ok && c.PortProfile == "" {
			t.Errorf("node %q has no portProfile but was drawn", n.ID)
		}
	}
}

// The Flex leaves via its own port 9, so the edge terminates there rather than
// on an invented second interface for the same socket.
func TestUplinkTerminatesOnOwnPort(t *testing.T) {
	reg := loadInfra(t)
	if got := ifaceOf("usw-flex-2-5g-8", reg); got != "port9" {
		t.Errorf("flex uplink: got %q want port9", got)
	}
	// The AP declares no ports of its own, so it needs the synthetic name.
	if got := ifaceOf("u7-pro-xg", reg); got != "uplink" {
		t.Errorf("ap uplink: got %q want uplink", got)
	}
}

// A client with no cable and no device entry is still named, because the
// physical view calls every plain client's single NIC eth0.
func TestIfaceOfUnknownKey(t *testing.T) {
	if got := ifaceOf("nothing-here", infra.Infra{}); got != "eth0" {
		t.Errorf("unknown key: got %q want eth0", got)
	}
}

// Roles are looked up by id, and anything unlisted is a plain device.
func TestRoleOfNode(t *testing.T) {
	g := BuildPhysical(loadInfra(t))
	if got := g.RoleOfNode("dream-machine-pro-max"); got != RoleRouter {
		t.Errorf("gateway role: got %q want %q", got, RoleRouter)
	}
	if got := g.RoleOfNode("server"); got != RoleHost {
		t.Errorf("server role: got %q want %q", got, RoleHost)
	}
	if got := g.RoleOfNode("not-a-node"); got != RoleDevice {
		t.Errorf("absent node role: got %q want %q", got, RoleDevice)
	}
}

// An empty registry still produces the internet node, and no WAN edge: with no
// gateway there is nothing to attach it to.
func TestBuildPhysicalWithoutGateway(t *testing.T) {
	g := BuildPhysical(infra.Infra{})

	if len(g.Edges) != 0 {
		t.Errorf("edges: got %d want 0", len(g.Edges))
	}
	var ids []string
	for _, n := range g.Nodes {
		ids = append(ids, n.ID)
	}
	// The internet plus one node per machine in hostPorts.
	want := []string{"internet", "desktop", "server"}
	sort.Strings(ids)
	sort.Strings(want)
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Errorf("nodes: got %v want %v", ids, want)
	}
}
