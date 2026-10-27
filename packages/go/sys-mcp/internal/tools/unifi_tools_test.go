package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
	"homelab/sys-mcp/internal/config"
)

// controllerStub stands in for the UniFi controller. Every endpoint the tools
// touch answers with the same envelope the real API uses, keyed by the tail
// of the path so both the classic and the /proxy/network-prefixed styles hit
// the same payloads.
func controllerStub(t *testing.T) *httptest.Server {
	t.Helper()

	payloads := map[string]any{
		"stat/device": []map[string]any{{
			"name": "usw-pro-hd-24-poe", "mac": "74:fa:29:50:06:e6",
			"model": "USWProHD24PoE", "type": "usw", "ip": "10.100.10.2",
			"version": "7.0.0", "state": 1, "num_sta": 4,
		}},
		"stat/sta": []map[string]any{{
			"name": "desktop", "hostname": "desktop", "mac": "aa:bb:cc:dd:ee:ff",
			"ip": "10.10.20.5", "is_wired": true, "network": "trust",
		}},
		"rest/networkconf": []map[string]any{{"name": "services", "vlan": 140}},
		"rest/portforward": []map[string]any{{"name": "http", "dst_port": "80"}},
		"rest/wlanconf":    []map[string]any{{"name": "home", "enabled": true}},
		"list/wlanconf":    []map[string]any{{"name": "home", "enabled": true}},
		"rest/portconf":    []map[string]any{{"name": "trunk", "forward": "all"}},
	}

	// The firewall policies live on the v2 API, which answers with a bare
	// array rather than the classic meta/data envelope.
	v2Payloads := map[string]any{
		"firewall-policies": []map[string]any{{"name": "allow-dns", "enabled": true}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		for suffix, data := range v2Payloads {
			if strings.HasSuffix(r.URL.Path, suffix) {
				_ = json.NewEncoder(w).Encode(data)
				return
			}
		}

		for suffix, data := range payloads {
			if strings.HasSuffix(r.URL.Path, suffix) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"meta": map[string]any{"rc": "ok"},
					"data": data,
				})
				return
			}
		}
		// Health is what probeRawBase asks for; anything else is an empty
		// envelope, which is a valid answer for a controller with nothing of
		// that kind configured.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"meta": map[string]any{"rc": "ok"},
			"data": []any{},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// unifiServer registers the controller tools against the stub.
func unifiServer(t *testing.T) *server.MCPServer {
	t.Helper()
	root := t.TempDir()
	ic := cache.NewInfraCacheWithData(testData)
	ac := &app.AppContext{
		Cfg: &config.Config{
			Hostname:    "desktop",
			FlakePath:   root,
			UnifiURL:    controllerStub(t).URL,
			UnifiAPIKey: "test-key",
			UnifiSite:   "default",
		},
		IC: ic,
		HC: cache.NewHostsCacheWithData(cache.HostsData{"server": {}, "desktop": {}}),
		SI: cache.NewSourceIndex(root, ic),
	}
	s := server.NewMCPServer("test", "0.0.0")
	RegisterUnifiTools(s, ac)
	return s
}

// With a URL and a key configured, all nine controller tools register. This
// is the half of the conditional registration the unconfigured tests cannot
// reach.
func TestRegisterUnifiToolsRegistersCatalog(t *testing.T) {
	names := toolNames(t, unifiServer(t))

	for _, want := range []string{
		"list_unifi_devices", "get_unifi_device", "list_unifi_clients",
		"get_unifi_client", "list_unifi_networks", "list_unifi_port_forwards",
		"list_unifi_wlans", "list_unifi_firewall_policies",
		"list_unifi_port_profiles",
	} {
		if !has(names, want) {
			t.Errorf("tool %q is not registered; got %v", want, names)
		}
	}
}

// Every listing answers from the controller rather than erroring.
func TestUnifiListingTools(t *testing.T) {
	s := unifiServer(t)

	for _, tool := range []string{
		"list_unifi_devices", "list_unifi_clients", "list_unifi_networks",
		"list_unifi_port_forwards", "list_unifi_wlans",
		"list_unifi_firewall_policies", "list_unifi_port_profiles",
	} {
		t.Run(tool, func(t *testing.T) {
			out, isErr := callTool(t, s, tool, nil)
			if isErr {
				t.Fatalf("%s reported an error: %s", tool, out)
			}
			if out == "" {
				t.Errorf("%s returned nothing", tool)
			}
		})
	}
}

// The client listing filters by connection type, which is the one argument
// that changes what comes back.
func TestListUnifiClientsFiltersByType(t *testing.T) {
	s := unifiServer(t)

	out, isErr := callTool(t, s, "list_unifi_clients", map[string]any{"type": "wired"})
	if isErr {
		t.Fatalf("list_unifi_clients reported an error: %s", out)
	}
	if !strings.Contains(out, "desktop") {
		t.Errorf("wired client missing: %s", out)
	}

	out, isErr = callTool(t, s, "list_unifi_clients", map[string]any{"type": "wireless"})
	if isErr {
		t.Fatalf("list_unifi_clients reported an error: %s", out)
	}
	if strings.Contains(out, "desktop") {
		t.Errorf("wired client leaked into the wireless filter: %s", out)
	}
}

func TestGetUnifiDeviceTool(t *testing.T) {
	s := unifiServer(t)

	out, isErr := callTool(t, s, "get_unifi_device", map[string]any{"name": "usw-pro-hd-24-poe"})
	if isErr {
		t.Fatalf("get_unifi_device reported an error: %s", out)
	}
	if !strings.Contains(out, "usw-pro-hd-24-poe") {
		t.Errorf("device missing from result: %s", out)
	}

	if _, isErr := callTool(t, s, "get_unifi_device", map[string]any{"name": "nosuchdevice"}); !isErr {
		t.Error("an unknown device should be an error result")
	}
}

func TestGetUnifiClientTool(t *testing.T) {
	s := unifiServer(t)

	out, isErr := callTool(t, s, "get_unifi_client", map[string]any{"mac": "aa:bb:cc:dd:ee:ff"})
	if isErr {
		t.Fatalf("get_unifi_client reported an error: %s", out)
	}
	if !strings.Contains(out, "desktop") {
		t.Errorf("client missing from result: %s", out)
	}

	if _, isErr := callTool(t, s, "get_unifi_client", map[string]any{"mac": "00:00:00:00:00:00"}); !isErr {
		t.Error("an unknown MAC should be an error result")
	}
	if _, isErr := callTool(t, s, "get_unifi_client", map[string]any{}); !isErr {
		t.Error("a missing MAC should be an error result")
	}
}

// probeRawBase picks the /proxy/network prefix when the controller answers
// there — a UDM proxies the classic API — and falls back to the bare URL
// otherwise.
func TestProbeRawBase(t *testing.T) {
	proxying := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/proxy/network/") {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(proxying.Close)

	got := probeRawBase(t.Context(), proxying.URL, "default", "key", proxying.Client())
	if got != proxying.URL+"/proxy/network" {
		t.Errorf("proxied base = %q, want %q", got, proxying.URL+"/proxy/network")
	}

	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(plain.Close)

	if got := probeRawBase(t.Context(), plain.URL, "default", "key", plain.Client()); got != plain.URL {
		t.Errorf("plain base = %q, want %q", got, plain.URL)
	}
}

// An unreachable controller must not take the process down at registration
// time; the tools simply do not appear.
func TestProbeRawBaseFallsBackWhenUnreachable(t *testing.T) {
	const addr = "http://127.0.0.1:1"

	if got := probeRawBase(t.Context(), addr, "default", "key", http.DefaultClient); got != addr {
		t.Errorf("unreachable base = %q, want %q", got, addr)
	}
}
