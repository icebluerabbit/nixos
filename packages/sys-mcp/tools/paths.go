package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"sys-mcp/app"
	"sys-mcp/cache"
)

func RegisterPathTools(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("resolve_path",
			mcp.WithDescription("Resolve a Nix file path to its topology context and return the file content. Returns JSON metadata (type, entity info, related file paths) followed by the raw file content. Accepts absolute paths (/etc/nixos/...) or flake-relative paths."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Path to a Nix file — absolute or flake-relative (e.g. /etc/nixos/modules/containers/bazarr.nix or modules/containers/bazarr.nix)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rawPath, err := req.RequireString("path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			relPath := toRelPath(rawPath, ac.Cfg.FlakePath)
			absPath := filepath.Join(ac.Cfg.FlakePath, relPath)

			// An aspect may be a directory rather than a single .nix — resolving
			// a container now legitimately yields one — so classify it and list
			// what it contains instead of trying to read it.
			if info, err := os.Stat(absPath); err == nil && info.IsDir() {
				entries, err := os.ReadDir(absPath)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("cannot list %s: %v", relPath, err)), nil
				}
				files := make([]string, 0, len(entries))
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".nix") {
						files = append(files, filepath.Join(relPath, e.Name()))
					}
				}
				meta := classifyPath(relPath, ac)
				metaJSON, _ := json.Marshal(meta)
				out := fmt.Sprintf("%s\n\n--- directory: %s ---\n%s", metaJSON, relPath, strings.Join(files, "\n"))
				return mcp.NewToolResultText(out), nil
			}

			content, err := os.ReadFile(absPath)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("cannot read %s: %v", relPath, err)), nil
			}

			meta := classifyPath(relPath, ac)
			metaJSON, _ := json.Marshal(meta)
			out := fmt.Sprintf("%s\n\n--- file: %s ---\n%s", metaJSON, relPath, content)
			return mcp.NewToolResultText(out), nil
		},
	)
}

// toRelPath strips the flake root prefix from an absolute path.
// If the path is already relative, returns it as-is.
func toRelPath(path, flakePath string) string {
	prefix := strings.TrimRight(flakePath, "/") + "/"
	if strings.HasPrefix(path, prefix) {
		return path[len(prefix):]
	}
	return strings.TrimLeft(path, "/")
}

type pathMeta struct {
	File     string          `json:"file"`
	Type     string          `json:"type"`
	Category string          `json:"category,omitempty"`
	Name     string          `json:"name,omitempty"`
	Entity   *resolvedEntity `json:"entity,omitempty"`
	Related  []relatedFile   `json:"related"`
}

type resolvedEntity struct {
	IP     string         `json:"ip,omitempty"`
	MAC    string         `json:"mac,omitempty"`
	Vlan   string         `json:"vlan,omitempty"`
	VlanID int            `json:"vlanId,omitempty"`
	Host   string         `json:"host,omitempty"`
	Ports  map[string]int `json:"ports,omitempty"`
	Subnet string         `json:"subnet,omitempty"`
	Domain string         `json:"domain,omitempty"`
}

type relatedFile struct {
	Path string `json:"path"`
	Role string `json:"role"`
}

func classifyPath(relPath string, ac *app.AppContext) pathMeta {
	stem := strings.TrimSuffix(filepath.Base(relPath), ".nix")
	meta := pathMeta{File: relPath, Related: []relatedFile{}}

	switch {
	case strings.HasPrefix(relPath, "modules/containers/"):
		meta.Type = "container"
		meta.Name = stem
		enrichContainer(&meta, stem, relPath, ac)

	case strings.HasPrefix(relPath, "modules/infra/clients/"):
		meta.Type = "infra-client"
		meta.Name = stem
		enrichClient(&meta, stem, relPath, ac)

	case strings.HasPrefix(relPath, "modules/infra/networks/"):
		meta.Type = "vlan"
		meta.Name = stem
		enrichVLAN(&meta, stem, relPath, ac)

	case strings.HasPrefix(relPath, "modules/infra/devices/"):
		meta.Type = "device"
		meta.Name = stem
		enrichDevice(&meta, stem, relPath, ac)

	case strings.HasPrefix(relPath, "modules/infra/wlans/"):
		meta.Type = "wlan"
		meta.Name = stem

	case strings.HasPrefix(relPath, "modules/infra/portforwards/"):
		meta.Type = "port-forward"
		meta.Name = stem

	default:
		if host, name, ok := parseHostContainerConfig(relPath); ok {
			meta.Type = "host-container-config"
			meta.Name = name
			enrichHostContainerConfig(&meta, name, host, relPath, ac)
		} else if strings.HasPrefix(relPath, "modules/hosts/") {
			meta.Type = "host-profile"
			parts := strings.SplitN(strings.TrimPrefix(relPath, "modules/hosts/"), "/", 2)
			if len(parts) > 0 {
				meta.Name = parts[0]
			}
		} else if strings.HasPrefix(relPath, "packages/") {
			meta.Type = "package"
			parts := strings.SplitN(strings.TrimPrefix(relPath, "packages/"), "/", 2)
			if len(parts) > 0 {
				meta.Category = parts[0]
			}
			meta.Name = stem
		} else {
			parts := strings.SplitN(relPath, "/", 3)
			if len(parts) >= 2 && parts[0] == "modules" {
				meta.Type = "aspect"
				meta.Category = parts[1]
				meta.Name = stem
			} else {
				meta.Type = "unknown"
				meta.Name = stem
			}
		}
	}

	return meta
}

// parseHostContainerConfig detects modules/hosts/{host}/containers/{name}[.nix]
func parseHostContainerConfig(relPath string) (host, name string, ok bool) {
	parts := strings.Split(relPath, "/")
	if len(parts) >= 5 &&
		parts[0] == "modules" && parts[1] == "hosts" && parts[3] == "containers" {
		return parts[2], strings.TrimSuffix(parts[4], ".nix"), true
	}
	return "", "", false
}

func enrichContainer(meta *pathMeta, name, relPath string, ac *app.AppContext) {
	hostName := containerHost(name, ac)

	clientKey := "container-" + name
	if infra, err := ac.IC.Get(); err == nil {
		if client, ok := infra.Clients[clientKey]; ok {
			ent := &resolvedEntity{MAC: client.Mac, Ports: client.Ports, Host: hostName}
			if client.Ip != nil {
				ent.IP = *client.Ip
				ent.Vlan = ipToVLAN(*client.Ip, infra)
			}
			meta.Entity = ent
		}
	}

	addIfExists(meta, relPath, "modules/infra/clients/"+clientKey+".nix",
		"infra client (MAC/IP/ports)", ac.Cfg.FlakePath)
	if hostName != "" {
		addIfExistsFileOrDir(meta, relPath, "modules/hosts/"+hostName+"/containers/"+name,
			"host container config", ac.Cfg.FlakePath)
	}
	if ac.SI != nil {
		// SI adds VLAN + any wlan/portforward links via reverse maps
		sp := ac.SI.ByContainer(name, hostName)
		addSourcePaths(meta, relPath, sp)
	}
}

func enrichClient(meta *pathMeta, key, relPath string, ac *app.AppContext) {
	infra, err := ac.IC.Get()
	if err != nil {
		return
	}
	client, ok := infra.Clients[key]
	if !ok {
		return
	}
	ent := &resolvedEntity{MAC: client.Mac, Ports: client.Ports}
	if client.Ip != nil {
		ent.IP = *client.Ip
		ent.Vlan = ipToVLAN(*client.Ip, infra)
	}
	meta.Entity = ent

	if ent.Vlan != "" {
		addIfExists(meta, relPath, "modules/infra/networks/"+ent.Vlan+".nix",
			"VLAN definition", ac.Cfg.FlakePath)
	}
	if strings.HasPrefix(key, "container-") {
		cName := strings.TrimPrefix(key, "container-")
		addIfExistsFileOrDir(meta, relPath, "modules/containers/"+cName,
			"container aspect", ac.Cfg.FlakePath)
		if h := containerHost(cName, ac); h != "" {
			addIfExistsFileOrDir(meta, relPath, "modules/hosts/"+h+"/containers/"+cName,
				"host container config", ac.Cfg.FlakePath)
		}
	}
}

func enrichVLAN(meta *pathMeta, name, relPath string, ac *app.AppContext) {
	if infra, err := ac.IC.Get(); err == nil {
		if v, ok := infra.Vlans[name]; ok {
			meta.Entity = &resolvedEntity{
				VlanID: v.VlanId,
				Subnet: v.Subnet,
				Domain: v.Domain,
			}
		}
	}
}

func enrichDevice(meta *pathMeta, key, relPath string, ac *app.AppContext) {
	if infra, err := ac.IC.Get(); err == nil {
		if d, ok := infra.Devices[key]; ok {
			meta.Entity = &resolvedEntity{MAC: d.Mac, IP: d.Ip}
		}
	}
}

func enrichHostContainerConfig(meta *pathMeta, name, host, relPath string, ac *app.AppContext) {
	meta.Category = host
	clientKey := "container-" + name
	if infra, err := ac.IC.Get(); err == nil {
		if client, ok := infra.Clients[clientKey]; ok {
			ent := &resolvedEntity{MAC: client.Mac, Ports: client.Ports, Host: host}
			if client.Ip != nil {
				ent.IP = *client.Ip
				ent.Vlan = ipToVLAN(*client.Ip, infra)
			}
			meta.Entity = ent
		}
	}
	addIfExistsFileOrDir(meta, relPath, "modules/containers/"+name,
		"container aspect", ac.Cfg.FlakePath)
	addIfExists(meta, relPath, "modules/infra/clients/"+clientKey+".nix",
		"infra client (MAC/IP/ports)", ac.Cfg.FlakePath)
}

// containerHost finds which host declares this container in HostsCache.
func containerHost(name string, ac *app.AppContext) string {
	if hosts, err := ac.HC.Get(); err == nil {
		for h, eval := range hosts {
			if _, ok := eval.Containers[name]; ok {
				return h
			}
		}
	}
	return ""
}

// ipToVLAN returns the VLAN name whose staticAddresses contains the given IP.
func ipToVLAN(ip string, infra *cache.InfraData) string {
	for name, v := range infra.Vlans {
		for _, addr := range v.StaticAddresses {
			if addr == ip {
				return name
			}
		}
	}
	return ""
}

// addSourcePaths adds non-nil SourcePaths entries to meta.Related, skipping the
// requested file and any paths already present (avoids duplicates from SI + addIfExists).
func addSourcePaths(meta *pathMeta, exclude string, sp cache.SourcePaths) {
	existing := make(map[string]bool, len(meta.Related)+1)
	existing[exclude] = true
	for _, r := range meta.Related {
		existing[r.Path] = true
	}
	add := func(p *string, role string) {
		if p != nil && *p != "" && !existing[*p] {
			meta.Related = append(meta.Related, relatedFile{Path: *p, Role: role})
			existing[*p] = true
		}
	}
	add(sp.ContainerAspect, "container aspect")
	add(sp.HostContainerConf, "host container config")
	add(sp.InfraVlan, "VLAN definition")
	add(sp.InfraWlan, "WLAN definition")
	add(sp.InfraPortForward, "port forward rule")
}

// addIfExists adds a related file entry only if the file exists on disk.
func addIfExists(meta *pathMeta, exclude, relPath, role, flakePath string) {
	if relPath == exclude {
		return
	}
	if cache.RelIfExists(flakePath, relPath) != "" {
		meta.Related = append(meta.Related, relatedFile{Path: relPath, Role: role})
	}
}

// addIfExistsFileOrDir is addIfExists for aspects that may be either a
// `<name>.nix` or a `<name>/` directory. `base` carries no extension.
func addIfExistsFileOrDir(meta *pathMeta, exclude, base, role, flakePath string) {
	p := cache.RelIfExistsFileOrDir(flakePath, base)
	if p == "" || p == exclude {
		return
	}
	meta.Related = append(meta.Related, relatedFile{Path: p, Role: role})
}
