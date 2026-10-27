package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/config"
)

// --- RegisterUnifiTools ---

func TestRegisterUnifiTools_Unconfigured(t *testing.T) {
	s := server.NewMCPServer("test", "0.0.0")
	ac := &app.AppContext{Cfg: &config.Config{}} // no URL or key
	RegisterUnifiTools(s, ac)                    // must not panic, must not register any tools
}

func TestRegisterUnifiTools_NoKeySkips(t *testing.T) {
	s := server.NewMCPServer("test", "0.0.0")
	ac := &app.AppContext{Cfg: &config.Config{UnifiURL: "https://10.100.10.1"}}
	RegisterUnifiTools(s, ac) // URL without key → skip
}

// --- rawGet ---

func unifiTestServer(t *testing.T, apiKey string, data any) (*httptest.Server, *unifiConn) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	conn := &unifiConn{
		rawURL: srv.URL,
		apiKey: apiKey,
		http:   srv.Client(),
		site:   "default",
	}
	return srv, conn
}

func TestRawGet_EnvelopeUnwrap(t *testing.T) {
	type item struct {
		Name string `json:"name"`
	}
	srv, conn := unifiTestServer(t, "secret", []item{{Name: "router"}})
	defer srv.Close()

	var result []item
	if err := conn.rawGet(context.Background(), "/api/test", &result); err != nil {
		t.Fatalf("rawGet error: %v", err)
	}
	if len(result) != 1 || result[0].Name != "router" {
		t.Errorf("got %+v", result)
	}
}

func TestRawGet_WrongAPIKey(t *testing.T) {
	srv, _ := unifiTestServer(t, "correct-key", nil)
	defer srv.Close()

	conn := &unifiConn{
		rawURL: srv.URL,
		apiKey: "wrong-key",
		http:   srv.Client(),
	}
	var result any
	if err := conn.rawGet(context.Background(), "/api/test", &result); err == nil {
		t.Error("expected error for wrong API key")
	}
}

func TestRawGet_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	conn := &unifiConn{rawURL: srv.URL, apiKey: "", http: srv.Client()}
	var result any
	if err := conn.rawGet(context.Background(), "/api/test", &result); err == nil {
		t.Error("expected error for 500 response")
	}
}

// --- fetchDeviceStats ---

func TestFetchDeviceStats_IndexedByMAC(t *testing.T) {
	up := int64(86400)
	stats := []deviceStat{
		{MAC: "aa:bb:cc:dd:ee:ff", Version: "7.0.1", Uptime: &up},
		{MAC: "11:22:33:44:55:66", Version: "6.5.0", Uptime: nil},
	}
	srv, conn := unifiTestServer(t, "key", stats)
	defer srv.Close()

	m, err := conn.fetchDeviceStats(context.Background())
	if err != nil {
		t.Fatalf("fetchDeviceStats: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
	if m["aa:bb:cc:dd:ee:ff"].Version != "7.0.1" {
		t.Errorf("version: got %q", m["aa:bb:cc:dd:ee:ff"].Version)
	}
	if m["aa:bb:cc:dd:ee:ff"].Uptime == nil || *m["aa:bb:cc:dd:ee:ff"].Uptime != 86400 {
		t.Error("uptime not parsed correctly")
	}
	if m["11:22:33:44:55:66"].Uptime != nil {
		t.Error("nil uptime should stay nil")
	}
}

func TestFetchDeviceStats_EmptyResponse(t *testing.T) {
	srv, conn := unifiTestServer(t, "key", []deviceStat{})
	defer srv.Close()

	m, err := conn.fetchDeviceStats(context.Background())
	if err != nil {
		t.Fatalf("fetchDeviceStats: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

// --- deviceStat JSON ---

func TestDeviceStat_JSONUnmarshal(t *testing.T) {
	raw := `{
		"mac": "aa:bb:cc:dd:ee:ff",
		"version": "7.0.1",
		"uptime": 3600,
		"num_sta": 12,
		"sys_stats": {
			"cpu": "15.5",
			"mem": "42.0",
			"loadavg_1": "0.12",
			"loadavg_5": "0.08"
		}
	}`
	var s deviceStat
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("MAC: got %q", s.MAC)
	}
	if s.Version != "7.0.1" {
		t.Errorf("Version: got %q", s.Version)
	}
	if s.Uptime == nil || *s.Uptime != 3600 {
		t.Errorf("Uptime: got %v", s.Uptime)
	}
	if s.NumSta == nil || *s.NumSta != 12 {
		t.Errorf("NumSta: got %v", s.NumSta)
	}
	if s.SysStats.CPU != "15.5" {
		t.Errorf("CPU: got %q", s.SysStats.CPU)
	}
	if s.SysStats.Loadavg1 != "0.12" {
		t.Errorf("Loadavg1: got %q", s.SysStats.Loadavg1)
	}
}

func TestDeviceStat_NullOptionals(t *testing.T) {
	raw := `{"mac": "aa:bb:cc:dd:ee:ff"}`
	var s deviceStat
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Uptime != nil {
		t.Error("Uptime should be nil when absent")
	}
	if s.NumSta != nil {
		t.Error("NumSta should be nil when absent")
	}
}
