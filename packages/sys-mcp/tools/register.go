package tools

import (
	"github.com/mark3labs/mcp-go/server"
	"sys-mcp/app"
)

func RegisterAll(s *server.MCPServer, ac *app.AppContext) {
	RegisterInfraTools(s, ac)
	RegisterSearchTools(s, ac)
	RegisterRuntimeTools(s, ac)
	RegisterUnifiTools(s, ac)
	RegisterPathTools(s, ac)
	RegisterProvisionTools(s, ac)
}
