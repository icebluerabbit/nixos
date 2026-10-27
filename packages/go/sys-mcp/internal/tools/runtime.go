package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
)

// RegisterRuntimeTools registers the live diagnostics — status, journals and
// exec — each of which resolves its target's host and SSHes there.
func RegisterRuntimeTools(s *server.MCPServer, ac *app.AppContext) {
	s.AddTool(
		mcp.NewTool("container_status",
			mcp.WithDescription("Get the systemd status of a container's host-level unit (container@<name>.service). SSHes to the correct host automatically via ~/.ssh/config if not local."),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name (e.g. grafana, radarr)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("container")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			entity, err := resolveContainer(name, ac)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			out := runcmd(entity.Host.IsLocal, entity.Host.SshTarget,
				"systemctl", "status", fmt.Sprintf("container@%s.service", name), "--no-pager")
			return mcp.NewToolResultText(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("container_journal",
			mcp.WithDescription("Read journal logs from inside a container via nixos-container. SSHes to the correct host automatically if not local."),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name (e.g. grafana, radarr)")),
			mcp.WithString("unit", mcp.Description("Systemd unit to filter by (e.g. grafana.service). Omit for all units.")),
			mcp.WithNumber("lines", mcp.Description("Number of lines to return (default: 100)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("container")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			entity, err := resolveContainer(name, ac)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			jArgs := []string{"journalctl", "--no-pager"}
			if args, ok := req.Params.Arguments.(map[string]any); ok {
				if unit, _ := args["unit"].(string); unit != "" {
					jArgs = append(jArgs, "-u", unit)
				}
				lines := 100
				if raw, ok := args["lines"]; ok {
					if n, err := strconv.Atoi(fmt.Sprintf("%v", raw)); err == nil && n > 0 {
						lines = n
					}
				}
				jArgs = append(jArgs, "-n", strconv.Itoa(lines))
			}

			args := append([]string{"sudo", "nixos-container", "run", name, "--"}, jArgs...)
			return mcp.NewToolResultText(runcmd(entity.Host.IsLocal, entity.Host.SshTarget, args...)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("container_exec",
			mcp.WithDescription("Run a command inside a container via nixos-container. SSHes to the correct host automatically if not local. Command is passed to sh -c inside the container."),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name (e.g. grafana, radarr)")),
			mcp.WithString("command", mcp.Required(), mcp.Description("Command to run inside the container (e.g. systemctl status grafana.service)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("container")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			command, err := req.RequireString("command")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			entity, err := resolveContainer(name, ac)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			args := []string{"sudo", "nixos-container", "run", name, "--", "sh", "-c", command}
			return mcp.NewToolResultText(runcmd(entity.Host.IsLocal, entity.Host.SshTarget, args...)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("container_list",
			mcp.WithDescription("List runtime container states (running/stopped/failed) on a host via systemd units. SSHes automatically if not local."),
			mcp.WithString("host", mcp.Description("Host name (e.g. server). Defaults to current host if it has containers.")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			hosts, err := ac.HC.Get()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if _, err := ac.IC.Get(); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			hostName := ""
			if args, ok := req.Params.Arguments.(map[string]any); ok {
				hostName, _ = args["host"].(string)
			}
			if hostName == "" {
				if eval, ok := hosts[ac.Cfg.Hostname]; ok && len(eval.Containers) > 0 {
					hostName = ac.Cfg.Hostname
				} else {
					names := make([]string, 0, len(hosts))
					for h := range hosts {
						names = append(names, h)
					}
					return mcp.NewToolResultError(fmt.Sprintf(
						"host not specified; known hosts: %s", strings.Join(names, ", "))), nil
				}
			}
			if _, ok := hosts[hostName]; !ok {
				return mcp.NewToolResultError(fmt.Sprintf("host %q not found", hostName)), nil
			}

			isLocal := hostName == ac.Cfg.Hostname
			sshTarget := hostName
			out := runcmd(isLocal, sshTarget,
				"systemctl", "list-units", "container@*.service", "--no-pager", "--all")
			return mcp.NewToolResultText(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("host_journal",
			mcp.WithDescription("Read the systemd journal from a host. SSHes automatically if not local."),
			mcp.WithString("host", mcp.Required(), mcp.Description("Host name (e.g. server, desktop, laptop)")),
			mcp.WithString("unit", mcp.Description("Systemd unit to filter by (e.g. sshd.service). Omit for all units.")),
			mcp.WithNumber("lines", mcp.Description("Number of lines to return (default: 100)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			hostName, err := req.RequireString("host")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			entity, err := resolveHost(hostName, ac)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			jArgs := []string{"journalctl", "--no-pager"}
			if args, ok := req.Params.Arguments.(map[string]any); ok {
				if unit, _ := args["unit"].(string); unit != "" {
					jArgs = append(jArgs, "-u", unit)
				}
				lines := 100
				if raw, ok := args["lines"]; ok {
					if n, err := strconv.Atoi(fmt.Sprintf("%v", raw)); err == nil && n > 0 {
						lines = n
					}
				}
				jArgs = append(jArgs, "-n", strconv.Itoa(lines))
			}
			return mcp.NewToolResultText(runcmd(entity.Host.IsLocal, entity.Host.SshTarget, jArgs...)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("host_exec",
			mcp.WithDescription("Run a command on a host via SSH (or locally if current host). Command is passed to sh -c."),
			mcp.WithString("host", mcp.Required(), mcp.Description("Host name (e.g. server, desktop, laptop)")),
			mcp.WithString("command", mcp.Required(), mcp.Description("Shell command to run (e.g. df -h, ip addr)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			hostName, err := req.RequireString("host")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			command, err := req.RequireString("command")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			entity, err := resolveHost(hostName, ac)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(runcmd(entity.Host.IsLocal, entity.Host.SshTarget, "sh", "-c", command)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("host_status",
			mcp.WithDescription("Get the systemd status of a unit on a host. SSHes automatically if not local."),
			mcp.WithString("host", mcp.Required(), mcp.Description("Host name (e.g. server, desktop, laptop)")),
			mcp.WithString("unit", mcp.Required(), mcp.Description("Systemd unit name (e.g. sshd.service, NetworkManager.service)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			hostName, err := req.RequireString("host")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			unit, err := req.RequireString("unit")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			entity, err := resolveHost(hostName, ac)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(runcmd(entity.Host.IsLocal, entity.Host.SshTarget,
				"systemctl", "status", unit, "--no-pager")), nil
		},
	)
}

// resolveHost returns a ConnectedEntity for the named host.
func resolveHost(name string, ac *app.AppContext) (cache.ConnectedEntity, error) {
	hosts, err := ac.HC.Get()
	if err != nil {
		return cache.ConnectedEntity{}, fmt.Errorf("hosts cache: %w", err)
	}
	infra, err := ac.IC.Get()
	if err != nil {
		return cache.ConnectedEntity{}, fmt.Errorf("infra cache: %w", err)
	}
	if _, ok := hosts[name]; !ok {
		return cache.ConnectedEntity{}, fmt.Errorf("host %q not known", name)
	}
	return cache.ResolveFromHost(name, infra, hosts, ac.Cfg.Hostname), nil
}

// resolveContainer finds which host runs the named container and returns its ConnectedEntity.
func resolveContainer(name string, ac *app.AppContext) (cache.ConnectedEntity, error) {
	hosts, err := ac.HC.Get()
	if err != nil {
		return cache.ConnectedEntity{}, fmt.Errorf("hosts cache: %w", err)
	}
	infra, err := ac.IC.Get()
	if err != nil {
		return cache.ConnectedEntity{}, fmt.Errorf("infra cache: %w", err)
	}
	for hostName, eval := range hosts {
		if _, ok := eval.Containers[name]; ok {
			return cache.ResolveFromContainer(name, hostName, infra, hosts, ac.SI, ac.Cfg.Hostname), nil
		}
	}
	return cache.ConnectedEntity{}, fmt.Errorf("container %q not found in any host", name)
}

// runcmd runs args locally or via ssh. Always returns combined output —
// non-zero exit is not treated as an error since systemctl routinely
// exits non-zero for inactive/stopped units.
//
// For SSH: args are shell-quoted and joined into a single string so the
// remote shell sees each token correctly (prevents glob expansion and
// sh -c argument splitting).
//
// It is a variable so tests can substitute a recorder: every diagnostic here
// ends in a real systemctl, journalctl or ssh, and what is worth asserting is
// the command each tool builds, not the output of running it.
var runcmd = func(isLocal bool, sshTarget string, args ...string) string {
	var cmd *exec.Cmd
	if isLocal || sshTarget == "" {
		cmd = exec.Command(args[0], args[1:]...) //nolint:gosec // diagnostics are the tool's purpose
	} else {
		quoted := make([]string, len(args))
		for i, a := range args {
			quoted[i] = shellQuote(a)
		}
		cmd = exec.Command("ssh", sshTarget, strings.Join(quoted, " ")) //nolint:gosec // same
	}
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// shellQuote wraps s in single quotes, escaping any single quotes within.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
