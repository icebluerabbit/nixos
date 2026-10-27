package graph

import (
	"sort"
	"strings"

	"homelab/topology/internal/infra"
)

// The logical view picks up where the physical one stops. The cabling diagram
// ends at the server's bond0; everything past that is interfaces the host
// synthesises, and none of it is visible to the switch.
//
// The chain is real config, not inference:
//
//	containers.<name>.hostBridge  →  br-media
//	bridges.br-media.interfaces   →  vlan-media
//	vlans.vlan-media.interface    →  bond0
//	vlans.vlan-media.id           →  120
//
// Containers bridge straight onto a VLAN with no host NAT, so each one is a
// first-class LAN client with its own address — which is why they are drawn as
// peers on a network rather than as things hanging off a host.

// VlanGroup is one VLAN and everything the host puts on it.
type VlanGroup struct {
	// Name is the infra VLAN key: media, services, apps.
	Name   string
	ID     int
	Subnet string
	Prefix int
	Bridge string
	Iface  string
	// HostIP is the address the bridge itself holds, i.e. the containers'
	// gateway onto the rest of the VLAN.
	HostIP     string
	Containers []ContainerNode
	// Clients is real hardware on the same VLAN — a desktop, a NAS, a console.
	// A VLAN the server runs nothing on is not therefore unused, and saying so
	// was the first thing this view got wrong.
	Clients []string
}

// ContainerNode is one nspawn guest on a VLAN.
type ContainerNode struct {
	Name string
	IP   string
}

// Logical is the server's internal network: one bond, N VLANs, their
// containers.
type Logical struct {
	Host   string
	Bond   string
	NICs   []string
	Groups []VlanGroup
	// Orphans are bridges with no VLAN uplink — the per-tunnel `br-vpn*` ones,
	// which carry veth pairs between vpn-gw and its clients and never touch a
	// VLAN. Counted rather than drawn: ten identical boxes say nothing.
	Orphans []string
}

func stripCIDR(addr string) string {
	if i := strings.IndexByte(addr, '/'); i >= 0 {
		return addr[:i]
	}
	return addr
}

// vlanKeyForIface maps `vlan-media` back to the infra VLAN key `media`. The
// host names its subinterfaces after the registry, which is what makes the
// join possible at all.
func vlanKeyForIface(iface string, reg infra.Infra) string {
	name := strings.TrimPrefix(iface, "vlan-")
	if _, ok := reg.Vlans[name]; ok {
		return name
	}
	return ""
}

// BuildLogical derives the server-internal graph. A VLAN with no containers is
// still drawn: the bridge exists, and its absence from the diagram would read
// as "not configured" rather than "nothing on it yet".
func BuildLogical(in infra.Input, host string) Logical {
	l := Logical{Host: host}

	for _, name := range sortedKeys(in.Bonds) {
		l.Bond = name
		l.NICs = in.Bonds[name].Interfaces
		break // one bond; more would need the view rethought, not just a loop
	}

	byBridge := map[string][]ContainerNode{}
	for _, name := range sortedKeys(in.Containers) {
		c := in.Containers[name]
		if c.HostBridge == "" {
			continue
		}
		byBridge[c.HostBridge] = append(byBridge[c.HostBridge],
			ContainerNode{Name: name, IP: stripCIDR(c.LocalAddress)})
	}

	for _, bridge := range sortedKeys(in.Bridges) {
		b := in.Bridges[bridge]
		if len(b.Interfaces) == 0 {
			l.Orphans = append(l.Orphans, bridge)
			continue
		}
		iface := b.Interfaces[0]
		key := vlanKeyForIface(iface, in.Infra)
		if key == "" {
			l.Orphans = append(l.Orphans, bridge)
			continue
		}
		v := in.Infra.Vlans[key]

		// The bridge holds the host's address on this VLAN. It is registered
		// as an infra client, so read it from there rather than re-deriving
		// it.
		hostIP := ""
		if c, ok := in.Infra.Clients["server-bridge-"+key]; ok && c.IP != nil {
			hostIP = *c.IP
		}

		// Physical only: the virtual clients on a VLAN are the server's own
		// bridge, subinterface and container veths, which this view already
		// draws as the bridge and its containers.
		var clients []string
		for _, ck := range sortedKeys(in.Infra.Clients) {
			c := in.Infra.Clients[ck]
			if c.Vlan == key && c.Kind == "physical" {
				clients = append(clients, ck)
			}
		}

		l.Groups = append(l.Groups, VlanGroup{
			Name:       key,
			ID:         v.VlanID,
			Subnet:     v.Subnet,
			Prefix:     v.SubnetPrefix,
			Bridge:     bridge,
			Iface:      iface,
			HostIP:     hostIP,
			Containers: byBridge[bridge],
			Clients:    clients,
		})
	}

	// Busiest VLANs first: the diagram is read top-down and the interesting
	// groups should not be at the bottom.
	sort.SliceStable(l.Groups, func(i, j int) bool {
		if len(l.Groups[i].Containers) != len(l.Groups[j].Containers) {
			return len(l.Groups[i].Containers) > len(l.Groups[j].Containers)
		}
		return l.Groups[i].Name < l.Groups[j].Name
	})

	return l
}
