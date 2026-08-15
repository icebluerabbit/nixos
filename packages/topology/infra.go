// Package main renders network diagrams from the infra registries.
//
// The structs below mirror `modules/infra/options.nix` and are deliberately a
// separate copy from sys-mcp's. Sharing them would mean either a source-sibling
// module or a published one, and Go's fixed-output vendor derivation forbids the
// obvious middle path — a shared derivation, because an FOD may reach the
// network but not the store. For two consumers reading a schema that changes in
// lockstep with this repo, a second copy plus golden tests is the cheaper trade.
//
// Only the fields a diagram needs are declared; unknown JSON keys are ignored.
package main

// Vlan is one L2 network.
type Vlan struct {
	VlanID          int               `json:"vlanId"`
	Subnet          string            `json:"subnet"`
	SubnetPrefix    int               `json:"subnetPrefix"`
	Domain          string            `json:"domain"`
	Name            string            `json:"name"`
	StaticAddresses map[string]string `json:"staticAddresses"`
}

// Client is anything with a MAC the controller knows about: real hardware, a
// host's own NIC, or an interface a host synthesises.
type Client struct {
	Mac  string  `json:"mac"`
	Name string  `json:"name"`
	IP   *string `json:"ip"`
	Vlan string  `json:"vlan"`
	// Kind is "physical" or "virtual".
	Kind string `json:"kind"`
	// PortProfile names the infra.ports key on the switch port this client is
	// wired to. Empty for anything with no cable.
	PortProfile string `json:"portProfile"`
}

// DevicePort is one switch port, keyed by index in Device.Ports.
type DevicePort struct {
	Name string `json:"name"`
	// Profile is an infra.ports key, or empty for a port carrying no profile:
	// a LAG member, or an uplink left unmanaged.
	Profile          string `json:"profile"`
	AggregateMembers []int  `json:"aggregateMembers"`
}

// Device is UniFi hardware.
type Device struct {
	Mac  string `json:"mac"`
	Name string `json:"name"`
	IP   string `json:"ip"`
	// PortProfile names the profile on the *upstream* port this device plugs
	// into — not a profile on one of its own ports.
	PortProfile string                `json:"portProfile"`
	Ports       map[string]DevicePort `json:"ports"`
}

// Port is a UniFi port profile. One per connection, so it doubles as the link
// identity: the switch names it on a port, the peer names it in PortProfile.
type Port struct {
	Name             string   `json:"name"`
	Network          string   `json:"network"`
	ResourceName     string   `json:"resourceName"`
	Trunk            bool     `json:"trunk"`
	ExcludedNetworks []string `json:"excludedNetworks"`
}

// Infra is `nixosConfigurations.<host>.config.infra`.
type Infra struct {
	Vlans   map[string]Vlan   `json:"vlans"`
	Clients map[string]Client `json:"clients"`
	Devices map[string]Device `json:"devices"`
	Ports   map[string]Port   `json:"ports"`
}

// Bond is an 802.3ad group of physical NICs.
type Bond struct {
	Interfaces []string `json:"interfaces"`
}

// VlanIface is an 802.1Q subinterface: `vlan-media` tagged 120 on `bond0`.
type VlanIface struct {
	ID        int    `json:"id"`
	Interface string `json:"interface"`
}

// Bridge is a Linux bridge. Its Interfaces are the uplinks it is built on — a
// VLAN subinterface for the VLAN bridges, and nothing at all for the per-tunnel
// `br-vpn*` ones, which exist only to hold veth pairs.
type Bridge struct {
	Interfaces []string `json:"interfaces"`
}

// Container is one nspawn guest. It bridges directly onto a VLAN — no host NAT —
// so its address is a first-class LAN address.
type Container struct {
	HostBridge   string `json:"hostBridge"`
	LocalAddress string `json:"localAddress"`
}

// Input is what the tool reads on stdin: the shared registries plus the parts of
// one host's config that describe how its containers reach the network. The
// logical view needs both; `infra` alone stops at the trunk.
type Input struct {
	Infra      Infra                `json:"infra"`
	Bonds      map[string]Bond      `json:"bonds"`
	VlanIfaces map[string]VlanIface `json:"vlans"`
	Bridges    map[string]Bridge    `json:"bridges"`
	Containers map[string]Container `json:"containers"`
}
