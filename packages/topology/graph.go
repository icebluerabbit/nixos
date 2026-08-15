package main

import (
	"sort"
	"strconv"
)

// Role is what a node is, which decides its shape and colour. It is not
// derivable from the registry: `infra.devices.*.type` is the DHCP/static
// setting, not a device class.
type Role string

const (
	RoleInternet    Role = "internet"
	RoleRouter      Role = "router"
	RoleSwitch      Role = "switch"
	RoleAccessPoint Role = "accessPoint"
	RoleHost        Role = "host"
	RoleDevice      Role = "device"
)

var roles = map[string]Role{
	"dream-machine-pro-max": RoleRouter,
	"usw-pro-hd-24-poe":     RoleSwitch,
	"usw-flex-2-5g-8":       RoleSwitch,
	"u7-pro-xg":             RoleAccessPoint,
}

// hostPort maps an infra client to the machine it belongs to. A machine reached
// over several cables appears once, with one interface per cable.
type hostPort struct {
	Node      string
	Interface string
}

// NIC names live in each host's own config, which this tool does not read, and
// several infra clients are one physical machine. Both facts have to be stated.
var hostPorts = map[string]hostPort{
	"server":       {Node: "server", Interface: "enp9s0"},
	"server-trunk": {Node: "server", Interface: "bond0"},
	// modules/hosts/desktop/networking.nix, which sets this interface's MAC
	// from infra.clients.desktop.
	"desktop": {Node: "desktop", Interface: "enp9s0"},
}

// Node is one box in a diagram.
type Node struct {
	ID   string
	Name string
	Role Role
	IP   string
}

// Edge is one cable.
type Edge struct {
	From      string
	FromIndex int
	// FromPort is the switch port index as text, empty for the WAN edge.
	FromPort string
	To       string
	// ToPort is the far-side interface name.
	ToPort string
	// Profile is the infra.ports key that pairs the two ends.
	Profile string
	Label   string
	// Network is the VLAN this port lands on untagged. On a trunk that is
	// usually "default" and says little; Carried is the interesting part.
	Network string
	// Carried lists the VLANs a trunk passes: every network minus the
	// profile's exclusions. Empty on an access port, where Network is the
	// whole story.
	Carried []string
	// Excluded is what the trunk deliberately drops.
	Excluded []string
	Trunk    bool
	// LAG is the member port count, 0 when the link is a single cable.
	LAG int
	// Upstream marks an edge whose far end sits above the switch that owns the
	// port. Renderers that care about direction flip it.
	Upstream bool
}

// Graph is the physical layer: what is plugged into what.
type Graph struct {
	Nodes []Node
	Edges []Edge
	// VlanIDs is every VLAN an access link in this graph lands on, used to
	// colour links and build the legend.
	VlanIDs map[string]int
}

func roleOf(key string) Role {
	if r, ok := roles[key]; ok {
		return r
	}
	return RoleDevice
}

func nodeOf(key string) string {
	if hp, ok := hostPorts[key]; ok {
		return hp.Node
	}
	return key
}

// ownUplinkPort returns the index of the device's own port that carries its
// uplink profile — the Flex reaches the Pro HD out of its port 9 and says so.
// Empty when the device declares no ports, which is when a synthetic name is
// needed instead.
func ownUplinkPort(d Device) string {
	if d.PortProfile == "" {
		return ""
	}
	for _, index := range sortedPortIndices(d.Ports) {
		if d.Ports[index].Profile == d.PortProfile {
			return index
		}
	}
	return ""
}

// ifaceOf names the far side of a cable: a host's real NIC, a device's own
// uplink port, or the single unnamed NIC of a plain client.
func ifaceOf(key string, infra Infra) string {
	if hp, ok := hostPorts[key]; ok {
		return hp.Interface
	}
	if d, ok := infra.Devices[key]; ok {
		if own := ownUplinkPort(d); own != "" {
			return "port" + own
		}
		return "uplink"
	}
	return "eth0"
}

// peersOf lists everything declaring a profile. One of them is the switch
// reading it, so callers drop themselves.
func peersOf(profile string, infra Infra) []string {
	var peers []string
	for key, c := range infra.Clients {
		if c.PortProfile == profile {
			peers = append(peers, key)
		}
	}
	for key, d := range infra.Devices {
		if d.PortProfile == profile {
			peers = append(peers, key)
		}
	}
	sort.Strings(peers)
	return peers
}

func sortedPortIndices(ports map[string]DevicePort) []string {
	out := make([]string, 0, len(ports))
	for index := range ports {
		out = append(out, index)
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := strconv.Atoi(out[i])
		b, _ := strconv.Atoi(out[j])
		return a < b
	})
	return out
}

// carriedVLANs is what a trunk actually passes. The controller model is a
// denylist — every network is tagged unless excluded — so this is derived, not
// declared, and adding a VLAN widens every trunk automatically.
func carriedVLANs(p Port, infra Infra) []string {
	if !p.Trunk {
		return nil
	}
	excluded := map[string]bool{}
	for _, n := range p.ExcludedNetworks {
		excluded[n] = true
	}
	var out []string
	for _, name := range sortedKeys(infra.Vlans) {
		if !excluded[name] {
			out = append(out, name)
		}
	}
	return out
}

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// BuildPhysical derives the cabling graph. Every edge comes from the
// port-profile pairing: a switch port names a profile, and whatever is on the
// other end names the same profile. An edge missing here means one of the two
// ends is silent, not that the cable does not exist.
//
// Everything with no cable is excluded — wireless clients, container veths,
// bridges, VLAN subinterfaces. Those belong to a logical view.
func BuildPhysical(infra Infra) Graph {
	g := Graph{Nodes: []Node{{ID: "internet", Name: "Internet", Role: RoleInternet}}}

	var gateway string
	for _, key := range sortedKeys(infra.Devices) {
		d := infra.Devices[key]
		role := roleOf(key)
		if role == RoleRouter && gateway == "" {
			gateway = key
		}
		g.Nodes = append(g.Nodes, Node{ID: key, Name: d.Name, Role: role, IP: d.IP})
	}

	// One node per machine, however many cables reach it.
	seenHost := map[string]bool{}
	for _, key := range sortedKeys(hostPorts) {
		node := hostPorts[key].Node
		if seenHost[node] {
			continue
		}
		seenHost[node] = true
		g.Nodes = append(g.Nodes, Node{ID: node, Name: node, Role: RoleHost})
	}

	for _, key := range sortedKeys(infra.Clients) {
		c := infra.Clients[key]
		if c.PortProfile == "" {
			continue // no cable
		}
		if _, isHostNIC := hostPorts[key]; isHostNIC {
			continue // already drawn as its machine
		}
		ip := ""
		if c.IP != nil {
			ip = *c.IP
		}
		g.Nodes = append(g.Nodes, Node{ID: key, Name: c.Name, Role: RoleDevice, IP: ip})
	}

	if gateway != "" {
		g.Edges = append(g.Edges, Edge{From: "internet", To: gateway, ToPort: "wan", Label: "WAN"})
	}

	for _, deviceKey := range sortedKeys(infra.Devices) {
		device := infra.Devices[deviceKey]
		for _, index := range sortedPortIndices(device.Ports) {
			port := device.Ports[index]
			if port.Profile == "" {
				continue
			}
			for _, peer := range peersOf(port.Profile, infra) {
				if peer == deviceKey {
					continue
				}
				idx, _ := strconv.Atoi(index)
				profile := infra.Ports[port.Profile]
				// A LAG stays one line, annotated with its member count. Drawing
				// a line per cable is more literal but reads worse: three
				// near-identical strands into one box, where the count says the
				// same thing in less ink.
				g.Edges = append(g.Edges, Edge{
					From:      deviceKey,
					FromIndex: idx,
					FromPort:  index,
					To:        nodeOf(peer),
					ToPort:    ifaceOf(peer, infra),
					Profile:   port.Profile,
					Label:     port.Name,
					Network:   profile.Network,
					Carried:   carriedVLANs(profile, infra),
					Excluded:  profile.ExcludedNetworks,
					Trunk:     profile.Trunk,
					LAG:       len(port.AggregateMembers),
					Upstream:  roleOf(peer) == RoleRouter,
				})
			}
		}
	}

	g.VlanIDs = map[string]int{}
	for _, e := range g.Edges {
		if !e.Trunk && e.Network != "" {
			g.VlanIDs[e.Network] = infra.Vlans[e.Network].VlanID
		}
	}

	return g
}

// RoleOfNode looks up a node's role by id, for renderers that vary an edge by
// what it lands on.
func (g Graph) RoleOfNode(id string) Role {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n.Role
		}
	}
	return RoleDevice
}
