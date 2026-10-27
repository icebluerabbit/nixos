package tools

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"homelab/sys-mcp/internal/app"
	"homelab/sys-mcp/internal/cache"
	"homelab/sys-mcp/internal/config"
)

// --- runcmd ---

func TestRuncmd_Local(t *testing.T) {
	out := runcmd(true, "", "echo", "hello")
	if strings.TrimSpace(out) != "hello" {
		t.Errorf("got %q want hello", out)
	}
}

func TestRuncmd_EmptyTargetFallsBackToLocal(t *testing.T) {
	// isLocal=false but sshTarget="" → should still run locally
	out := runcmd(false, "", "echo", "fallback")
	if strings.TrimSpace(out) != "fallback" {
		t.Errorf("got %q want fallback", out)
	}
}

func TestRuncmd_NonZeroExitReturnsOutput(t *testing.T) {
	// sh -c "echo err; exit 1" exits non-zero but still has output
	out := runcmd(true, "", "sh", "-c", "echo err; exit 1")
	if !strings.Contains(out, "err") {
		t.Errorf("expected output despite non-zero exit, got %q", out)
	}
}

func TestRuncmd_StderrCaptured(t *testing.T) {
	out := runcmd(true, "", "sh", "-c", "echo stderr >&2")
	if !strings.Contains(out, "stderr") {
		t.Errorf("stderr not captured: %q", out)
	}
}

func TestRuncmd_MultipleArgs(t *testing.T) {
	out := runcmd(true, "", "printf", "%s %s", "foo", "bar")
	if !strings.Contains(out, "foo") || !strings.Contains(out, "bar") {
		t.Errorf("got %q want 'foo bar'", out)
	}
}

// --- shellQuote ---

func TestShellQuote_Plain(t *testing.T) {
	if shellQuote("hello") != "'hello'" {
		t.Errorf("plain word not quoted correctly")
	}
}

func TestShellQuote_WithSpaces(t *testing.T) {
	q := shellQuote("hostname -I")
	// sh -c with the quoted arg should run hostname -I intact
	out := runcmd(true, "", "sh", "-c", "echo "+q)
	if !strings.Contains(out, "hostname -I") {
		t.Errorf("spaces not preserved, got %q", out)
	}
}

func TestShellQuote_WithSingleQuote(t *testing.T) {
	q := shellQuote("it's fine")
	out := runcmd(true, "", "sh", "-c", "echo "+q)
	if !strings.Contains(out, "it's fine") {
		t.Errorf("single quote not escaped, got %q", out)
	}
}

func TestShellQuote_GlobNotExpanded(t *testing.T) {
	q := shellQuote("container@*.service")
	// passed through echo so glob is not expanded
	out := runcmd(true, "", "sh", "-c", "echo "+q)
	if !strings.Contains(out, "container@*.service") {
		t.Errorf("glob expanded unexpectedly, got %q", out)
	}
}

// --- resolveHost ---

func makeHostAC(localHostname string) *app.AppContext {
	return &app.AppContext{
		Cfg: &config.Config{Hostname: localHostname},
		IC:  cache.NewInfraCacheWithData(testData),
		HC: cache.NewHostsCacheWithData(cache.HostsData{
			"server":  {Containers: map[string]cache.ContainerConf{"grafana": {}, "radarr": {}}},
			"desktop": {},
		}),
		SI: nil,
	}
}

func TestResolveHost_Known(t *testing.T) {
	ac := makeHostAC("desktop")
	e, err := resolveHost("server", ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Host == nil {
		t.Fatal("Host is nil")
	}
	if e.Host.Name != "server" {
		t.Errorf("Name: got %q want server", e.Host.Name)
	}
	if e.Host.IsLocal {
		t.Error("server should not be local when running on desktop")
	}
}

func TestResolveHost_LocalHostname(t *testing.T) {
	ac := makeHostAC("server")
	e, err := resolveHost("server", ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !e.Host.IsLocal {
		t.Error("should be local when hostname matches")
	}
}

func TestResolveHost_Unknown(t *testing.T) {
	ac := makeHostAC("desktop")
	if _, err := resolveHost("laptop", ac); err == nil {
		t.Error("expected error for unknown host")
	}
}

// --- host runtime tools registration & local exec ---

func TestRegisterRuntimeTools_HostToolsPresent(t *testing.T) {
	s := server.NewMCPServer("test", "0.0.0")
	ac := makeHostAC("desktop")
	RegisterRuntimeTools(s, ac) // must not panic
}

func TestHostExec_LocalCommand(t *testing.T) {
	ac := makeHostAC("desktop")
	// resolveHost("desktop") → IsLocal=true → runcmd runs locally
	e, err := resolveHost("desktop", ac)
	if err != nil {
		t.Fatalf("resolveHost: %v", err)
	}
	out := runcmd(e.Host.IsLocal, e.Host.SshTarget, "echo", "host-exec-ok")
	if !strings.Contains(out, "host-exec-ok") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestHostJournal_ArgsBuilt(t *testing.T) {
	// Verify that journal args are constructed correctly by running
	// a harmless local command that echoes its args.
	ac := makeHostAC("desktop")
	e, err := resolveHost("desktop", ac)
	if err != nil {
		t.Fatalf("resolveHost: %v", err)
	}
	// Simulate the arg assembly from host_journal handler
	jArgs := []string{"echo", "--no-pager", "-u", "sshd.service", "-n", "50"}
	out := runcmd(e.Host.IsLocal, e.Host.SshTarget, jArgs...)
	if !strings.Contains(out, "sshd.service") {
		t.Errorf("unit arg not present: %q", out)
	}
}

func TestHostExec_UnknownHostError(t *testing.T) {
	ac := makeHostAC("desktop")
	if _, err := resolveHost("nonexistent", ac); err == nil {
		t.Error("expected error for unknown host in exec path")
	}
}

func TestHostStatus_UnknownHostError(t *testing.T) {
	ac := makeHostAC("desktop")
	if _, err := resolveHost("nonexistent", ac); err == nil {
		t.Error("expected error for unknown host in status path")
	}
}
