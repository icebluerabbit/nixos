// Package serve wires sys-mcp's caches and tools onto an MCP server and
// speaks JSON-RPC over stdio.
//
// stdout is the protocol here, not a product: every byte written to it is a
// JSON-RPC frame the client parses. Diagnostics go to the logger, which
// lib/obs points at stderr — a stray write to stdout corrupts the session
// rather than merely looking untidy, which is why forbidigo's ban on
// fmt.Print* matters more in this binary than in any other.
package serve

import (
	"context"
	"os"

	"github.com/mark3labs/mcp-go/server"

	hlapp "homelab/app"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
	"homelab/sys-mcp/internal/config"
	"homelab/sys-mcp/internal/tools"
)

// version is reported to MCP clients in the initialize handshake.
const version = "0.1.0"

// Run loads the configuration, prewarms the caches, registers every tool and
// serves until ctx is cancelled or stdin closes.
func Run(ctx context.Context, env *hlapp.Env) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ic := cache.NewInfraCache(cfg.FlakePath)
	ic.Prewarm()

	hc := cache.NewHostsCache(cfg.FlakePath)
	hc.Prewarm()

	alerts := cache.NewAlertsCache(cfg.FlakePath)
	alerts.Prewarm()

	ac := &app.AppContext{
		Cfg: cfg,
		IC:  ic,
		HC:  hc,
		SI:  cache.NewSourceIndex(cfg.FlakePath, ic),
		AC:  alerts,
		DC:  cache.NewDashboardCache(cfg.FlakePath),
	}

	s := server.NewMCPServer("sys-mcp", version,
		server.WithRecovery(),
	)

	tools.RegisterAll(s, ac)

	// NewStdioServer rather than ServeStdio: the latter installs its own
	// SIGTERM/SIGINT handler, and lib/app already owns signals for every
	// binary here. Listen returns when that context is cancelled or stdin
	// closes, which is the same shutdown either way.
	return server.NewStdioServer(s).Listen(ctx, os.Stdin, env.Stdout)
}
