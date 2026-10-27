package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
)

// RegisterInfraTools registers the registry lookups: topology, hosts,
// containers, clients, devices, VLANs and the by-IP/by-port resolvers.
func RegisterInfraTools(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("describe_topology",
			mcp.WithDescription("High-level overview of the entire homelab: VLAN count, host count, container count, device count, domain names. Start here when context is unknown."),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			infra, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			domains := make([]string, 0, len(infra.Vlans))
			for _, v := range infra.Vlans {
				domains = append(domains, v.Domain)
			}
			containerCount := 0
			for name := range infra.Clients {
				if strings.HasPrefix(name, "container-") {
					containerCount++
				}
			}
			hosts, _ := ac.HC.Get()
			result := map[string]any{
				"vlans":      len(infra.Vlans),
				"clients":    len(infra.Clients),
				"containers": containerCount,
				"devices":    len(infra.Devices),
				"hosts":      len(hosts),
				"domains":    domains,
			}
			return jsonResult(result)
		},
	)

	s.AddTool(
		mcp.NewTool("list_vlans",
			mcp.WithDescription("List all VLANs with ID, subnet, domain, and static address count."),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			infra, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			type row struct {
				Name               string `json:"name"`
				VlanId             int    `json:"vlanId"`
				Subnet             string `json:"subnet"`
				SubnetPrefix       int    `json:"subnetPrefix"`
				Domain             string `json:"domain"`
				StaticAddressCount int    `json:"staticAddressCount"`
			}
			rows := make([]row, 0, len(infra.Vlans))
			for name, v := range infra.Vlans {
				rows = append(rows, row{
					Name:               name,
					VlanId:             v.VlanId,
					Subnet:             v.Subnet,
					SubnetPrefix:       v.SubnetPrefix,
					Domain:             v.Domain,
					StaticAddressCount: len(v.StaticAddresses),
				})
			}
			return jsonResult(rows)
		},
	)

	s.AddTool(
		mcp.NewTool("get_vlan",
			mcp.WithDescription("Full detail for a single VLAN including all static addresses and which infra clients live on it."),
			mcp.WithString("name", mcp.Required(), mcp.Description("VLAN name (e.g. services, download, media)")),
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
			if _, ok := infra.Vlans[name]; !ok {
				return mcp.NewToolResultError(fmt.Sprintf("vlan %q not found", name)), nil
			}
			return jsonResult(cache.ResolveFromVlan(name, infra, ac.SI))
		},
	)

	s.AddTool(
		mcp.NewTool("list_clients",
			mcp.WithDescription("List all registered infra clients (containers, host interfaces, devices) with name, IP, MAC, and ports."),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			infra, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			type row struct {
				Name  string         `json:"name"`
				Ip    *string        `json:"ip"`
				Mac   string         `json:"mac"`
				Ports map[string]int `json:"ports"`
			}
			rows := make([]row, 0, len(infra.Clients))
			for name, c := range infra.Clients {
				rows = append(rows, row{Name: name, Ip: c.Ip, Mac: c.Mac, Ports: c.Ports})
			}
			return jsonResult(rows)
		},
	)

	s.AddTool(
		mcp.NewTool("get_client",
			mcp.WithDescription("Full detail for a single infra client: IP, MAC, ports, VLAN, and container+host if applicable."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Client name (e.g. container-grafana, server-bridge-services)")),
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
			if _, ok := infra.Clients[name]; !ok {
				return mcp.NewToolResultError(fmt.Sprintf("client %q not found", name)), nil
			}
			hosts, _ := ac.HC.Get()
			return jsonResult(cache.ResolveFromClient(name, infra, hosts, ac.SI, ac.Cfg.Hostname))
		},
	)

	s.AddTool(
		mcp.NewTool("list_devices",
			mcp.WithDescription("List physical network devices (switches, APs, UDM) registered in infra."),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			infra, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			type row struct {
				Name string `json:"name"`
				Ip   string `json:"ip"`
				Mac  string `json:"mac"`
				Type string `json:"type"`
			}
			rows := make([]row, 0, len(infra.Devices))
			for name, d := range infra.Devices {
				rows = append(rows, row{Name: name, Ip: d.Ip, Mac: d.Mac, Type: d.Type})
			}
			return jsonResult(rows)
		},
	)

	s.AddTool(
		mcp.NewTool("get_device",
			mcp.WithDescription("Full detail for a single physical network device."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Device name (e.g. usw-pro-hd-24-poe)")),
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
			d, ok := infra.Devices[name]
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("device %q not found", name)), nil
			}
			out := map[string]any{
				"name": name,
				"ip":   d.Ip,
				"mac":  d.Mac,
				"type": d.Type,
			}
			// The upstream port this device plugs into, not a profile on its own
			// ports — switches and APs are both peers and port owners.
			if link := cache.LinkForPortProfile(d.PortProfile, name, infra); link != nil {
				out["uplink"] = link
			}
			return jsonResult(out)
		},
	)

	s.AddTool(
		mcp.NewTool("list_containers",
			mcp.WithDescription("List all declared containers with their host, IP, ports, VLAN, bridge, and pre-built diagnostic commands."),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			infra, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			hosts, err := ac.HC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			var entities []cache.ConnectedEntity
			for hostName, eval := range hosts {
				for containerName := range eval.Containers {
					entities = append(entities, cache.ResolveFromContainer(containerName, hostName, infra, hosts, ac.SI, ac.Cfg.Hostname))
				}
			}
			return jsonResult(entities)
		},
	)

	s.AddTool(
		mcp.NewTool("get_container",
			mcp.WithDescription("Full detail for a single container: infra client (IP, MAC, ports), VLAN, host, bridge, bind mounts, and pre-built SSH/nixos-container commands."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Container name (e.g. grafana, radarr, loki)")),
			mcp.WithString("host", mcp.Description("Host name to scope the search (e.g. server). Omit to search all hosts.")),
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

			hostFilter, _ := req.Params.Arguments.(map[string]any)["host"].(string)

			for hostName, eval := range hosts {
				if hostFilter != "" && hostName != hostFilter {
					continue
				}
				if _, ok := eval.Containers[name]; ok {
					return jsonResult(cache.ResolveFromContainer(name, hostName, infra, hosts, ac.SI, ac.Cfg.Hostname))
				}
			}
			return mcp.NewToolResultError(fmt.Sprintf("container %q not found", name)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("list_hosts",
			mcp.WithDescription("List all NixOS hosts with container counts and SSH targets."),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			infra, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			hosts, err := ac.HC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			entities := make([]cache.ConnectedEntity, 0, len(hosts))
			for hostName := range hosts {
				entities = append(entities, cache.ResolveFromHost(hostName, infra, hosts, ac.Cfg.Hostname))

			}
			return jsonResult(entities)
		},
	)

	s.AddTool(
		mcp.NewTool("get_host",
			mcp.WithDescription("Full detail for a NixOS host: SSH target, all container names, and whether it is the current host."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Host name (e.g. server, desktop)")),
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
			if _, ok := hosts[name]; !ok {
				return mcp.NewToolResultError(fmt.Sprintf("host %q not found", name)), nil
			}
			return jsonResult(cache.ResolveFromHost(name, infra, hosts, ac.Cfg.Hostname))
		},
	)
}

// clientVlan finds the VLAN name whose staticAddresses contains the client's IP.
// Kept for search.go and infra_test.go compatibility.
func clientVlan(clientName string, c cache.Client, data *cache.InfraData) string {
	if c.Ip == nil {
		return ""
	}
	for vlanName, v := range data.Vlans {
		for _, addr := range v.StaticAddresses {
			if addr == *c.Ip {
				return vlanName
			}
		}
	}
	return ""
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
