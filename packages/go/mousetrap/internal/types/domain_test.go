package types

import "testing"

func TestAccountStateVIP(t *testing.T) {
	cases := map[string]bool{
		"VIP":          true,
		"vip":          true,
		"Elite VIP":    true,
		"SomeVIPClass": true,
		"User":         false,
		"Power User":   false,
		"":             false,
	}
	for class, want := range cases {
		if got := (AccountState{ClassName: class}).VIP(); got != want {
			t.Errorf("VIP(%q) = %v, want %v", class, got, want)
		}
	}
}
