package cache

import (
	"fmt"
	"sort"
	"strings"
)

type ConnectedEntity struct {
	Type      string         `json:"type"` // "vlan"|"client"|"container"|"host"
	Vlan      *VlanInfo      `json:"vlan,omitempty"`
	Client    *ClientInfo    `json:"client,omitempty"`
	Container *ContainerInfo `json:"container,omitempty"`
	Host      *HostInfo      `json:"host,omitempty"`
	Sources   SourcePaths    `json:"sources"`
}

type VlanInfo struct {
	Name            string            `json:"name"`
	VlanId          int               `json:"vlanId"`
	Subnet          string            `json:"subnet"`
	SubnetPrefix    int               `json:"subnetPrefix"`
	Domain          string            `json:"domain"`
	StaticAddresses map[string]string `json:"staticAddresses"`
	// ClientKeys lists infra client keys whose IPs appear in this VLAN's staticAddresses.
	ClientKeys []string `json:"clientKeys,omitempty"`
}

type ClientInfo struct {
	Name    string         `json:"name"`
	Display string         `json:"display"`
	Ip      *string        `json:"ip"`
	Mac     string         `json:"mac"`
	Ports   map[string]int `json:"ports"`
	Blocked bool           `json:"blocked"`
	// Link is the switch port this client is cabled to, absent for wireless
	// clients and anything a host synthesises.
	Link *PortLink `json:"link,omitempty"`
}

type ContainerInfo struct {
	Name         string               `json:"name"`
	Bridge       string               `json:"bridge"`
	LocalAddress string               `json:"localAddress"`
	BindMounts   map[string]BindMount `json:"bindMounts"`
	// Commands are pre-built strings ready to paste into a terminal or Bash tool.
	Commands ContainerCommands `json:"commands"`
}

type ContainerCommands struct {
	Journal string `json:"journal"`
	Exec    string `json:"exec"`
	Status  string `json:"status"`
}

type HostInfo struct {
	Name      string `json:"name"`
	SshTarget string `json:"sshTarget"` // empty if local
	IsLocal   bool   `json:"isLocal"`
	// ContainerNames is set when the entity is rooted at a host.
	ContainerNames []string            `json:"containerNames,omitempty"`
	Aspects        map[string][]string `json:"aspects,omitempty"`
}

// PortPeer is one end of a cable: whatever declares a given port profile.
type PortPeer struct {
	Kind string `json:"kind"` // "client"|"device"
	Key  string `json:"key"`
	Name string `json:"name"`
}

// PortLink is the port profile a client or device is wired through, plus
// whoever else declares it. Nothing in Terraform ties the two ends together —
// the shared profile is the only thing that does.
type PortLink struct {
	Profile      string     `json:"profile"`
	Display      string     `json:"display"`
	Network      string     `json:"network"`
	Trunk        bool       `json:"trunk"`
	ResourceName string     `json:"resourceName"`
	Peers        []PortPeer `json:"peers,omitempty"`
}

// PortKeyForUnifiId maps a controller port-profile object id — what a switch's
// port_override carries — back to its infra.ports key. Empty if unknown, which
// happens for a profile applied outside Terraform.
func PortKeyForUnifiId(id string, infra *InfraData) string {
	if id == "" {
		return ""
	}
	for key, p := range infra.Ports {
		if p.UnifiId == id {
			return key
		}
	}
	return ""
}

// PeersOfPort lists everything declaring this port profile, skipping selfKey.
// Pass an empty selfKey to get every end.
func PeersOfPort(portKey, selfKey string, infra *InfraData) []PortPeer {
	if portKey == "" {
		return nil
	}
	var peers []PortPeer
	for key, c := range infra.Clients {
		if c.PortProfile == portKey && key != selfKey {
			peers = append(peers, PortPeer{Kind: "client", Key: key, Name: c.Name})
		}
	}
	for key, d := range infra.Devices {
		if d.PortProfile == portKey && key != selfKey {
			peers = append(peers, PortPeer{Kind: "device", Key: key, Name: d.Name})
		}
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].Key < peers[j].Key })
	return peers
}

// LinkForPortProfile builds the PortLink for a named profile, from the point of
// view of selfKey. Returns nil when the profile is unset or unknown.
func LinkForPortProfile(portKey, selfKey string, infra *InfraData) *PortLink {
	if portKey == "" {
		return nil
	}
	p, ok := infra.Ports[portKey]
	if !ok {
		return nil
	}
	return &PortLink{
		Profile:      portKey,
		Display:      p.Name,
		Network:      p.Network,
		Trunk:        p.Trunk,
		ResourceName: p.ResourceName,
		Peers:        PeersOfPort(portKey, selfKey, infra),
	}
}

type SourcePaths struct {
	InfraClient       *string `json:"infraClient,omitempty"`
	InfraVlan         *string `json:"infraVlan,omitempty"`
	InfraDevice       *string `json:"infraDevice,omitempty"`
	InfraWlan         *string `json:"infraWlan,omitempty"`
	InfraPortForward  *string `json:"infraPortForward,omitempty"`
	ContainerAspect   *string `json:"containerAspect,omitempty"`
	HostContainerConf *string `json:"hostContainerConf,omitempty"`
}

// ResolveFromVlan returns a ConnectedEntity rooted at a VLAN.
// ClientKeys lists which infra clients live on this VLAN.
func ResolveFromVlan(vlanName string, infra *InfraData, idx *SourceIndex) ConnectedEntity {
	e := ConnectedEntity{Type: "vlan"}
	v, ok := infra.Vlans[vlanName]
	if !ok {
		return e
	}

	ipToClient := ipToClientKey(infra)
	var clientKeys []string
	for _, addr := range v.StaticAddresses {
		if key, ok := ipToClient[addr]; ok {
			clientKeys = append(clientKeys, key)
		}
	}

	e.Vlan = &VlanInfo{
		Name:            vlanName,
		VlanId:          v.VlanId,
		Subnet:          v.Subnet,
		SubnetPrefix:    v.SubnetPrefix,
		Domain:          v.Domain,
		StaticAddresses: v.StaticAddresses,
		ClientKeys:      clientKeys,
	}
	e.Sources = idx.Resolve("", "", vlanName, "")
	return e
}

// ResolveFromClient returns a ConnectedEntity rooted at an infra client key.
// Resolves VLAN upward and container+host downward (requires hosts data).
func ResolveFromClient(clientKey string, infra *InfraData, hosts HostsData, idx *SourceIndex, localHostname string) ConnectedEntity {
	e := ConnectedEntity{Type: "client"}
	c, ok := infra.Clients[clientKey]
	if !ok {
		return e
	}

	e.Client = &ClientInfo{
		Name:    clientKey,
		Display: c.Name,
		Ip:      c.Ip,
		Mac:     c.Mac,
		Ports:   c.Ports,
		Blocked: c.Blocked,
		Link:    LinkForPortProfile(c.PortProfile, clientKey, infra),
	}

	vlanName := vlanForClient(c, infra)
	if vlanName != "" {
		e.Vlan = vlanInfo(vlanName, infra)
	}

	containerName, hostName := containerForIP(c.Ip, hosts)
	if containerName != "" {
		conf := hosts[hostName].Containers[containerName]
		sshTarget := hostName
		e.Host = &HostInfo{Name: hostName, SshTarget: sshTarget, IsLocal: hostName == localHostname}
		e.Container = containerInfo(containerName, conf, sshTarget, hostName == localHostname)
	}

	e.Sources = idx.Resolve(clientKey, containerName, vlanName, hostName)
	return e
}

// ResolveFromContainer returns a ConnectedEntity rooted at a container name on a specific host.
// Resolves client+VLAN from localAddress and host from hosts data.
func ResolveFromContainer(containerName, hostName string, infra *InfraData, hosts HostsData, idx *SourceIndex, localHostname string) ConnectedEntity {
	e := ConnectedEntity{Type: "container"}
	conf, ok := hosts[hostName].Containers[containerName]
	if !ok {
		return e
	}

	sshTarget := hostName
	e.Host = &HostInfo{Name: hostName, SshTarget: sshTarget, IsLocal: hostName == localHostname}
	e.Container = containerInfo(containerName, conf, sshTarget, hostName == localHostname)

	ip := stripCIDR(conf.LocalAddress)
	vlanName := ""
	clientKey := ""
	if ip != "" {
		for key, c := range infra.Clients {
			if c.Ip != nil && *c.Ip == ip {
				clientKey = key
				e.Client = &ClientInfo{
					Name:    key,
					Display: c.Name,
					Ip:      c.Ip,
					Mac:     c.Mac,
					Ports:   c.Ports,
					Blocked: c.Blocked,
				}
				vlanName = vlanForClient(c, infra)
				if vlanName != "" {
					e.Vlan = vlanInfo(vlanName, infra)
				}
				break
			}
		}
	}

	e.Sources = idx.Resolve(clientKey, containerName, vlanName, hostName)
	return e
}

// ResolveFromHost returns a ConnectedEntity rooted at a host.
// ContainerNames lists all containers declared on that host.
func ResolveFromHost(hostName string, infra *InfraData, hosts HostsData, localHostname string) ConnectedEntity {
	e := ConnectedEntity{Type: "host"}
	hostEval, ok := hosts[hostName]
	if !ok {
		return e
	}

	names := make([]string, 0, len(hostEval.Containers))
	for name := range hostEval.Containers {
		names = append(names, name)
	}

	sshTarget := hostName
	e.Host = &HostInfo{
		Name:           hostName,
		SshTarget:      sshTarget,
		IsLocal:        hostName == localHostname,
		ContainerNames: names,
		Aspects:        summarizeAspects(hostEval.Aspects),
	}
	return e
}

// summarizeAspects filters and groups a flat list of dotted aspect names for LLM consumption.
// Drops infra.* and containers.* (covered by dedicated tools).
// Groups remainder by top-level prefix, truncating to depth 2.
// Entries with children show count: "neovim (42)"; leaves at exactly depth 2 show plain: "bat".
func summarizeAspects(aspects []string) map[string][]string {
	if len(aspects) == 0 {
		return nil
	}

	type entry struct {
		count  int
		isLeaf bool
	}
	groups := map[string]map[string]*entry{}

	for _, a := range aspects {
		if strings.HasPrefix(a, "infra.") || strings.HasPrefix(a, "containers.") {
			continue
		}
		parts := strings.SplitN(a, ".", 3)
		if len(parts) < 2 {
			continue
		}
		top, second := parts[0], parts[1]
		if groups[top] == nil {
			groups[top] = map[string]*entry{}
		}
		if groups[top][second] == nil {
			groups[top][second] = &entry{}
		}
		groups[top][second].count++
		if len(parts) == 2 {
			groups[top][second].isLeaf = true
		}
	}

	if len(groups) == 0 {
		return nil
	}

	result := make(map[string][]string, len(groups))
	for top, seconds := range groups {
		labels := make([]string, 0, len(seconds))
		for second, e := range seconds {
			if e.count == 1 && e.isLeaf {
				labels = append(labels, second)
			} else {
				labels = append(labels, fmt.Sprintf("%s (%d)", second, e.count))
			}
		}
		sort.Strings(labels)
		result[top] = labels
	}
	return result
}

// --- internal helpers ---

func vlanInfo(vlanName string, infra *InfraData) *VlanInfo {
	v := infra.Vlans[vlanName]
	return &VlanInfo{
		Name:            vlanName,
		VlanId:          v.VlanId,
		Subnet:          v.Subnet,
		SubnetPrefix:    v.SubnetPrefix,
		Domain:          v.Domain,
		StaticAddresses: v.StaticAddresses,
	}
}

func containerInfo(name string, conf ContainerConf, sshTarget string, isLocal bool) *ContainerInfo {
	return &ContainerInfo{
		Name:         name,
		Bridge:       conf.HostBridge,
		LocalAddress: conf.LocalAddress,
		BindMounts:   conf.BindMounts,
		Commands:     buildCommands(name, sshTarget, isLocal),
	}
}

func buildCommands(name, sshTarget string, isLocal bool) ContainerCommands {
	prefix := ""
	if !isLocal && sshTarget != "" {
		prefix = fmt.Sprintf("ssh %s ", sshTarget)
	}
	return ContainerCommands{
		Journal: fmt.Sprintf("%ssudo nixos-container run %s -- journalctl -n 100", prefix, name),
		Exec:    fmt.Sprintf("%ssudo nixos-container run %s -- <command>", prefix, name),
		Status:  fmt.Sprintf("%ssystemctl status container@%s.service", prefix, name),
	}
}

// VlanForClient prefers the VLAN the client declares. The staticAddresses
// lookup is only a fallback: it cannot resolve a client whose address is a
// literal rather than an allocation — every physical device is one — and it
// cannot resolve a client with no address at all.
func VlanForClient(c Client, infra *InfraData) string {
	if c.Vlan != "" {
		return c.Vlan
	}
	if c.Ip == nil {
		return ""
	}
	for vlanName, v := range infra.Vlans {
		for _, addr := range v.StaticAddresses {
			if addr == *c.Ip {
				return vlanName
			}
		}
	}
	return ""
}

func vlanForClient(c Client, infra *InfraData) string { return VlanForClient(c, infra) }

func containerForIP(ip *string, hosts HostsData) (containerName, hostName string) {
	if ip == nil || hosts == nil {
		return "", ""
	}
	for host, eval := range hosts {
		for name, conf := range eval.Containers {
			if stripCIDR(conf.LocalAddress) == *ip {
				return name, host
			}
		}
	}
	return "", ""
}

// HostSSH returns the first IP found for hostName across all VLAN staticAddresses.
// Prefer using the hostname directly as an SSH target — ~/.ssh/config maps it correctly.
func HostSSH(hostName string, infra *InfraData) string {
	for _, v := range infra.Vlans {
		if ip, ok := v.StaticAddresses[hostName]; ok {
			return ip
		}
	}
	return ""
}

// ClientKeyForContainer maps a container name to its infra client key.
//
// The usual key is "container-<name>", but that is a convention rather than a
// guarantee — `dnsmasq` registers as `container-dns` and `postgresql` as
// `container-postgres`. When the name does not match, fall back to the
// container's own address: its `localAddress` is assigned from the client's
// `ip`, so the two are the same host address and the match is exact.
func ClientKeyForContainer(containerName string, hosts HostsData, infra *InfraData) string {
	if infra == nil {
		return ""
	}
	conventional := "container-" + containerName
	if _, ok := infra.Clients[conventional]; ok {
		return conventional
	}
	byIP := ipToClientKey(infra)
	for _, eval := range hosts {
		conf, ok := eval.Containers[containerName]
		if !ok {
			continue
		}
		if key, ok := byIP[stripCIDR(conf.LocalAddress)]; ok {
			return key
		}
	}
	return ""
}

func ipToClientKey(infra *InfraData) map[string]string {
	m := make(map[string]string, len(infra.Clients))
	for key, c := range infra.Clients {
		if c.Ip != nil {
			m[*c.Ip] = key
		}
	}
	return m
}

func stripCIDR(addr string) string {
	if i := strings.Index(addr, "/"); i >= 0 {
		return addr[:i]
	}
	return addr
}
