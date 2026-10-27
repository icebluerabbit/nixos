// Package tools holds sys-mcp's MCP tool surface, one Register function per
// domain. The tool names and input schemas registered here are a contract with
// the client and with the skills that call them by name, so they change only
// deliberately.
package tools

import (
	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
)

// RegisterAll registers every tool sys-mcp serves.
func RegisterAll(s *server.MCPServer, ac *app.AppContext) {
	RegisterInfraTools(s, ac)
	RegisterSearchTools(s, ac)
	RegisterRuntimeTools(s, ac)
	RegisterUnifiTools(s, ac)
	RegisterPathTools(s, ac)
	RegisterProvisionTools(s, ac)
	RegisterObservabilityTools(s, ac)
}
