// Package render turns a derived graph into the markdown that goes in a page.
//
// Mermaid rather than an image because GitHub renders it inline from the
// markdown source: nothing binary is committed, it follows the reader's
// light/dark theme, and a rewiring shows up in review as one changed line.
//
// `flowchart TD` on purpose. A README column is about 900px, so a wide
// left-to-right layout has to scale down until the labels are unreadable.
package render

import (
	"fmt"
	"strings"

	"homelab/topology/internal/graph"
)

// mermaid node ids cannot contain '-' or '.'.
var idReplacer = strings.NewReplacer("-", "_", ".", "_")

// Every label is quoted, which is what lets parentheses through unharmed. HTML
// entities are not an option: GitHub runs mermaid at securityLevel strict,
// which disables HTML labels, so `&#40;` would reach the reader as `&(`. Only
// the quote itself has to go. `<br/>` is the exception strict mode still
// honours.
var labelReplacer = strings.NewReplacer(`"`, "'")

func mermaidID(id string) string { return idReplacer.Replace(id) }

func label(s string) string { return labelReplacer.Replace(s) }

// Shape carries the role as well as colour does, so the diagram survives a
// monochrome print or a colour-blind reader.
var shapes = map[graph.Role][2]string{
	graph.RoleInternet:    {"((", "))"},
	graph.RoleRouter:      {"{{", "}}"},
	graph.RoleSwitch:      {"[", "]"},
	graph.RoleAccessPoint: {"([", "])"},
	graph.RoleHost:        {"[[", "]]"},
	graph.RoleDevice:      {"(", ")"},
}

var classDefs = []struct {
	Role  graph.Role
	Style string
}{
	{graph.RoleInternet, "fill:#495057,stroke:#adb5bd,color:#fff"},
	{graph.RoleRouter, "fill:#4c6ef5,stroke:#748ffc,color:#fff"},
	{graph.RoleSwitch, "fill:#2f9e44,stroke:#69db7c,color:#fff"},
	{graph.RoleAccessPoint, "fill:#f08c00,stroke:#ffd43b,color:#fff"},
	{graph.RoleHost, "fill:#7048e8,stroke:#9775fa,color:#fff"},
	{graph.RoleDevice, "fill:#495057,stroke:#868e96,color:#fff"},
}

func nodeLine(n graph.Node) string {
	text := label(n.Name)
	if n.IP != "" {
		text += "<br/>" + label(n.IP)
	}
	shape := shapes[n.Role]
	// Concatenated rather than formatted: %q would re-escape a label that
	// labelReplacer has already made safe, and the quotes here are mermaid
	// syntax, not Go's.
	return "  " + mermaidID(n.ID) + shape[0] + `"` + text + `"` + shape[1]
}

// vlanSummary is what a cable carries.
//
// An access port lands on exactly one VLAN, and naming it is the single most
// useful fact about the port. A trunk is just called a trunk: spelling out
// four or eleven VLAN names made the widest labels on the page, and the set is
// derived from a denylist that changes whenever a VLAN is added — so the
// diagram would go stale in a way nobody notices.
// `infra.ports.<key>.excludedNetworks` is the answer when you actually need
// it.
func vlanSummary(e graph.Edge) string {
	if e.Trunk {
		return "trunk"
	}
	return e.Network
}

// A label says what the cable carries, nothing else. Port indices and
// interface names are both already in the config and in the controller, and on
// the diagram they were two more strings competing with the VLAN for the
// reader's attention. The endpoints are named by the boxes; the wire only has
// to say what rides it.
//
// A member count survives because it is a property of the link itself, not an
// endpoint: it says this line is more than one cable.
//
// Colour is the fast path — you spot all the trust links at a glance — but it
// is not readable on its own under monochrome printing, colour blindness, or
// in a diff of the source, so the text carries the same fact and there is no
// key to cross-reference.
func edgeLabel(e graph.Edge) string {
	if e.FromPort == "" {
		return e.Label
	}
	out := vlanSummary(e)
	if e.LAG > 0 {
		out = strings.TrimSpace(out + fmt.Sprintf(" x%d", e.LAG))
	}
	return out
}

// VLAN ids are multiples of ten, so id/10 is a dense index that stays put when
// a VLAN is added — unlike a position in a sorted list, which would reshuffle
// every colour on insert. Colours therefore belong to a VLAN, not to whatever
// happens to be drawn today.
var vlanPalette = []string{
	"#868e96", // 0  unused
	"#e8590c", // 10 mgmt
	"#1971c2", // 20 trust
	"#2f9e44", // 30 iot
	"#ae3ec9", // 40 guest
	"#0c8599", // 50 storage
	"#c2255c", // 60
	"#5f3dc4", // 70 work
	"#d6336c", // 80 game
	"#495057", // 90
	"#f76707", // 100 dmz
	"#087f5b", // 110 apps
	"#1864ab", // 120 media
	"#a61e4d", // 130 download
	"#6741d9", // 140 services
}

func vlanColor(vlanID int) string {
	if i := vlanID / 10; i > 0 && i < len(vlanPalette) {
		return vlanPalette[i]
	}
	return vlanPalette[0]
}

// linkStyle is positional: index N styles the Nth link in declaration order.
// So the order edges are written in is load-bearing, not cosmetic.
func linkStyles(g graph.Graph) []string {
	var out []string
	for i, e := range g.Edges {
		switch {
		case e.Trunk:
			// A trunk carries a set, so no single VLAN colour is honest.
			// Weight distinguishes it instead — and it is the line you trace
			// first when something is unreachable.
			out = append(out, fmt.Sprintf("  linkStyle %d stroke:#adb5bd,stroke-width:4px;", i))
		case e.Network != "":
			out = append(out, fmt.Sprintf("  linkStyle %d stroke:%s,stroke-width:2px;",
				i, vlanColor(g.VlanIDs[e.Network])))
		default:
			out = append(out, fmt.Sprintf("  linkStyle %d stroke:%s,stroke-width:2px;",
				i, vlanPalette[0]))
		}
	}
	return out
}

func edgeLine(e graph.Edge) string {
	// A trunk carries many VLANs and is the line you follow when something is
	// unreachable, so it is drawn heavier than an access port.
	arrow := "-->"
	if e.Trunk {
		arrow = "==>"
	}
	from, to := e.From, e.To
	// Edges are undirected but the layout is not. The gateway sits upstream of
	// the switch whose port names the profile, so drawing that one as declared
	// would hang the internet off the bottom of the chart.
	if e.Upstream {
		from, to = e.To, e.From
	}
	return "  " + mermaidID(from) + " " + arrow +
		`|"` + label(edgeLabel(e)) + `"| ` + mermaidID(to)
}

// Mermaid returns a fenced markdown block ready to paste into any page.
func Mermaid(g graph.Graph) string {
	var b strings.Builder

	b.WriteString("```mermaid\nflowchart TD\n")
	for _, n := range g.Nodes {
		b.WriteString(nodeLine(n) + "\n")
	}
	b.WriteString("\n")
	for _, e := range g.Edges {
		b.WriteString(edgeLine(e) + "\n")
	}
	b.WriteString("\n")

	for _, cd := range classDefs {
		fmt.Fprintf(&b, "  classDef %-12s %s;\n", cd.Role, cd.Style)
	}
	for _, cd := range classDefs {
		var ids []string
		for _, n := range g.Nodes {
			if n.Role == cd.Role {
				ids = append(ids, mermaidID(n.ID))
			}
		}
		if len(ids) > 0 {
			fmt.Fprintf(&b, "  class %s %s;\n", strings.Join(ids, ","), cd.Role)
		}
	}

	b.WriteString("\n")
	for _, s := range linkStyles(g) {
		b.WriteString(s + "\n")
	}

	b.WriteString("```\n")

	return b.String()
}
