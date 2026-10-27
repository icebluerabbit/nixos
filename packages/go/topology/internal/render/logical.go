package render

import (
	"fmt"
	"strings"

	"homelab/topology/internal/graph"
)

// Logical writes the host's internal network as tables.
//
// No diagram. Every shape it went through — a tree, a grid, a fan — ended up
// restating what these tables say. Fifty containers is an inventory, and an
// inventory sorts, greps and diffs; a picture of it does none of those.
func Logical(l graph.Logical) string {
	var b strings.Builder

	// Bridge and uplink names are mechanical — `br-<vlan>` on `vlan-<vlan>` on
	// the one bond, every time. They are still derived and carried on
	// VlanGroup for anything that needs to name an interface; they just do not
	// earn a column that repeats the VLAN name twice per row.
	b.WriteString("| VLAN | id | subnet | host | containers | devices |\n")
	b.WriteString("|:---|---:|:---|:---|---:|---:|\n")
	for _, g := range l.Groups {
		fmt.Fprintf(&b, "| %s | %d | %s/%d | %s | %s | %s |\n",
			g.Name, g.ID, g.Subnet, g.Prefix, g.HostIP,
			count(len(g.Containers)), count(len(g.Clients)))
	}

	b.WriteString(containerTable(l))
	return b.String()
}

func count(n int) string {
	if n == 0 {
		return "—"
	}
	return fmt.Sprintf("%d", n)
}

// Grouped by VLAN so it reads alongside the summary above.
func containerTable(l graph.Logical) string {
	var b strings.Builder
	for _, g := range l.Groups {
		if len(g.Containers) == 0 {
			continue
		}
		// h4: these sit under whatever heading the host document gives the
		// section, which is h3 in the README.
		fmt.Fprintf(&b, "\n#### %s · VLAN %d · %s/%d\n\n", g.Name, g.ID, g.Subnet, g.Prefix)
		b.WriteString("| container | address |\n|:---|:---|\n")
		for _, c := range g.Containers {
			fmt.Fprintf(&b, "| %s | %s |\n", c.Name, c.IP)
		}
	}
	return b.String()
}
