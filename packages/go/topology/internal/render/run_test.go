package render

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"homelab/app"
)

func testEnv(stdout io.Writer) *app.Env {
	return &app.Env{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Stdout: stdout,
	}
}

// A minimal dump: one gateway device, one VLAN, and the server's bridge onto
// it. Enough for either view to render something recognisable.
const inputJSON = `{
  "infra": {
    "vlans": {"media": {"vlanId": 120, "subnet": "10.0.120.0", "subnetPrefix": 24}},
    "clients": {
      "server-bridge-media": {"ip": "10.0.120.2", "vlan": "media", "kind": "virtual"},
      "unas-pro": {"ip": "10.0.50.10", "vlan": "storage", "kind": "physical",
                   "name": "NAS", "portProfile": "nas"}
    },
    "devices": {
      "dream-machine-pro-max": {"name": "UDM", "ip": "10.0.10.1",
                                "ports": {"28": {"name": "nas", "profile": "nas"}}}
    },
    "ports": {"nas": {"name": "nas", "network": "storage"}}
  },
  "bonds": {"bond0": {"interfaces": ["enp1s0"]}},
  "bridges": {"br-media": {"interfaces": ["vlan-media"]}},
  "containers": {"jellyfin": {"hostBridge": "br-media", "localAddress": "10.0.120.20/24"}}
}`

// With no --into, the rendered view is the product and goes to stdout, banner
// first.
func TestRunPhysicalToStdout(t *testing.T) {
	var out bytes.Buffer

	if err := Run(testEnv(&out), Defaults(), strings.NewReader(inputJSON)); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := out.String()
	if !strings.HasPrefix(got, banner) {
		t.Errorf("missing banner: %q", got[:min(len(got), 60)])
	}
	if !strings.Contains(got, "```mermaid") {
		t.Error("physical view is not a mermaid block")
	}
	if !strings.Contains(got, `dream_machine_pro_max`) {
		t.Error("gateway missing from diagram")
	}
}

// The logical view renders tables for the named host instead.
func TestRunLogicalToStdout(t *testing.T) {
	var out bytes.Buffer

	cfg := Defaults()
	cfg.View = ViewLogical

	if err := Run(testEnv(&out), cfg, strings.NewReader(inputJSON)); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := out.String()
	if strings.Contains(got, "```mermaid") {
		t.Error("logical view emitted a diagram")
	}
	if !strings.Contains(got, "| media | 120 | 10.0.120.0/24 | 10.0.120.2 | 1 | — |") {
		t.Errorf("missing media row:\n%s", got)
	}
	if !strings.Contains(got, "| jellyfin | 10.0.120.20 |") {
		t.Error("missing container row")
	}
}

// --into rewrites the marked region in place and leaves everything around it
// untouched, fenced off from prettier so the two formatters cannot fight.
func TestRunSplicesIntoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	original := "# Homelab\n\nbefore\n\n<!-- BEGIN topology-physical -->\nstale\n<!-- END topology-physical -->\n\nafter\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var out bytes.Buffer
	cfg := Defaults()
	cfg.Into = path
	cfg.Marker = "topology-physical"

	if err := Run(testEnv(&out), cfg, strings.NewReader(inputJSON)); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("stdout written to as well: %q", out.String())
	}

	raw, err := os.ReadFile(path) //nolint:gosec // the path is the test's own
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	got := string(raw)

	for _, want := range []string{
		"# Homelab\n\nbefore\n\n",
		"<!-- BEGIN topology-physical -->\n\n<!-- prettier-ignore-start -->\n\n",
		banner,
		"```mermaid",
		"\n<!-- prettier-ignore-end -->\n\n<!-- END topology-physical -->\n\nafter\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing region:\n  %q\nin:\n%s", want, got)
		}
	}
	if strings.Contains(got, "stale") {
		t.Error("previous content survived the splice")
	}
}

// Splicing twice is a no-op on the second run: `just topology` is expected to
// be idempotent, and CI fails on a stale README by diffing it.
func TestRunSpliceIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	original := "<!-- BEGIN topology -->\n<!-- END topology -->\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := Defaults()
	cfg.Into = path

	var first, second []byte
	for i := range 2 {
		var out bytes.Buffer
		if err := Run(testEnv(&out), cfg, strings.NewReader(inputJSON)); err != nil {
			t.Fatalf("Run() %d error = %v", i, err)
		}
		raw, err := os.ReadFile(path) //nolint:gosec // the path is the test's own
		if err != nil {
			t.Fatalf("read result %d: %v", i, err)
		}
		if i == 0 {
			first = raw
		} else {
			second = raw
		}
	}

	if !bytes.Equal(first, second) {
		t.Errorf("second run changed the file:\n%s\n---\n%s", first, second)
	}
}

// An empty stdin is a forgotten pipe, which is worth naming rather than
// rendering an empty diagram for.
func TestRunRejectsEmptyInput(t *testing.T) {
	err := Run(testEnv(io.Discard), Defaults(), strings.NewReader(""))

	if !errors.Is(err, ErrNoInput) {
		t.Errorf("error = %v, want ErrNoInput", err)
	}
}

func TestRunRejectsBadJSON(t *testing.T) {
	err := Run(testEnv(io.Discard), Defaults(), strings.NewReader("{not json"))

	if err == nil || !strings.Contains(err.Error(), "parse json") {
		t.Errorf("error = %v, want a parse json failure", err)
	}
}

func TestRunRejectsUnknownView(t *testing.T) {
	cfg := Defaults()
	cfg.View = "cabling"

	err := Run(testEnv(io.Discard), cfg, strings.NewReader(inputJSON))

	if !errors.Is(err, ErrUnknownView) {
		t.Errorf("error = %v, want ErrUnknownView", err)
	}
}

// A read failure on stdin is reported rather than treated as no input.
func TestRunReportsStdinFailure(t *testing.T) {
	err := Run(testEnv(io.Discard), Defaults(), errReader{})

	if err == nil || !strings.Contains(err.Error(), "read stdin") {
		t.Errorf("error = %v, want a read stdin failure", err)
	}
}

type errReader struct{}

var errRead = errors.New("broken pipe")

func (errReader) Read([]byte) (int, error) { return 0, errRead }

// Missing markers are refused: appending to the end of someone's README is
// worse than doing nothing.
func TestRunRejectsFileWithoutMarkers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte("# Homelab\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := Defaults()
	cfg.Into = path

	err := Run(testEnv(io.Discard), cfg, strings.NewReader(inputJSON))

	if !errors.Is(err, ErrNoMarkers) {
		t.Errorf("error = %v, want ErrNoMarkers", err)
	}
}

// An END before its BEGIN is not a block, and splicing it would delete the
// text between them.
func TestRunRejectsInvertedMarkers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	body := "<!-- END topology -->\nkeep me\n<!-- BEGIN topology -->\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := Defaults()
	cfg.Into = path

	err := Run(testEnv(io.Discard), cfg, strings.NewReader(inputJSON))

	if !errors.Is(err, ErrNoMarkers) {
		t.Errorf("error = %v, want ErrNoMarkers", err)
	}
	raw, readErr := os.ReadFile(path) //nolint:gosec // the path is the test's own
	if readErr != nil {
		t.Fatalf("read result: %v", readErr)
	}
	if string(raw) != body {
		t.Errorf("file was rewritten:\n%s", raw)
	}
}

// A missing --into file is a caller error, reported with the path so the
// typo is visible.
func TestRunReportsMissingFile(t *testing.T) {
	cfg := Defaults()
	cfg.Into = filepath.Join(t.TempDir(), "nope.md")

	err := Run(testEnv(io.Discard), cfg, strings.NewReader(inputJSON))

	if err == nil || !strings.Contains(err.Error(), "nope.md") {
		t.Errorf("error = %v, want a read failure naming the file", err)
	}
}

// The defaults are what `topology` with no flags renders, and what every unit
// relies on when it passes only the flags it cares about.
func TestDefaults(t *testing.T) {
	want := Config{View: ViewPhysical, Host: "server", Marker: "topology"}

	if got := Defaults(); got != want {
		t.Errorf("Defaults() = %+v, want %+v", got, want)
	}
}
