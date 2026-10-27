package render

import (
	"strings"
	"testing"

	"homelab/topology/internal/graph"
)

// sample is a cut-down physical graph: a gateway reached from the internet, a
// switch trunked to a host, and one access-port device.
func sample() graph.Graph {
	return graph.Graph{
		Nodes: []graph.Node{
			{ID: "internet", Name: "Internet", Role: graph.RoleInternet},
			{ID: "dream-machine-pro-max", Name: "UDM Pro Max (r1)", Role: graph.RoleRouter, IP: "10.0.10.1"},
			{ID: "usw-pro-hd-24-poe", Name: "Pro HD 24", Role: graph.RoleSwitch, IP: "10.0.10.2"},
			{ID: "server", Name: "server", Role: graph.RoleHost},
			{ID: "unas-pro", Name: `NAS "main"`, Role: graph.RoleDevice, IP: "10.0.50.10"},
		},
		Edges: []graph.Edge{
			{From: "internet", To: "dream-machine-pro-max", ToPort: "wan", Label: "WAN"},
			{
				From: "usw-pro-hd-24-poe", FromPort: "27", To: "dream-machine-pro-max",
				ToPort: "uplink", Trunk: true, Upstream: true, Label: "uplink",
			},
			{
				From: "usw-pro-hd-24-poe", FromPort: "25", To: "server", ToPort: "bond0",
				Trunk: true, LAG: 2, Label: "server trunk",
			},
			{
				From: "usw-pro-hd-24-poe", FromPort: "28", To: "unas-pro", ToPort: "eth0",
				Network: "storage", Label: "nas",
			},
		},
		VlanIDs: map[string]int{"storage": 50},
	}
}

// Node ids reach mermaid with the characters it rejects replaced, and a name
// carrying a double quote does not break out of its label.
func TestMermaidNodeLines(t *testing.T) {
	out := Mermaid(sample())

	for _, want := range []string{
		`  internet(("Internet"))`,
		`  dream_machine_pro_max{{"UDM Pro Max (r1)<br/>10.0.10.1"}}`,
		`  usw_pro_hd_24_poe["Pro HD 24<br/>10.0.10.2"]`,
		`  server[["server"]]`,
		`  unas_pro("NAS 'main'<br/>10.0.50.10")`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing node line:\n  %s", want)
		}
	}
}

// A trunk is drawn heavier and says only "trunk"; a LAG adds its member
// count; an access port names its VLAN; and the WAN edge keeps its label.
func TestMermaidEdgeLines(t *testing.T) {
	out := Mermaid(sample())

	for _, want := range []string{
		`  internet -->|"WAN"| dream_machine_pro_max`,
		`  usw_pro_hd_24_poe ==>|"trunk x2"| server`,
		`  usw_pro_hd_24_poe -->|"storage"| unas_pro`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing edge line:\n  %s", want)
		}
	}
}

// The gateway sits above the switch whose port names the profile, so that one
// edge is drawn the other way round — otherwise the internet hangs off the
// bottom of the chart.
func TestMermaidUpstreamEdgeIsFlipped(t *testing.T) {
	out := Mermaid(sample())

	want := `  dream_machine_pro_max ==>|"trunk"| usw_pro_hd_24_poe`
	if !strings.Contains(out, want) {
		t.Errorf("missing flipped edge:\n  %s", want)
	}
	if strings.Contains(out, `usw_pro_hd_24_poe ==>|"trunk"| dream_machine_pro_max`) {
		t.Error("upstream edge drawn as declared")
	}
}

// linkStyle is positional, so every edge needs one and they must be in
// declaration order: a trunk heavy and grey, an access port in its VLAN's
// colour, anything else in the fallback.
func TestMermaidLinkStyles(t *testing.T) {
	out := Mermaid(sample())

	for _, want := range []string{
		"  linkStyle 0 stroke:#868e96,stroke-width:2px;",
		"  linkStyle 1 stroke:#adb5bd,stroke-width:4px;",
		"  linkStyle 2 stroke:#adb5bd,stroke-width:4px;",
		"  linkStyle 3 stroke:#0c8599,stroke-width:2px;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing link style:\n  %s", want)
		}
	}
}

// Every role gets a classDef, and only the roles actually drawn get a class
// assignment.
func TestMermaidClasses(t *testing.T) {
	out := Mermaid(sample())

	if !strings.Contains(out, "  classDef router       fill:#4c6ef5,stroke:#748ffc,color:#fff;") {
		t.Error("missing router classDef")
	}
	if !strings.Contains(out, "  class dream_machine_pro_max router;") {
		t.Error("missing router class assignment")
	}
	if strings.Contains(out, "  class  accessPoint;") {
		t.Error("empty class assignment emitted for an undrawn role")
	}
}

// The block has to be fenced as mermaid or GitHub renders it as text.
func TestMermaidFence(t *testing.T) {
	out := Mermaid(sample())

	if !strings.HasPrefix(out, "```mermaid\nflowchart TD\n") {
		t.Errorf("bad opening fence: %q", out[:min(len(out), 40)])
	}
	if !strings.HasSuffix(out, "```\n") {
		t.Error("missing closing fence")
	}
}

// A VLAN id outside the palette falls back to the unused colour rather than
// panicking on the index.
func TestVlanColorOutOfRange(t *testing.T) {
	if got := vlanColor(9000); got != vlanPalette[0] {
		t.Errorf("out of range: got %q want %q", got, vlanPalette[0])
	}
	if got := vlanColor(0); got != vlanPalette[0] {
		t.Errorf("zero: got %q want %q", got, vlanPalette[0])
	}
	if got := vlanColor(120); got != "#1864ab" {
		t.Errorf("media: got %q want #1864ab", got)
	}
}
