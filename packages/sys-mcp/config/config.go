package config

import (
	"fmt"
	"os"
)

type Config struct {
	FlakePath     string
	Hostname      string
	UnifiURL      string
	UnifiAPIKey   string
	UnifiSite     string
	PrometheusURL string
	LokiURL       string
}

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
