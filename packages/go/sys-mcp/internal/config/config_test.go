package config

import "testing"

// The flake path is the one setting without a fallback: every cache evaluates
// against it, so a missing one is a startup error rather than a degraded mode.
func TestLoadRequiresFlakePath(t *testing.T) {
	t.Setenv("SYS_MCP_FLAKE_PATH", "")
	t.Setenv("FLAKE", "")
	t.Setenv("NH_FLAKE", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load succeeded with no flake path set")
	}
}

// The three names are tried in order, so a session that exports FLAKE for nh
// needs no sys-mcp-specific variable.
func TestLoadFlakePathPrecedence(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "sys-mcp wins",
			env:  map[string]string{"SYS_MCP_FLAKE_PATH": "/a", "FLAKE": "/b", "NH_FLAKE": "/c"},
			want: "/a",
		},
		{
			name: "FLAKE next",
			env:  map[string]string{"SYS_MCP_FLAKE_PATH": "", "FLAKE": "/b", "NH_FLAKE": "/c"},
			want: "/b",
		},
		{
			name: "NH_FLAKE last",
			env:  map[string]string{"SYS_MCP_FLAKE_PATH": "", "FLAKE": "", "NH_FLAKE": "/c"},
			want: "/c",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			t.Setenv("SYS_MCP_HOSTNAME", "desktop")

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.FlakePath != tc.want {
				t.Errorf("FlakePath = %q, want %q", cfg.FlakePath, tc.want)
			}
		})
	}
}

// Every other setting is optional and passed straight through, except the
// UniFi site, which defaults rather than being empty.
func TestLoadPassesEnvironmentThrough(t *testing.T) {
	t.Setenv("SYS_MCP_FLAKE_PATH", "/projects/nixos")
	t.Setenv("SYS_MCP_HOSTNAME", "server")
	t.Setenv("UNIFI_URL", "https://10.0.0.1")
	t.Setenv("UNIFI_API_KEY", "key")
	t.Setenv("UNIFI_SITE", "other")
	t.Setenv("PROMETHEUS_URL", "http://prom:9090")
	t.Setenv("LOKI_URL", "http://loki:3100")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := Config{
		FlakePath:     "/projects/nixos",
		Hostname:      "server",
		UnifiURL:      "https://10.0.0.1",
		UnifiAPIKey:   "key",
		UnifiSite:     "other",
		PrometheusURL: "http://prom:9090",
		LokiURL:       "http://loki:3100",
	}
	if *cfg != want {
		t.Errorf("config = %+v, want %+v", *cfg, want)
	}
}

// An unset site is "default", which is what a single-site controller calls
// its only site.
func TestLoadDefaultsUnifiSite(t *testing.T) {
	t.Setenv("SYS_MCP_FLAKE_PATH", "/projects/nixos")
	t.Setenv("SYS_MCP_HOSTNAME", "server")
	t.Setenv("UNIFI_SITE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.UnifiSite != "default" {
		t.Errorf("UnifiSite = %q, want default", cfg.UnifiSite)
	}
}

// With no SYS_MCP_HOSTNAME the machine's own name is used, which is how the
// local/remote decision is made for diagnostics.
func TestLoadFallsBackToMachineHostname(t *testing.T) {
	t.Setenv("SYS_MCP_FLAKE_PATH", "/projects/nixos")
	t.Setenv("SYS_MCP_HOSTNAME", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Hostname == "" {
		t.Error("Hostname is empty; os.Hostname should have supplied one")
	}
}
