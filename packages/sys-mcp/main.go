package main

import (
	"log"

	"github.com/mark3labs/mcp-go/server"
	"sys-mcp/app"
	"sys-mcp/cache"
	"sys-mcp/config"
	"sys-mcp/tools"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ic := cache.NewInfraCache(cfg.FlakePath)
	ic.Prewarm()

	hc := cache.NewHostsCache(cfg.FlakePath)
	hc.Prewarm()

	ac := &app.AppContext{
		Cfg: cfg,
		IC:  ic,
		HC:  hc,
		SI:  cache.NewSourceIndex(cfg.FlakePath, ic),
	}

	s := server.NewMCPServer("sys-mcp", "0.1.0",
		server.WithRecovery(),
	)

	tools.RegisterAll(s, ac)

	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
