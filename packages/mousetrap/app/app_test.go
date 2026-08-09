package app

import (
	"testing"

	"mousetrap/types"
)

func TestNewAndClose(t *testing.T) {
	a, err := New(types.Config{StateDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if a.Store == nil || a.MAM == nil || a.Policy == nil || a.Webhook == nil {
		t.Fatal("New left a dependency nil")
	}
	if err := a.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
