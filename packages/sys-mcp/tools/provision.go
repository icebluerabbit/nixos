package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"sys-mcp/app"
)

const containerMACOUI = "62:f5:a4:5b:20:"
const bridgeMACOUI = "82:13:8a:a0:4f:"

func RegisterProvisionTools(s *server.MCPServer, ac *app.AppContext) {
	registerNextFreeIP(s, ac)
	registerNextFreeMAC(s, ac)
	registerValidateContainer(s, ac)
}

func registerNextFreeIP(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("next_free_ip",
			mcp.WithDescription("Find the next available static IP in a VLAN. Scans all staticAddresses and returns the lowest unused host address ≥ .3 (skipping network, gateway, and server slots). Useful when adding a new container."),
			mcp.WithString("vlan", mcp.Required(), mcp.Description("VLAN name (e.g. services, download, media)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			vlanName, err := req.RequireString("vlan")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			infra, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			v, ok := infra.Vlans[vlanName]
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("vlan %q not found", vlanName)), nil
			}

			base, err := ipv4Base(v.Subnet)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("parse subnet %q: %v", v.Subnet, err)), nil
			}

			used := make(map[int]string) // last octet → name
			for name, addr := range v.StaticAddresses {
				if oct, err := lastIPOctet(addr); err == nil {
					used[oct] = name
				}
			}

			next := nextFreeOctet(used)
			if next == -1 {
				return mcp.NewToolResultError("no free host addresses in /24 range"), nil
			}

			octets := make([]int, 0, len(used))
			for o := range used {
				octets = append(octets, o)
			}
			sort.Ints(octets)
			allocated := make([]string, 0, len(octets))
			for _, o := range octets {
				allocated = append(allocated, fmt.Sprintf("%s%d → %s", base, o, used[o]))
			}

			return jsonResult(map[string]any{
				"vlan":           vlanName,
				"vlanId":         v.VlanId,
				"subnet":         fmt.Sprintf("%s/%d", v.Subnet, v.SubnetPrefix),
				"nextFreeIp":     fmt.Sprintf("%s%d", base, next),
				"nextOctet":      next,
				"allocatedCount": len(used),
				"allocated":      allocated,
			})
		},
	)
}

func registerNextFreeMAC(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("next_free_mac",
			mcp.WithDescription("Compute the next available deterministic container MAC for a VLAN. Formula: OUI 62:f5:a4:5b:20: + hex(vlanId + containerIndex), containerIndex ≥ 1. Picks the lowest index whose last byte is free BOTH on this VLAN and globally — the single-byte space is shared across VLANs, so bytes owned by other VLANs are skipped to guarantee a collision-free MAC. Also returns the expected bridge MAC (82:13:8a:a0:4f: + hex(vlanId+32))."),
			mcp.WithString("vlan", mcp.Required(), mcp.Description("VLAN name (e.g. services, download, media)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			vlanName, err := req.RequireString("vlan")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			infra, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			v, ok := infra.Vlans[vlanName]
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("vlan %q not found", vlanName)), nil
			}

			vlanId := v.VlanId
			if vlanId <= 0 || vlanId+32 > 255 {
				return mcp.NewToolResultError(fmt.Sprintf("vlanId %d incompatible with MAC formula (bridge byte vlanId+32 must be ≤ 255)", vlanId)), nil
			}

			vlanIPs := make(map[string]bool, len(v.StaticAddresses))
			for _, addr := range v.StaticAddresses {
				vlanIPs[addr] = true
			}

			type usedEntry struct {
				Index    int    `json:"index"`
				LastByte string `json:"lastByte"`
				Mac      string `json:"mac"`
				Client   string `json:"client"`
			}

			usedIdx := make(map[int]string) // index → client key
			for clientKey, c := range infra.Clients {
				if c.Ip == nil || !vlanIPs[*c.Ip] {
					continue
				}
				mac := strings.ToLower(c.Mac)
				if !strings.HasPrefix(mac, containerMACOUI) {
					continue
				}
				lastHex := strings.TrimPrefix(mac, containerMACOUI)
				lastByte, err := strconv.ParseInt(lastHex, 16, 16)
				if err != nil {
					continue
				}
				idx := int(lastByte) - vlanId
				if idx >= 0 {
					usedIdx[idx] = clientKey
				}
			}

			// Every container-OUI last byte in use across ALL VLANs. The byte space
			// is shared, so the next index must avoid bytes owned by other VLANs too.
			globalBytes := make(map[int]string) // last byte → client key
			for clientKey, c := range infra.Clients {
				mac := strings.ToLower(c.Mac)
				if !strings.HasPrefix(mac, containerMACOUI) {
					continue
				}
				b, err := strconv.ParseInt(strings.TrimPrefix(mac, containerMACOUI), 16, 16)
				if err != nil {
					continue
				}
				globalBytes[int(b)] = clientKey
			}

			nextIndex := nextContainerMACIndex(usedIdx, vlanId, globalBytes)
			if nextIndex == -1 {
				return mcp.NewToolResultError("no free container MAC byte available (VLAN indices exhausted or all collide with other VLANs)"), nil
			}

			nextByte := vlanId + nextIndex
			nextMAC := containerMACFormula(vlanId, nextIndex)
			bridgeMAC := bridgeMACFormula(vlanId)

			used := make([]usedEntry, 0, len(usedIdx))
			for idx, key := range usedIdx {
				used = append(used, usedEntry{
					Index:    idx,
					LastByte: fmt.Sprintf("0x%02x", vlanId+idx),
					Mac:      fmt.Sprintf("%s%02x", containerMACOUI, vlanId+idx),
					Client:   key,
				})
			}
			sort.Slice(used, func(i, j int) bool { return used[i].Index < used[j].Index })

			return jsonResult(map[string]any{
				"vlan":         vlanName,
				"vlanId":       vlanId,
				"nextIndex":    nextIndex,
				"nextLastByte": fmt.Sprintf("0x%02x", nextByte),
				"nextMac":      nextMAC,
				"bridgeMac":    bridgeMAC,
				"formula":      fmt.Sprintf("%shex(%d + index)", containerMACOUI, vlanId),
				"usedOnVlan":   used,
			})
		},
	)
}

func registerValidateContainer(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("validate_container",
			mcp.WithDescription("Check a container's configuration completeness against the setup checklist. Verifies: VLAN static address, infra client registration, port declarations, IP/VLAN consistency, MAC formula validity, host declaration, and all required source files."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Container name without prefix (e.g. grafana, radarr, loki)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			infra, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			hosts, err := ac.HC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			type check struct {
				Name   string `json:"check"`
				Pass   bool   `json:"pass"`
				Detail string `json:"detail,omitempty"`
			}
			var checks []check
			addCheck := func(label string, ok bool, detail string) {
				checks = append(checks, check{label, ok, detail})
			}

			clientKey := "container-" + name

			// 1. VLAN static address
			vlanName := ""
			for vn, v := range infra.Vlans {
				if ip, ok := v.StaticAddresses[name]; ok {
					vlanName = vn
					addCheck("vlan_static_address", true, fmt.Sprintf("%s.staticAddresses.%s = %s", vn, name, ip))
					break
				}
			}
			if vlanName == "" {
				addCheck("vlan_static_address", false, fmt.Sprintf("no VLAN has staticAddresses.%s", name))
			}

			// 2. Infra client registered
			client, hasClient := infra.Clients[clientKey]
			if hasClient {
				addCheck("infra_client", true, fmt.Sprintf("%s registered (IP: %s, MAC: %s)", clientKey, derefIP(client.Ip), client.Mac))
			} else {
				addCheck("infra_client", false, fmt.Sprintf("no client %q found in infra", clientKey))
			}

			// 3. Ports declared
			if hasClient {
				if len(client.Ports) > 0 {
					ports := make([]string, 0, len(client.Ports))
					for k, v := range client.Ports {
						ports = append(ports, fmt.Sprintf("%s=%d", k, v))
					}
					sort.Strings(ports)
					addCheck("ports_declared", true, strings.Join(ports, ", "))
				} else {
					addCheck("ports_declared", false, "client has no ports declared")
				}
			} else {
				addCheck("ports_declared", false, "skipped: no infra client")
			}

			// 4. IP matches VLAN static address
			if hasClient && vlanName != "" {
				expectedIP := infra.Vlans[vlanName].StaticAddresses[name]
				if client.Ip != nil && *client.Ip == expectedIP {
					addCheck("ip_vlan_consistent", true, fmt.Sprintf("%s matches vlan staticAddress", *client.Ip))
				} else {
					addCheck("ip_vlan_consistent", false, fmt.Sprintf("client IP %s ≠ vlan staticAddress %s", derefIP(client.Ip), expectedIP))
				}
			}

			// 5. MAC formula valid
			if hasClient && vlanName != "" {
				v := infra.Vlans[vlanName]
				mac := strings.ToLower(client.Mac)
				if strings.HasPrefix(mac, containerMACOUI) {
					lastHex := strings.TrimPrefix(mac, containerMACOUI)
					lastByte, err := strconv.ParseInt(lastHex, 16, 16)
					if err == nil {
						index := int(lastByte) - v.VlanId
						if index >= 1 {
							addCheck("mac_formula", true, fmt.Sprintf("MAC %s = OUI + hex(%d+%d), index=%d", client.Mac, v.VlanId, index, index))
						} else {
							addCheck("mac_formula", false, fmt.Sprintf("MAC last byte 0x%02x → index=%d (must be ≥ 1; index 0 is the host VLAN client slot)", lastByte, index))
						}
					} else {
						addCheck("mac_formula", false, fmt.Sprintf("cannot parse last byte of MAC %s", client.Mac))
					}
				} else {
					addCheck("mac_formula", false, fmt.Sprintf("MAC %s does not use container OUI %s", client.Mac, containerMACOUI))
				}
			}

			// 5b. MAC globally unique. The last-byte space is shared across VLANs
			// (vlanId+index can collide between VLANs), so a formula-valid MAC may
			// still duplicate another client's. UniFi keys clients by MAC, so a
			// duplicate makes two Terraform clients fight over one controller record.
			if hasClient {
				mac := strings.ToLower(client.Mac)
				var clashes []string
				for k, c := range infra.Clients {
					if k == clientKey || mac == "" {
						continue
					}
					if strings.ToLower(c.Mac) == mac {
						clashes = append(clashes, k)
					}
				}
				if len(clashes) == 0 {
					addCheck("mac_unique", true, fmt.Sprintf("MAC %s not used by any other client", client.Mac))
				} else {
					sort.Strings(clashes)
					addCheck("mac_unique", false, fmt.Sprintf("MAC %s also used by: %s", client.Mac, strings.Join(clashes, ", ")))
				}
			}

			// 6. Container declared on a host
			containerHost := ""
			for hostName, eval := range hosts {
				if _, ok := eval.Containers[name]; ok {
					containerHost = hostName
					break
				}
			}
			if containerHost != "" {
				addCheck("host_declared", true, fmt.Sprintf("declared on host %q", containerHost))
			} else {
				addCheck("host_declared", false, fmt.Sprintf("container %q not found on any host", name))
			}

			// 7–10. Source files
			sources := ac.SI.Resolve(clientKey, name, vlanName, containerHost)

			if sources.InfraClient != nil {
				addCheck("source_infra_client", true, *sources.InfraClient)
			} else {
				addCheck("source_infra_client", false, fmt.Sprintf("modules/infra/clients/%s.nix not found", clientKey))
			}

			if vlanName != "" {
				if sources.InfraVlan != nil {
					addCheck("source_infra_vlan", true, *sources.InfraVlan)
				} else {
					addCheck("source_infra_vlan", false, fmt.Sprintf("modules/infra/networks/%s.nix not found", vlanName))
				}
			}

			if sources.ContainerAspect != nil {
				addCheck("source_container_aspect", true, *sources.ContainerAspect)
			} else {
				addCheck("source_container_aspect", false, fmt.Sprintf("modules/containers/%s.nix not found", name))
			}

			if sources.HostContainerConf != nil {
				addCheck("source_host_conf", true, *sources.HostContainerConf)
			} else {
				hostLabel := containerHost
				if hostLabel == "" {
					hostLabel = "<host>"
				}
				addCheck("source_host_conf", false, fmt.Sprintf("modules/hosts/%s/containers/%s.nix not found", hostLabel, name))
			}

			passCount, failCount := 0, 0
			for _, c := range checks {
				if c.Pass {
					passCount++
				} else {
					failCount++
				}
			}

			return jsonResult(map[string]any{
				"container": name,
				"clientKey": clientKey,
				"vlan":      vlanName,
				"host":      containerHost,
				"ready":     failCount == 0,
				"passCount": passCount,
				"failCount": failCount,
				"checks":    checks,
				"sources":   sources,
			})
		},
	)
}

// ipv4Base returns the first three octets as a prefix string (e.g. "10.10.140.").
func ipv4Base(subnet string) (string, error) {
	parts := strings.Split(subnet, ".")
	if len(parts) != 4 {
		return "", fmt.Errorf("not a dotted-quad IPv4: %q", subnet)
	}
	for _, p := range parts[:3] {
		if _, err := strconv.Atoi(p); err != nil {
			return "", fmt.Errorf("invalid octet %q in %q", p, subnet)
		}
	}
	return strings.Join(parts[:3], ".") + ".", nil
}

// lastIPOctet extracts the last octet of a dotted-quad IPv4 address.
func lastIPOctet(addr string) (int, error) {
	parts := strings.Split(addr, ".")
	if len(parts) != 4 {
		return 0, fmt.Errorf("not IPv4: %q", addr)
	}
	return strconv.Atoi(parts[3])
}

func derefIP(ip *string) string {
	if ip == nil {
		return "<nil>"
	}
	return *ip
}

// nextFreeOctet returns the lowest integer ≥ 3 not present in used (octet → name).
// Skips 0 (network), 1, and 2 (conventionally gateway/server).
func nextFreeOctet(used map[int]string) int {
	for i := 3; i <= 254; i++ {
		if _, taken := used[i]; !taken {
			return i
		}
	}
	return -1
}

// nextContainerMACIndex returns the lowest container index ≥ 1 whose last byte
// (vlanId+index) is free both on this VLAN (used: index → client key) and
// globally (globalBytes: last byte → client key). The MAC last-byte space is
// shared across every VLAN, so a byte free on this VLAN can still collide with a
// client on another VLAN whose (vlanId+index) lands on the same byte — that is
// exactly how media (base 0x78) index ≥10 overlaps download (base 0x82).
// Index 0 is reserved for the host VLAN client.
func nextContainerMACIndex(used map[int]string, vlanId int, globalBytes map[int]string) int {
	for i := 1; i <= 254-vlanId; i++ {
		if _, taken := used[i]; taken {
			continue
		}
		if _, clash := globalBytes[vlanId+i]; clash {
			continue
		}
		return i
	}
	return -1
}

// containerMACFormula computes the deterministic container MAC for a given VLAN ID and index.
// Formula: 62:f5:a4:5b:20: + hex(vlanId + index).
func containerMACFormula(vlanId, index int) string {
	return fmt.Sprintf("%s%02x", containerMACOUI, vlanId+index)
}

// bridgeMACFormula computes the deterministic host bridge MAC for a given VLAN ID.
// Formula: 82:13:8a:a0:4f: + hex(vlanId + 32).
func bridgeMACFormula(vlanId int) string {
	return fmt.Sprintf("%s%02x", bridgeMACOUI, vlanId+32)
}
