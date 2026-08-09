package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"sys-mcp/app"
	"sys-mcp/cache"
)

type searchResult struct {
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Summary   string            `json:"summary"`
	MatchedOn string            `json:"matched_on"`
	Sources   cache.SourcePaths `json:"sources,omitempty"`
}

func RegisterSearchTools(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("search",
			mcp.WithDescription("Fuzzy search across all infra types (clients, containers, VLANs, devices, hosts) by any identifying value: name fragment, IP address, MAC address, or port number. Returns tagged results with a one-line summary and the field that matched. Use when the type of the target is unknown."),
			mcp.WithString("query", mcp.Required(), mcp.Description("Search term: name fragment, IP, MAC, or port number")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, err := req.RequireString("query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			q := strings.ToLower(query)
			var results []searchResult

			for name, v := range data.Vlans {
				if m := matchVlan(name, v, q); m != "" {
					results = append(results, searchResult{
						Type:      "vlan",
						Name:      name,
						Summary:   fmt.Sprintf("VLAN %d | %s/%d | domain: %s | %d static addresses", v.VlanId, v.Subnet, v.SubnetPrefix, v.Domain, len(v.StaticAddresses)),
						MatchedOn: m,
						Sources:   ac.SI.ByVLANName(name),
					})
				}
			}

			for name, c := range data.Clients {
				if m := matchClient(name, c, q); m != "" {
					results = append(results, searchResult{
						Type:      "client",
						Name:      name,
						Summary:   clientSummary(c),
						MatchedOn: m,
						Sources:   ac.SI.ByClientKey(name),
					})
				}
			}

			for name, d := range data.Devices {
				if m := matchDevice(name, d, q); m != "" {
					results = append(results, searchResult{
						Type:      "device",
						Name:      name,
						Summary:   fmt.Sprintf("IP: %s | MAC: %s | type: %s", d.Ip, d.Mac, d.Type),
						MatchedOn: m,
						Sources:   ac.SI.ByDeviceKey(name),
					})
				}
			}

			return jsonResult(results)
		},
	)

	s.AddTool(
		mcp.NewTool("get_by_ip",
			mcp.WithDescription("Exact IP address lookup. Returns the entity that owns the IP: infra client, device, or host interface. Includes type tag and one-line summary."),
			mcp.WithString("ip", mcp.Required(), mcp.Description("Exact IP address (e.g. 10.10.140.9)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ip, err := req.RequireString("ip")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			for name, c := range data.Clients {
				if c.Ip != nil && *c.Ip == ip {
					return jsonResult(searchResult{
						Type:      "client",
						Name:      name,
						Summary:   clientSummary(c),
						MatchedOn: "ip:" + ip,
						Sources:   ac.SI.ByClientKey(name),
					})
				}
			}
			for name, d := range data.Devices {
				if d.Ip == ip {
					return jsonResult(searchResult{
						Type:      "device",
						Name:      name,
						Summary:   fmt.Sprintf("IP: %s | MAC: %s | type: %s", d.Ip, d.Mac, d.Type),
						MatchedOn: "ip:" + ip,
						Sources:   ac.SI.ByDeviceKey(name),
					})
				}
			}
			for vlanName, v := range data.Vlans {
				for addrName, addr := range v.StaticAddresses {
					if addr == ip {
						return jsonResult(searchResult{
							Type:      "vlan-address",
							Name:      vlanName + "." + addrName,
							Summary:   fmt.Sprintf("static address %q in VLAN %s (%s/%d)", addrName, vlanName, v.Subnet, v.SubnetPrefix),
							MatchedOn: "ip:" + ip,
						})
					}
				}
			}

			return mcp.NewToolResultError(fmt.Sprintf("no entity found with IP %s", ip)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("get_by_port",
			mcp.WithDescription("Find all infra clients and containers that expose a specific port number. Returns tagged results with the named port key and a one-line summary."),
			mcp.WithNumber("port", mcp.Required(), mcp.Description("Port number to look up (e.g. 3000)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			raw, ok := req.Params.Arguments.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("invalid arguments"), nil
			}
			port, err := strconv.Atoi(fmt.Sprintf("%v", raw["port"]))
			if err != nil {
				return mcp.NewToolResultError("port must be a number"), nil
			}
			data, err := ac.IC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var results []searchResult
			for name, c := range data.Clients {
				for portName, p := range c.Ports {
					if p == port {
						results = append(results, searchResult{
							Type:      "client",
							Name:      name,
							Summary:   clientSummary(c),
							MatchedOn: fmt.Sprintf("port:%s=%d", portName, port),
							Sources:   ac.SI.ByClientKey(name),
						})
					}
				}
			}
			return jsonResult(results)
		},
	)
}

func clientSummary(c cache.Client) string {
	ip := "<no ip>"
	if c.Ip != nil {
		ip = *c.Ip
	}
	ports := make([]string, 0, len(c.Ports))
	for k, v := range c.Ports {
		ports = append(ports, fmt.Sprintf("%s=%d", k, v))
	}
	return fmt.Sprintf("IP: %s | MAC: %s | ports: %s", ip, c.Mac, strings.Join(ports, ", "))
}

func matchVlan(name string, v cache.Vlan, q string) string {
	if strings.Contains(strings.ToLower(name), q) {
		return "name:" + name
	}
	if strings.Contains(v.Domain, q) {
		return "domain:" + v.Domain
	}
	if strings.Contains(v.Subnet, q) {
		return "subnet:" + v.Subnet
	}
	if strings.Contains(fmt.Sprintf("%d", v.VlanId), q) {
		return fmt.Sprintf("vlanId:%d", v.VlanId)
	}
	for addrName, addr := range v.StaticAddresses {
		if strings.Contains(addr, q) || strings.Contains(addrName, q) {
			return fmt.Sprintf("staticAddress:%s=%s", addrName, addr)
		}
	}
	return ""
}

func matchClient(name string, c cache.Client, q string) string {
	if strings.Contains(strings.ToLower(name), q) {
		return "name:" + name
	}
	if c.Ip != nil && strings.Contains(*c.Ip, q) {
		return "ip:" + *c.Ip
	}
	if strings.Contains(strings.ToLower(c.Mac), q) {
		return "mac:" + c.Mac
	}
	if strings.Contains(strings.ToLower(c.Name), q) {
		return "displayName:" + c.Name
	}
	for portName, p := range c.Ports {
		if strings.Contains(fmt.Sprintf("%d", p), q) {
			return fmt.Sprintf("port:%s=%d", portName, p)
		}
	}
	return ""
}

func matchDevice(name string, d cache.Device, q string) string {
	if strings.Contains(strings.ToLower(name), q) {
		return "name:" + name
	}
	if strings.Contains(d.Ip, q) {
		return "ip:" + d.Ip
	}
	if strings.Contains(strings.ToLower(d.Mac), q) {
		return "mac:" + d.Mac
	}
	if strings.Contains(strings.ToLower(d.Name), q) {
		return "displayName:" + d.Name
	}
	return ""
}
