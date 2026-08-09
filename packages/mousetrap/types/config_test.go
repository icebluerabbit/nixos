package types

import "testing"

func TestConfigAddr(t *testing.T) {
	c := Config{Listen: "127.0.0.1", Port: "9000"}
	if got := c.Addr(); got != "127.0.0.1:9000" {
		t.Errorf("Addr() = %q, want 127.0.0.1:9000", got)
	}
}
