package tools

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	unificlient "github.com/ubiquiti-community/go-unifi/unifi"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
)

// unifiConn holds a live UniFi API client, site name, a raw HTTP client for
// runtime stats not covered by go-unifi's typed structs, and a SourceIndex
// for enriching responses with Nix source file paths.

type unifiConn struct {
	site   string
	apiKey string
	http   *http.Client
	si     *cache.SourceIndex

	// Dialled on first use rather than at registration: logging in to the
	// controller is a network round trip, and doing it while wiring up tools
	// meant an unreachable controller hung the MCP handshake itself — the
	// laptop off the homelab network never finished starting sys-mcp. Guarded
	// by a mutex and not sync.Once so a failure is retried on the next call;
	// Once would cache "unreachable" for the life of the process, and this
	// host moves between networks.
	baseURL string
	mu      sync.Mutex
	c       *unificlient.ApiClient
	rawURL  string
}

// api returns the controller client, dialling it on first use. The caller's
// context carries the request deadline, so an unreachable controller fails
// that one tool call instead of blocking startup.
func (conn *unifiConn) api(ctx context.Context) (*unificlient.ApiClient, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.c != nil {
		return conn.c, nil
	}

	// go-unifi defaults to a long timeout and several retries, each of which
	// waits out a full connect against an unreachable controller. Off-LAN
	// that turned one tool call into minutes, so cap both.
	timeoutSeconds := 10
	retryMax := 1
	client, err := unificlient.New(ctx, &unificlient.Config{
		BaseURL:        conn.baseURL,
		APIKey:         conn.apiKey,
		AllowInsecure:  true,
		TimeoutSeconds: &timeoutSeconds,
		RetryMax:       &retryMax,
	})
	if err != nil {
		return nil, fmt.Errorf("unifi controller %s: %w", conn.baseURL, err)
	}

	conn.c = client
	// Only meaningful once the controller answered, and rawGet needs it.
	conn.rawURL = probeRawBase(ctx, conn.baseURL, conn.site, conn.apiKey, conn.http)
	return conn.c, nil
}

// raw returns the base URL for classic API paths, dialling if needed so the
// /proxy/network probe has happened. An already-known base is returned as-is:
// the stats endpoints are plain HTTP and need no client, so a caller that
// supplied the base itself never pays for a login.
func (conn *unifiConn) raw(ctx context.Context) (string, error) {
	conn.mu.Lock()
	if conn.rawURL != "" {
		defer conn.mu.Unlock()
		return conn.rawURL, nil
	}
	conn.mu.Unlock()

	if _, err := conn.api(ctx); err != nil {
		return "", err
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()
	return conn.rawURL, nil
}

// clientStat maps the raw UniFi sta endpoint. go-unifi's generated ClientInfo
// has ChannelWidth as string but the API returns a number, causing unmarshal
// errors. Raw struct avoids the type mismatch entirely.
type clientStat struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
	Mac      string `json:"mac"`
	IsWired  bool   `json:"is_wired"`
	Essid    string `json:"essid"`
	ApMac    string `json:"ap_mac"`
	SwPort   *int64 `json:"sw_port"`
	Signal   *int64 `json:"signal"`
	TxRate   *int64 `json:"tx_rate"`
	RxRate   *int64 `json:"rx_rate"`
	TxBytes  *int64 `json:"tx_bytes"`
	RxBytes  *int64 `json:"rx_bytes"`
	Uptime   *int64 `json:"uptime"`
	Oui      string `json:"oui"`
}

// devicePort is the live per-port state from stat/device.
//
// Decoded here rather than read off unificlient.Device.PortTable: go-unifi
// v1.33.42 has an inverted error check in DevicePortTable.UnmarshalJSON —
// `if v, err := aux.PortIdx.Int64(); err != nil { dst.PortIdx = v }` — so every
// field routed through that custom unmarshaler (port_idx, speed, rx/tx bytes and
// errors, satisfaction) is left at zero. stat/device carries the same data, so
// decoding it straight avoids the bug entirely.
type devicePort struct {
	PortIdx         int64  `json:"port_idx"`
	Name            string `json:"name"`
	Up              bool   `json:"up"`
	Speed           int64  `json:"speed"`
	PoeEnable       bool   `json:"poe_enable"`
	PoePower        string `json:"poe_power"`
	RxBytes         int64  `json:"rx_bytes"`
	TxBytes         int64  `json:"tx_bytes"`
	RxErrors        int64  `json:"rx_errors"`
	TxErrors        int64  `json:"tx_errors"`
	NativeNetworkID string `json:"native_networkconf_id"`
	IsUplink        bool   `json:"is_uplink"`
	Satisfaction    int64  `json:"satisfaction"`
	OpMode          string `json:"op_mode"`
}

// devicePortOverride is the declared config for a port. `port_profile_id` is the
// controller object id, which joins to infra.ports.<key>.unifiId.
type devicePortOverride struct {
	PortIdx          int64   `json:"port_idx"`
	Name             string  `json:"name"`
	PortProfileID    string  `json:"portconf_id"`
	OpMode           string  `json:"op_mode"`
	AggregateMembers []int64 `json:"aggregate_members"`
}

type deviceStat struct {
	MAC      string `json:"mac"`
	Version  string `json:"version"`
	Uptime   *int64 `json:"uptime"`
	NumSta   *int64 `json:"num_sta"`
	SysStats struct {
		CPU      string `json:"cpu"`
		Mem      string `json:"mem"`
		Loadavg1 string `json:"loadavg_1"`
		Loadavg5 string `json:"loadavg_5"`
	} `json:"sys_stats"`
	PortTable     []devicePort         `json:"port_table"`
	PortOverrides []devicePortOverride `json:"port_overrides"`
}

func (conn *unifiConn) rawGet(ctx context.Context, path string, out any) error {
	base, err := conn.raw(ctx)
	if err != nil {
		return err
	}
	url := strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", conn.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := conn.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	return json.Unmarshal(envelope.Data, out)
}

func (conn *unifiConn) fetchDeviceStats(ctx context.Context) (map[string]deviceStat, error) {
	var stats []deviceStat
	if err := conn.rawGet(ctx, fmt.Sprintf("api/s/%s/stat/device", conn.site), &stats); err != nil {
		return nil, err
	}
	m := make(map[string]deviceStat, len(stats))
	for _, s := range stats {
		m[s.MAC] = s
	}
	return m, nil
}

// probeRawBase detects whether the UDM requires a /proxy/network prefix for
// classic API paths (api/s/{site}/...). UDM devices proxy the old API there.
func probeRawBase(ctx context.Context, baseURL, site, apiKey string, httpClient *http.Client) string {
	probe := strings.TrimRight(baseURL, "/") + "/proxy/network/api/s/" + site + "/stat/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probe, http.NoBody)
	if err != nil {
		return baseURL
	}
	req.Header.Set("X-API-Key", apiKey)
	resp, err := httpClient.Do(req)
	if err != nil {
		return baseURL
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return strings.TrimRight(baseURL, "/") + "/proxy/network"
	}
	return baseURL
}

// RegisterUnifiTools registers the controller lookups. They register only
// when a URL and an API key are configured, so an unconfigured sys-mcp serves
// the rest of its catalog rather than failing every call.
func RegisterUnifiTools(s *server.MCPServer, ac *app.AppContext) {
	if ac.Cfg.UnifiURL == "" || ac.Cfg.UnifiAPIKey == "" {
		return
	}

	httpClient := &http.Client{
		// Without a timeout an off-LAN request waits on the OS connect
		// timeout, which is long enough to look like a hang.
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			// The controller serves its own certificate on the LAN.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // self-signed controller cert
		},
	}

	conn := &unifiConn{
		site:    ac.Cfg.UnifiSite,
		apiKey:  ac.Cfg.UnifiAPIKey,
		si:      ac.SI,
		http:    httpClient,
		baseURL: ac.Cfg.UnifiURL,
	}

	s.AddTool(
		mcp.NewTool("list_unifi_devices",
			mcp.WithDescription("List all UniFi network devices (UDM, switches, APs) with model, state, IP, MAC, firmware version, uptime, and connected client count."),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			api, err := conn.api(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			devices, err := api.ListDevice(ctx, conn.site)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			stats, _ := conn.fetchDeviceStats(ctx)

			type row struct {
				Name      string            `json:"name"`
				Model     string            `json:"model"`
				Type      string            `json:"type"`
				State     string            `json:"state"`
				IP        string            `json:"ip"`
				MAC       string            `json:"mac"`
				Version   string            `json:"version,omitempty"`
				UptimeSec *int64            `json:"uptime_sec,omitempty"`
				NumSta    *int64            `json:"connected_clients,omitempty"`
				Sources   cache.SourcePaths `json:"sources,omitempty"`
			}
			rows := make([]row, 0, len(devices))
			for _, d := range devices {
				r := row{
					Name:    d.Name,
					Model:   d.Model,
					Type:    d.Type,
					State:   d.State.String(),
					IP:      d.IP,
					MAC:     d.MAC,
					Sources: conn.si.ByMAC(d.MAC),
				}
				if st, ok := stats[d.MAC]; ok {
					r.Version = st.Version
					r.UptimeSec = st.Uptime
					r.NumSta = st.NumSta
				}
				rows = append(rows, r)
			}
			return jsonResult(rows)
		},
	)

	s.AddTool(
		mcp.NewTool("get_unifi_device",
			mcp.WithDescription("Full detail for a single UniFi device including per-port table (speed, PoE, rx/tx bytes, errors, native VLAN). Lookup by name or MAC."),
			mcp.WithString("name", mcp.Description("Device name (e.g. USW-Pro-HD-24-PoE)")),
			mcp.WithString("mac", mcp.Description("Device MAC address")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args, _ := req.Params.Arguments.(map[string]any)
			nameFilter, _ := args["name"].(string)
			macFilter, _ := args["mac"].(string)
			if nameFilter == "" && macFilter == "" {
				return mcp.NewToolResultError("provide name or mac"), nil
			}

			api, err := conn.api(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			devices, err := api.ListDevice(ctx, conn.site)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var found *unificlient.Device
			for i, d := range devices {
				if (nameFilter != "" && strings.EqualFold(d.Name, nameFilter)) ||
					(macFilter != "" && strings.EqualFold(d.MAC, macFilter)) {
					found = &devices[i]
					break
				}
			}
			if found == nil {
				return mcp.NewToolResultError(fmt.Sprintf("device %q not found", nameFilter+macFilter)), nil
			}

			stats, _ := conn.fetchDeviceStats(ctx)
			st := stats[found.MAC]

			type portRow struct {
				Idx              int64            `json:"port"`
				Name             string           `json:"name,omitempty"`
				Up               bool             `json:"up"`
				Speed            int64            `json:"speed_mbps"`
				PoE              bool             `json:"poe"`
				PoePower         string           `json:"poe_power_w,omitempty"`
				RxBytes          int64            `json:"rx_bytes"`
				TxBytes          int64            `json:"tx_bytes"`
				RxErrors         int64            `json:"rx_errors"`
				TxErrors         int64            `json:"tx_errors"`
				NativeNetworkID  string           `json:"native_vlan_id,omitempty"`
				IsUplink         bool             `json:"is_uplink"`
				Satisfaction     int64            `json:"satisfaction"`
				OpMode           string           `json:"op_mode,omitempty"`
				AggregateMembers []int64          `json:"aggregate_members,omitempty"`
				PortProfile      string           `json:"port_profile,omitempty"`
				Peers            []cache.PortPeer `json:"peers,omitempty"`
			}

			infra, _ := ac.IC.Get()
			overrides := make(map[int64]devicePortOverride, len(st.PortOverrides))
			for _, o := range st.PortOverrides {
				overrides[o.PortIdx] = o
			}

			ports := make([]portRow, 0, len(st.PortTable))
			for _, p := range st.PortTable {
				r := portRow{
					Idx:             p.PortIdx,
					Name:            p.Name,
					Up:              p.Up,
					Speed:           p.Speed,
					PoE:             p.PoeEnable,
					PoePower:        p.PoePower,
					RxBytes:         p.RxBytes,
					TxBytes:         p.TxBytes,
					RxErrors:        p.RxErrors,
					TxErrors:        p.TxErrors,
					NativeNetworkID: p.NativeNetworkID,
					IsUplink:        p.IsUplink,
					Satisfaction:    p.Satisfaction,
					OpMode:          p.OpMode,
				}
				// The port profile is what pairs this port with whatever is
				// plugged into it — a port with no profile has no resolvable peer.
				if o, ok := overrides[p.PortIdx]; ok && infra != nil {
					r.AggregateMembers = o.AggregateMembers
					if key := cache.PortKeyForUnifiId(o.PortProfileID, infra); key != "" {
						r.PortProfile = key
						r.Peers = cache.PeersOfPort(key, "", infra)
					}
				}
				ports = append(ports, r)
			}

			result := map[string]any{
				"name":              found.Name,
				"model":             found.Model,
				"type":              found.Type,
				"state":             found.State.String(),
				"ip":                found.IP,
				"mac":               found.MAC,
				"version":           st.Version,
				"uptime_sec":        st.Uptime,
				"connected_clients": st.NumSta,
				"cpu_pct":           st.SysStats.CPU,
				"mem_pct":           st.SysStats.Mem,
				"loadavg_1m":        st.SysStats.Loadavg1,
				"loadavg_5m":        st.SysStats.Loadavg5,
				"ports":             ports,
				"sources":           conn.si.ByMAC(found.MAC),
			}
			return jsonResult(result)
		},
	)

	s.AddTool(
		mcp.NewTool("list_unifi_clients",
			mcp.WithDescription("List all currently active network clients: hostname, IP, MAC, wired/wireless, SSID, AP or switch port, signal, uptime, throughput. Filter by connection type."),
			mcp.WithString("type", mcp.Description("Filter by connection type: wired or wireless. Omit for all.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var clients []clientStat
			if err := conn.rawGet(ctx, fmt.Sprintf("api/s/%s/stat/sta", conn.site), &clients); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			typeFilter := ""
			if args, ok := req.Params.Arguments.(map[string]any); ok {
				typeFilter, _ = args["type"].(string)
			}

			type row struct {
				Name       string            `json:"name,omitempty"`
				Hostname   string            `json:"hostname,omitempty"`
				IP         string            `json:"ip,omitempty"`
				MAC        string            `json:"mac"`
				IsWired    bool              `json:"is_wired"`
				SSID       string            `json:"ssid,omitempty"`
				APMac      string            `json:"ap_uplink_mac,omitempty"`
				SwPort     *int64            `json:"switch_port,omitempty"`
				Signal     *int64            `json:"signal_dbm,omitempty"`
				TxRateKbps *int64            `json:"tx_rate_kbps,omitempty"`
				RxRateKbps *int64            `json:"rx_rate_kbps,omitempty"`
				TxBytes    *int64            `json:"tx_bytes,omitempty"`
				RxBytes    *int64            `json:"rx_bytes,omitempty"`
				UptimeSec  *int64            `json:"uptime_sec,omitempty"`
				OUI        string            `json:"oui,omitempty"`
				Sources    cache.SourcePaths `json:"sources,omitempty"`
			}

			rows := make([]row, 0, len(clients))
			for _, c := range clients {
				if typeFilter == "wired" && !c.IsWired {
					continue
				}
				if typeFilter == "wireless" && c.IsWired {
					continue
				}
				rows = append(rows, row{
					Name:       c.Name,
					Hostname:   c.Hostname,
					IP:         c.IP,
					MAC:        c.Mac,
					IsWired:    c.IsWired,
					SSID:       c.Essid,
					APMac:      c.ApMac,
					SwPort:     c.SwPort,
					Signal:     c.Signal,
					TxRateKbps: c.TxRate,
					RxRateKbps: c.RxRate,
					TxBytes:    c.TxBytes,
					RxBytes:    c.RxBytes,
					UptimeSec:  c.Uptime,
					OUI:        c.Oui,
					Sources:    conn.si.ByMAC(c.Mac),
				})
			}
			return jsonResult(rows)
		},
	)

	s.AddTool(
		mcp.NewTool("get_unifi_client",
			mcp.WithDescription("Full runtime detail for a single active network client by MAC address."),
			mcp.WithString("mac", mcp.Required(), mcp.Description("Client MAC address (e.g. aa:bb:cc:dd:ee:ff)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			mac, err := req.RequireString("mac")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			var clients []clientStat
			if err := conn.rawGet(ctx, fmt.Sprintf("api/s/%s/stat/sta", conn.site), &clients); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			var c *clientStat
			for i := range clients {
				if strings.EqualFold(clients[i].Mac, mac) {
					c = &clients[i]
					break
				}
			}
			if c == nil {
				return mcp.NewToolResultError(fmt.Sprintf("client %q not found", mac)), nil
			}
			return jsonResult(map[string]any{
				"name":          c.Name,
				"hostname":      c.Hostname,
				"ip":            c.IP,
				"mac":           c.Mac,
				"is_wired":      c.IsWired,
				"ssid":          c.Essid,
				"ap_uplink_mac": c.ApMac,
				"switch_port":   c.SwPort,
				"signal_dbm":    c.Signal,
				"tx_rate_kbps":  c.TxRate,
				"rx_rate_kbps":  c.RxRate,
				"tx_bytes":      c.TxBytes,
				"rx_bytes":      c.RxBytes,
				"uptime_sec":    c.Uptime,
				"oui":           c.Oui,
				"sources":       conn.si.ByMAC(c.Mac),
			})
		},
	)

	s.AddTool(
		mcp.NewTool("list_unifi_networks",
			mcp.WithDescription("List all configured UniFi networks/VLANs: name, VLAN ID, subnet, purpose, DHCP state."),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			api, err := conn.api(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			networks, err := api.ListNetwork(ctx, conn.site)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			type row struct {
				Name        string            `json:"name"`
				VLAN        *int64            `json:"vlan_id,omitempty"`
				VLANEnabled bool              `json:"vlan_enabled"`
				Subnet      *string           `json:"subnet,omitempty"`
				Purpose     string            `json:"purpose"`
				DHCPEnabled bool              `json:"dhcp_enabled"`
				Enabled     bool              `json:"enabled"`
				Sources     cache.SourcePaths `json:"sources,omitempty"`
			}
			rows := make([]row, 0, len(networks))
			for _, n := range networks {
				name := ""
				if n.Name != nil {
					name = *n.Name
				}
				rows = append(rows, row{
					Name:        name,
					VLAN:        n.VLAN,
					VLANEnabled: n.VLANEnabled,
					Subnet:      n.IPSubnet,
					Purpose:     n.Purpose,
					DHCPEnabled: n.DHCPDEnabled,
					Enabled:     n.Enabled,
					Sources:     conn.si.ByVLANID(n.VLAN),
				})
			}
			return jsonResult(rows)
		},
	)

	s.AddTool(
		mcp.NewTool("list_unifi_port_forwards",
			mcp.WithDescription("List all UniFi port forward rules: name, protocol, WAN port, forward IP/port, enabled state."),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			api, err := conn.api(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fwds, err := api.ListPortForward(ctx, conn.site)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			type row struct {
				Name    string            `json:"name"`
				Proto   string            `json:"protocol"`
				WanPort string            `json:"wan_port"`
				FwdIP   string            `json:"forward_ip"`
				FwdPort string            `json:"forward_port"`
				Src     string            `json:"source_restrict,omitempty"`
				Enabled bool              `json:"enabled"`
				Sources cache.SourcePaths `json:"sources,omitempty"`
			}
			rows := make([]row, 0, len(fwds))
			for _, f := range fwds {
				rows = append(rows, row{
					Name:    f.Name,
					Proto:   f.Proto,
					WanPort: f.DstPort,
					FwdIP:   f.Fwd,
					FwdPort: f.FwdPort,
					Src:     f.Src,
					Enabled: f.Enabled,
					Sources: conn.si.ByPortForward(f.Name),
				})
			}
			return jsonResult(rows)
		},
	)

	s.AddTool(
		mcp.NewTool("list_unifi_wlans",
			mcp.WithDescription("List all UniFi WiFi networks (SSIDs): name, enabled, security, bands, network association, guest status."),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			api, err := conn.api(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			wlans, err := api.ListWLAN(ctx, conn.site)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			type row struct {
				Name      string            `json:"name"`
				Enabled   bool              `json:"enabled"`
				Security  string            `json:"security"`
				Band      string            `json:"band,omitempty"`
				NetworkID string            `json:"network_id,omitempty"`
				HideSSID  bool              `json:"hide_ssid"`
				IsGuest   bool              `json:"is_guest"`
				Sources   cache.SourcePaths `json:"sources,omitempty"`
			}
			rows := make([]row, 0, len(wlans))
			for _, w := range wlans {
				rows = append(rows, row{
					Name:      w.Name,
					Enabled:   w.Enabled,
					Security:  w.Security,
					Band:      w.WLANBand,
					NetworkID: w.NetworkID,
					HideSSID:  w.HideSSID,
					IsGuest:   w.IsGuest,
					Sources:   conn.si.ByWLAN(w.Name),
				})
			}
			return jsonResult(rows)
		},
	)

	s.AddTool(
		mcp.NewTool("list_unifi_firewall_policies",
			mcp.WithDescription("List all UniFi firewall policies: name, action, source, destination, enabled state."),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			api, err := conn.api(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			policies, err := api.ListFirewallPolicy(ctx, conn.site)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			type row struct {
				Name        string `json:"name"`
				Action      string `json:"action"`
				Enabled     bool   `json:"enabled"`
				IPVersion   string `json:"ip_version,omitempty"`
				Description string `json:"description,omitempty"`
			}
			rows := make([]row, 0, len(policies))
			for _, p := range policies {
				rows = append(rows, row{
					Name:        p.Name,
					Action:      p.Action,
					Enabled:     p.Enabled,
					IPVersion:   p.IPVersion,
					Description: p.Description,
				})
			}
			return jsonResult(rows)
		},
	)

	s.AddTool(
		mcp.NewTool("list_unifi_port_profiles",
			mcp.WithDescription("List all UniFi switch port profiles: name, native VLAN, forward mode, isolation, PoE enabled."),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			api, err := conn.api(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			profiles, err := api.ListPortProfile(ctx, conn.site)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			type row struct {
				Name            string `json:"name"`
				Forward         string `json:"forward,omitempty"`
				NativeNetworkID string `json:"native_vlan_id,omitempty"`
				Isolation       bool   `json:"isolation"`
			}
			rows := make([]row, 0, len(profiles))
			for _, p := range profiles {
				rows = append(rows, row{
					Name:            p.Name,
					Forward:         p.Forward,
					NativeNetworkID: p.NATiveNetworkID,
					Isolation:       p.Isolation,
				})
			}
			return jsonResult(rows)
		},
	)
}
