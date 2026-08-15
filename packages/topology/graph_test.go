package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// testdata/infra.json is a real `nix eval` dump. Refresh it with:
//
//	nix eval .#nixosConfigurations.server.config.infra --json > testdata/infra.json
func loadInfra(t *testing.T) Infra {
	t.Helper()
	raw, err := os.ReadFile("testdata/infra.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var infra Infra
	if err := json.Unmarshal(raw, &infra); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return infra
}

// The edge list is the whole point of this tool and the thing that can silently
// drift from sys-mcp's copy of the same join. Assert every cable by hand: a
// wrong diagram should fail here rather than get committed.
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
		// instead. It is the fact that matters and the one that silently changes
		// when a VLAN is added.
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
	infra := loadInfra(t)
	for _, e := range BuildPhysical(infra).Edges {
		if e.From != "internet" && e.Profile == "" {
			t.Errorf("edge %s:%s has no profile", e.From, e.FromPort)
		}
	}
}

// Wireless clients and container veths have no cable and must not appear.
func TestNoUncabledNodes(t *testing.T) {
	infra := loadInfra(t)
	g := BuildPhysical(infra)
	for _, n := range g.Nodes {
		if c, ok := infra.Clients[n.ID]; ok && c.PortProfile == "" {
			t.Errorf("node %q has no portProfile but was drawn", n.ID)
		}
	}
}

// The Flex leaves via its own port 9, so the edge terminates there rather than
// on an invented second interface for the same socket.
func TestUplinkTerminatesOnOwnPort(t *testing.T) {
	infra := loadInfra(t)
	if got := ifaceOf("usw-flex-2-5g-8", infra); got != "port9" {
		t.Errorf("flex uplink: got %q want port9", got)
	}
	// The AP declares no ports of its own, so it needs the synthetic name.
	if got := ifaceOf("u7-pro-xg", infra); got != "uplink" {
		t.Errorf("ap uplink: got %q want uplink", got)
	}
}
