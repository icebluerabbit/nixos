package app

import (
	"sys-mcp/cache"
	"sys-mcp/config"
)

type AppContext struct {
	Cfg *config.Config
	IC  *cache.InfraCache
	HC  *cache.HostsCache
	SI  *cache.SourceIndex
}
