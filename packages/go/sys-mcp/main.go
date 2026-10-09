// Command sys-mcp is the homelab's MCP server: it resolves hosts, containers,
// clients, IPs, ports and VLANs out of this flake, and runs diagnostics
// against whichever machine owns the thing being asked about.
package main

import (
	"homelab/app"
	"homelab/sys-mcp/internal/serve"
)

// newApp wires the command line to the work. sys-mcp takes no flags: it is
// launched by an MCP client and configured entirely by environment
// (SYS_MCP_FLAKE_PATH, SYS_MCP_HOSTNAME, UNIFI_*, PROMETHEUS_URL, LOKI_URL),
// which internal/config reads directly under those names.
func newApp() app.App {
	return app.App{
		Name:  "sys-mcp",
		Short: "Resolve and diagnose this homelab over MCP",
		Long: "sys-mcp answers topology questions about this flake — which " +
			"host runs a container, what owns an IP or port, which VLAN " +
			"something is on — and runs diagnostics against the right " +
			"machine. It speaks JSON-RPC over stdio to an MCP client.",
		Run: serve.Run,
	}
}

func main() {
	newApp().Main()
}
