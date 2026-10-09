package tools

import (
	"testing"
)

// --- ipv4Base ---

func TestIPv4Base_Valid(t *testing.T) {
	cases := []struct{ subnet, want string }{
		{"10.10.140.0", "10.10.140."},
		{"10.10.120.0", "10.10.120."},
		{"192.168.1.0", "192.168.1."},
	}
	for _, tc := range cases {
		got, err := ipv4Base(tc.subnet)
		if err != nil {
			t.Errorf("ipv4Base(%q): unexpected error: %v", tc.subnet, err)
		}
		if got != tc.want {
			t.Errorf("ipv4Base(%q) = %q, want %q", tc.subnet, got, tc.want)
		}
	}
}

func TestIPv4Base_Invalid(t *testing.T) {
	cases := []string{"notanip", "10.10.140", "10.10.x.0"}
	for _, s := range cases {
		if _, err := ipv4Base(s); err == nil {
			t.Errorf("ipv4Base(%q): expected error, got nil", s)
		}
	}
}

// --- lastIPOctet ---

func TestLastIPOctet_Valid(t *testing.T) {
	cases := []struct {
		addr string
		want int
	}{
		{"10.10.140.9", 9},
		{"10.10.140.1", 1},
		{"10.10.140.254", 254},
		{"10.10.140.3", 3},
	}
	for _, tc := range cases {
		got, err := lastIPOctet(tc.addr)
		if err != nil {
			t.Errorf("lastIPOctet(%q): unexpected error: %v", tc.addr, err)
		}
		if got != tc.want {
			t.Errorf("lastIPOctet(%q) = %d, want %d", tc.addr, got, tc.want)
		}
	}
}

func TestLastIPOctet_Invalid(t *testing.T) {
	cases := []string{"notanip", "10.10.140", "10.10.140.x"}
	for _, s := range cases {
		if _, err := lastIPOctet(s); err == nil {
			t.Errorf("lastIPOctet(%q): expected error, got nil", s)
		}
	}
}

// --- nextFreeOctet (logic extracted for testing) ---

func TestNextFreeOctet_TakesMaxPlusOne(t *testing.T) {
	// .1=gateway, .2=server, .8=loki, .9=grafana; the gap at .3-.7 is not reused
	used := map[int]string{1: "gateway", 2: "server", 8: "loki", 9: "grafana"}
	got := nextFreeOctet(used, 100)
	if got != 10 {
		t.Errorf("nextFreeOctet = %d, want 10", got)
	}
}

func TestNextFreeOctet_DoesNotReuseReservedGap(t *testing.T) {
	// media keeps .12 deliberately unassigned; allocating must land past .22,
	// never back-fill the reservation.
	used := map[int]string{1: "gateway", 2: "server", 11: "unpackerr", 22: "seasonpackarr"}
	got := nextFreeOctet(used, 100)
	if got != 23 {
		t.Errorf("nextFreeOctet = %d, want 23", got)
	}
}

func TestNextFreeOctet_Exhausted(t *testing.T) {
	// .99 is the last address below a pool starting at .100
	used := map[int]string{99: "last"}
	if got := nextFreeOctet(used, 100); got != -1 {
		t.Errorf("nextFreeOctet = %d, want -1", got)
	}
}

// storage's UNAS sits at .222, above the DHCP pool. It is a reservation, not a
// static allocation, so it must not set the high-water mark — otherwise the
// whole static range beneath it is stranded.
func TestNextFreeOctet_IgnoresAddressesAtOrAbovePool(t *testing.T) {
	used := map[int]string{1: "gateway", 2: "server", 222: "unas-pro"}
	if got := nextFreeOctet(used, 100); got != 3 {
		t.Errorf("nextFreeOctet = %d, want 3", got)
	}
}

func TestNextFreeOctet_IgnoresInPoolReservation(t *testing.T) {
	// a fixed-IP DHCP reservation at .188 does not advance static allocation
	used := map[int]string{1: "gateway", 2: "server", 8: "loki", 188: "google-home"}
	if got := nextFreeOctet(used, 100); got != 9 {
		t.Errorf("nextFreeOctet = %d, want 9", got)
	}
}

func TestNextFreeOctet_DenseAllocation(t *testing.T) {
	used := map[int]string{1: "gw", 2: "srv", 3: "a", 4: "b", 5: "c"}
	got := nextFreeOctet(used, 100)
	if got != 6 {
		t.Errorf("nextFreeOctet = %d, want 6", got)
	}
}

func TestNextFreeOctet_EmptyVlan(t *testing.T) {
	// only .1 and .2 as a convention floor; otherwise empty
	used := map[int]string{1: "gateway"}
	got := nextFreeOctet(used, 100)
	if got != 3 {
		t.Errorf("nextFreeOctet = %d, want 3", got)
	}
}

// --- nextContainerMACIndex (logic extracted for testing) ---

func TestNextContainerMACIndex_TakesMaxPlusOne(t *testing.T) {
	// indices 8 and 9 used; the gap at 1-7 is not reused
	used := map[int]string{8: "container-loki", 9: "container-grafana"}
	got := nextContainerMACIndex(used, 140, nil)
	if got != 10 {
		t.Errorf("nextContainerMACIndex = %d, want 10", got)
	}
}

func TestNextContainerMACIndex_Dense(t *testing.T) {
	used := map[int]string{1: "a", 2: "b", 3: "c"}
	got := nextContainerMACIndex(used, 140, nil)
	if got != 4 {
		t.Errorf("nextContainerMACIndex = %d, want 4", got)
	}
}

func TestNextContainerMACIndex_Empty(t *testing.T) {
	got := nextContainerMACIndex(map[int]string{}, 120, nil)
	if got != 1 {
		t.Errorf("nextContainerMACIndex = %d, want 1", got)
	}
}

func TestNextContainerMACIndex_SkipsIndexZero(t *testing.T) {
	// index 0 = host VLAN client; must not be returned
	used := map[int]string{0: "server-vlan-services"}
	got := nextContainerMACIndex(used, 140, nil)
	if got != 1 {
		t.Errorf("nextContainerMACIndex = %d, want 1 (must skip index 0)", got)
	}
}

func TestNextContainerMACIndex_SkipsCrossVlanCollision(t *testing.T) {
	// media (base 120): index 12 → byte 132 (0x84). vpn-gw on download already
	// owns byte 132. The picker must skip 12 (and 13 → 0x85, also taken) and
	// return 14 → byte 134 (0x86), the first globally-free byte.
	used := map[int]string{}
	for i := 1; i <= 11; i++ { // media indices 1..11 taken on-vlan
		used[i] = "media-container"
	}
	globalBytes := map[int]string{132: "container-vpn-gw", 133: "container-qbittorrent"}
	got := nextContainerMACIndex(used, 120, globalBytes)
	if got != 14 {
		t.Errorf("nextContainerMACIndex = %d, want 14 (skip cross-VLAN bytes 0x84/0x85)", got)
	}
}

// --- containerMACFormula ---

func TestContainerMACFormula(t *testing.T) {
	// vlanId=140, index=9 → byte=149=0x95 → 62:f5:a4:5b:20:95
	got := containerMACFormula(140, 9)
	if got != "62:f5:a4:5b:20:95" {
		t.Errorf("containerMACFormula(140,9) = %q, want 62:f5:a4:5b:20:95", got)
	}
}

func TestContainerMACFormula_FirstContainer(t *testing.T) {
	// vlanId=120, index=1 → byte=121=0x79 → 62:f5:a4:5b:20:79
	got := containerMACFormula(120, 1)
	if got != "62:f5:a4:5b:20:79" {
		t.Errorf("containerMACFormula(120,1) = %q, want 62:f5:a4:5b:20:79", got)
	}
}

func TestBridgeMACFormula(t *testing.T) {
	// vlanId=140 → byte=172=0xac → 82:13:8a:a0:4f:ac
	got := bridgeMACFormula(140)
	if got != "82:13:8a:a0:4f:ac" {
		t.Errorf("bridgeMACFormula(140) = %q, want 82:13:8a:a0:4f:ac", got)
	}
}

func TestBridgeMACFormula_Media(t *testing.T) {
	// vlanId=120 → byte=152=0x98 → 82:13:8a:a0:4f:98
	got := bridgeMACFormula(120)
	if got != "82:13:8a:a0:4f:98" {
		t.Errorf("bridgeMACFormula(120) = %q, want 82:13:8a:a0:4f:98", got)
	}
}
