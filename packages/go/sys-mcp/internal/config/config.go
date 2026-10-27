// Package config reads sys-mcp's settings from the environment its MCP client
// sets — see the sys-mcp entry in modules/cli/mcp.nix. There are no flags: the
// server is launched by a client, not by a person.
package config

import (
	"fmt"
	"os"
)

// Config is the resolved environment sys-mcp runs against.
type Config struct {
	FlakePath     string
	Hostname      string
	UnifiURL      string
	UnifiAPIKey   string
	UnifiSite     string
	PrometheusURL string
	LokiURL       string
}

// Load resolves the configuration from the environment. It fails only when
// the flake path cannot be determined; every other setting is optional and an
// unset one disables the tools that need it.
func Load() (*Config, error) {
	flake, err := resolveFlakePath()
	if err != nil {
		return nil, err
	}

	hostname, err := resolveHostname()
	if err != nil {
		return nil, err
	}

	site := os.Getenv("UNIFI_SITE")
	if site == "" {
		site = "default"
	}

	return &Config{
		FlakePath:     flake,
		Hostname:      hostname,
		UnifiURL:      os.Getenv("UNIFI_URL"),
		UnifiAPIKey:   os.Getenv("UNIFI_API_KEY"),
		UnifiSite:     site,
		PrometheusURL: os.Getenv("PROMETHEUS_URL"),
		LokiURL:       os.Getenv("LOKI_URL"),
	}, nil
}

func resolveFlakePath() (string, error) {
	for _, key := range []string{"SYS_MCP_FLAKE_PATH", "FLAKE", "NH_FLAKE"} {
		if v := os.Getenv(key); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("flake path not set: export FLAKE or NH_FLAKE")
}

func resolveHostname() (string, error) {
	if v := os.Getenv("SYS_MCP_HOSTNAME"); v != "" {
		return v, nil
	}
	return os.Hostname()
}
