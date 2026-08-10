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

func vlanForClient(c Client, infra *InfraData) string {
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
