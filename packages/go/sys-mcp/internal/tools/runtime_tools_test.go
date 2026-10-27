package tools

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
	"homelab/sys-mcp/internal/config"
)

// recordedCmd is one invocation the diagnostics would have made.
type recordedCmd struct {
	IsLocal bool
	SSH     string
	Args    []string
}

// recordCmds swaps the exec seam for a recorder, so a test asserts the
// command a tool builds and where it would have run it. Running a diagnostic
// against the wrong host returns plausible-looking data with no error, so the
// target is as much a part of the contract as the command line.
func recordCmds(t *testing.T, output string) *[]recordedCmd {
	t.Helper()

	var calls []recordedCmd
	original := runcmd
	runcmd = func(isLocal bool, sshTarget string, args ...string) string {
		calls = append(calls, recordedCmd{IsLocal: isLocal, SSH: sshTarget, Args: args})
		return output
	}
	t.Cleanup(func() { runcmd = original })
	return &calls
}

// runtimeServer registers the diagnostics with the local host set to
// localHostname, which decides local execution versus ssh.
func runtimeServer(t *testing.T, localHostname string) *server.MCPServer {
	t.Helper()
	s := server.NewMCPServer("test", "0.0.0")
	RegisterRuntimeTools(s, makeHostAC(localHostname))
	return s
}

func TestRegisterRuntimeToolsRegistersCatalog(t *testing.T) {
	names := toolNames(t, runtimeServer(t, "desktop"))

	for _, want := range []string{
		"container_status", "container_journal", "container_exec",
		"container_list", "host_journal", "host_exec", "host_status",
	} {
		if !has(names, want) {
			t.Errorf("tool %q is not registered; got %v", want, names)
		}
	}
}

// grafana runs on server; from desktop that is an ssh, and the unit is the
// host-level container@ unit rather than anything inside the guest.
func TestContainerStatusToolTargetsTheOwningHost(t *testing.T) {
	calls := recordCmds(t, "active")
	s := runtimeServer(t, "desktop")

	out, isErr := callTool(t, s, "container_status", map[string]any{"container": "grafana"})
	if isErr {
		t.Fatalf("container_status reported an error: %s", out)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d commands, want 1: %+v", len(*calls), *calls)
	}
	got := (*calls)[0]
	if got.IsLocal {
		t.Error("grafana runs on server, so from desktop this must not be local")
	}
	if !strings.Contains(strings.Join(got.Args, " "), "container@grafana.service") {
		t.Errorf("wrong unit: %v", got.Args)
	}
}

// The same call from the host that runs the container executes locally.
func TestContainerStatusToolRunsLocallyOnTheOwningHost(t *testing.T) {
	calls := recordCmds(t, "active")
	s := runtimeServer(t, "server")

	if out, isErr := callTool(t, s, "container_status", map[string]any{"container": "grafana"}); isErr {
		t.Fatalf("container_status reported an error: %s", out)
	}
	if !(*calls)[0].IsLocal {
		t.Error("on server, a server container must run locally")
	}
}

func TestContainerStatusToolRejectsUnknownContainer(t *testing.T) {
	recordCmds(t, "")
	s := runtimeServer(t, "desktop")

	if _, isErr := callTool(t, s, "container_status", map[string]any{"container": "nope"}); !isErr {
		t.Error("unknown container should be an error result")
	}
}

// The journal is read from inside the guest, and the line count and unit
// filter reach the command line.
func TestContainerJournalToolPassesFilters(t *testing.T) {
	calls := recordCmds(t, "log line")
	s := runtimeServer(t, "desktop")

	out, isErr := callTool(t, s, "container_journal", map[string]any{
		"container": "grafana",
		"unit":      "grafana.service",
		"lines":     25,
	})
	if isErr {
		t.Fatalf("container_journal reported an error: %s", out)
	}

	line := strings.Join((*calls)[0].Args, " ")
	for _, want := range []string{"grafana", "grafana.service", "25"} {
		if !strings.Contains(line, want) {
			t.Errorf("%q missing from command: %s", want, line)
		}
	}
}

func TestContainerExecTool(t *testing.T) {
	calls := recordCmds(t, "ok")
	s := runtimeServer(t, "desktop")

	out, isErr := callTool(t, s, "container_exec", map[string]any{
		"container": "grafana",
		"command":   "systemctl is-active grafana.service",
	})
	if isErr {
		t.Fatalf("container_exec reported an error: %s", out)
	}

	line := strings.Join((*calls)[0].Args, " ")
	if !strings.Contains(line, "nixos-container run grafana") {
		t.Errorf("wrong entry command: %s", line)
	}
	if !strings.Contains(line, "systemctl is-active grafana.service") {
		t.Errorf("command not passed through: %s", line)
	}
}

func TestContainerExecToolRequiresCommand(t *testing.T) {
	recordCmds(t, "")
	s := runtimeServer(t, "desktop")

	if _, isErr := callTool(t, s, "container_exec", map[string]any{"container": "grafana"}); !isErr {
		t.Error("missing command should be an error result")
	}
}

func TestContainerListTool(t *testing.T) {
	recordCmds(t, "container@grafana.service loaded active running")
	s := runtimeServer(t, "desktop")

	out, isErr := callTool(t, s, "container_list", map[string]any{"host": "server"})
	if isErr {
		t.Fatalf("container_list reported an error: %s", out)
	}
	if out == "" {
		t.Error("container_list returned nothing")
	}
}

func TestHostJournalTool(t *testing.T) {
	calls := recordCmds(t, "log line")
	s := runtimeServer(t, "desktop")

	out, isErr := callTool(t, s, "host_journal", map[string]any{
		"host":  "server",
		"unit":  "sshd.service",
		"lines": 5,
	})
	if isErr {
		t.Fatalf("host_journal reported an error: %s", out)
	}

	line := strings.Join((*calls)[0].Args, " ")
	for _, want := range []string{"journalctl", "sshd.service", "-n 5"} {
		if !strings.Contains(line, want) {
			t.Errorf("%q missing from command: %s", want, line)
		}
	}
}

// Omitting the line count keeps the documented default rather than asking
// for the whole journal.
func TestHostJournalToolDefaultsLineCount(t *testing.T) {
	calls := recordCmds(t, "log line")
	s := runtimeServer(t, "desktop")

	if out, isErr := callTool(t, s, "host_journal", map[string]any{"host": "server"}); isErr {
		t.Fatalf("host_journal reported an error: %s", out)
	}
	if line := strings.Join((*calls)[0].Args, " "); !strings.Contains(line, "-n 100") {
		t.Errorf("default of 100 lines missing: %s", line)
	}
}

func TestHostExecTool(t *testing.T) {
	calls := recordCmds(t, "Filesystem")
	s := runtimeServer(t, "desktop")

	out, isErr := callTool(t, s, "host_exec", map[string]any{
		"host":    "server",
		"command": "df -h",
	})
	if isErr {
		t.Fatalf("host_exec reported an error: %s", out)
	}
	if line := strings.Join((*calls)[0].Args, " "); !strings.Contains(line, "sh -c df -h") {
		t.Errorf("command not passed to a shell: %s", line)
	}
}

func TestHostToolsRejectUnknownHost(t *testing.T) {
	recordCmds(t, "")
	s := runtimeServer(t, "desktop")

	for _, tool := range []string{"host_journal", "host_exec", "host_status"} {
		args := map[string]any{"host": "nosuchhost", "command": "true", "unit": "sshd.service"}
		if _, isErr := callTool(t, s, tool, args); !isErr {
			t.Errorf("%s: unknown host should be an error result", tool)
		}
	}
}

func TestHostStatusTool(t *testing.T) {
	calls := recordCmds(t, "active")
	s := runtimeServer(t, "desktop")

	out, isErr := callTool(t, s, "host_status", map[string]any{
		"host": "server",
		"unit": "sshd.service",
	})
	if isErr {
		t.Fatalf("host_status reported an error: %s", out)
	}
	if line := strings.Join((*calls)[0].Args, " "); !strings.Contains(line, "systemctl status sshd.service") {
		t.Errorf("wrong command: %s", line)
	}
}

// resolveContainer is what makes a diagnostic land on the right machine.
func TestResolveContainer(t *testing.T) {
	ac := makeHostAC("desktop")

	entity, err := resolveContainer("grafana", ac)
	if err != nil {
		t.Fatalf("resolveContainer: %v", err)
	}
	if entity.Host == nil || entity.Host.Name != "server" {
		t.Errorf("grafana should resolve to server, got %+v", entity.Host)
	}

	if _, err := resolveContainer("nope", ac); err == nil {
		t.Error("an unknown container should not resolve")
	}
}

// A container name that exists on no host must not silently fall back to the
// local machine.
func TestResolveContainerDoesNotFallBackLocally(t *testing.T) {
	ac := &app.AppContext{
		Cfg: &config.Config{Hostname: "server"},
		IC:  cache.NewInfraCacheWithData(testData),
		HC:  cache.NewHostsCacheWithData(cache.HostsData{"server": {}}),
	}

	if _, err := resolveContainer("grafana", ac); err == nil {
		t.Error("a container declared by no host must not resolve")
	}
}
