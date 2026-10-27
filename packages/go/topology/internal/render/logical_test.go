package render

import (
	"strings"
	"testing"

	"homelab/topology/internal/graph"
)

func logicalSample() graph.Logical {
	return graph.Logical{
		Host: "server",
		Bond: "bond0",
		NICs: []string{"enp1s0", "enp2s0"},
		Groups: []graph.VlanGroup{
			{
				Name: "media", ID: 120, Subnet: "10.0.120.0", Prefix: 24,
				Bridge: "br-media", Iface: "vlan-media", HostIP: "10.0.120.2",
				Containers: []graph.ContainerNode{
					{Name: "jellyfin", IP: "10.0.120.20"},
					{Name: "navidrome", IP: "10.0.120.21"},
				},
				Clients: []string{"nas", "tv"},
			},
			{
				Name: "guest", ID: 40, Subnet: "10.0.40.0", Prefix: 24,
				Bridge: "br-guest", Iface: "vlan-guest", HostIP: "10.0.40.2",
			},
		},
	}
}

// The summary table has one row per VLAN, in the order the builder put them.
func TestLogicalSummaryTable(t *testing.T) {
	out := Logical(logicalSample())

	for _, want := range []string{
		"| VLAN | id | subnet | host | containers | devices |\n",
		"|:---|---:|:---|:---|---:|---:|\n",
		"| media | 120 | 10.0.120.0/24 | 10.0.120.2 | 2 | 2 |\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing summary row:\n  %s", want)
		}
	}
}

// An empty count is a dash, not a zero: the column is read as "is there
// anything here", and a row of zeroes is noise.
func TestLogicalEmptyCountsAreDashes(t *testing.T) {
	out := Logical(logicalSample())

	want := "| guest | 40 | 10.0.40.0/24 | 10.0.40.2 | — | — |\n"
	if !strings.Contains(out, want) {
		t.Errorf("missing dashed row:\n  %s", want)
	}
}

// Containers are listed per VLAN under an h4, and a VLAN with none gets no
// section at all.
func TestLogicalContainerTable(t *testing.T) {
	out := Logical(logicalSample())

	for _, want := range []string{
		"\n#### media · VLAN 120 · 10.0.120.0/24\n\n",
		"| container | address |\n|:---|:---|\n",
		"| jellyfin | 10.0.120.20 |\n",
		"| navidrome | 10.0.120.21 |\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing container line:\n  %s", want)
		}
	}
	if strings.Contains(out, "#### guest") {
		t.Error("empty VLAN got a container section")
	}
}

// No VLANs at all still emits the header, so the spliced region is a valid
// table rather than nothing.
func TestLogicalWithoutGroups(t *testing.T) {
	out := Logical(graph.Logical{Host: "server"})

	if !strings.HasPrefix(out, "| VLAN | id | subnet | host | containers | devices |\n") {
		t.Errorf("missing header: %q", out)
	}
	if strings.Contains(out, "####") {
		t.Error("container section emitted with no groups")
	}
}
