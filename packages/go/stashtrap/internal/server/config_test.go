package server_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"homelab/stashtrap/internal/server"
)

func TestDefaults(t *testing.T) {
	t.Parallel()

	cfg := server.Defaults()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("defaults do not validate: %v", err)
	}
	if cfg.Addr == "" || cfg.DBPath == "" || cfg.ShutdownTimeout == 0 {
		t.Fatalf("incomplete defaults: %+v", cfg)
	}
	// An endpoint without a key is inert, so shipping StashDB's is safe and
	// saves every deployment from repeating it.
	if cfg.StashboxEndpoint == "" {
		t.Fatal("no default stash-box endpoint")
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	cases := map[string]server.Config{
		"no addr": {DBPath: "/tmp/x.db"},
		"no db":   {Addr: ":8080"},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate accepted an unusable configuration")
			}
		})
	}
}

func TestStashbox(t *testing.T) {
	t.Parallel()

	t.Run("no key means no catalogue", func(t *testing.T) {
		t.Parallel()
		// A deployment without a key has to run: the announce path does not
		// need one, and refusing to start would take autobrr's gate down
		// over a credential it never used.
		_, ok, err := server.Config{StashboxEndpoint: "https://stashdb.org/graphql"}.Stashbox()
		if err != nil || ok {
			t.Fatalf("Stashbox() = %v, %v; want false, nil", ok, err)
		}
	})

	t.Run("no endpoint means no catalogue", func(t *testing.T) {
		t.Parallel()
		_, ok, err := server.Config{StashboxKey: "k"}.Stashbox()
		if err != nil || ok {
			t.Fatalf("Stashbox() = %v, %v; want false, nil", ok, err)
		}
	})

	t.Run("literal key", func(t *testing.T) {
		t.Parallel()
		cfg, ok, err := server.Config{
			StashboxEndpoint:    "https://stashdb.org/graphql",
			StashboxKey:         "literal",
			StashboxMinInterval: 2 * time.Second,
		}.Stashbox()
		if err != nil || !ok {
			t.Fatalf("Stashbox() = %v, %v", ok, err)
		}
		if cfg.APIKey.Reveal() != "literal" || cfg.MinInterval != 2*time.Second {
			t.Fatalf("config = %+v", cfg)
		}
	})

	t.Run("key from file", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "key")
		if err := os.WriteFile(path, []byte("from-file\n"), 0o600); err != nil {
			t.Fatalf("write key: %v", err)
		}
		cfg, ok, err := server.Config{
			StashboxEndpoint: "https://stashdb.org/graphql",
			StashboxKeyFile:  path,
		}.Stashbox()
		if err != nil || !ok {
			t.Fatalf("Stashbox() = %v, %v", ok, err)
		}
		// Trailing newline is the normal shape of a sops secret file.
		if cfg.APIKey.Reveal() != "from-file" {
			t.Fatalf("key = %q", cfg.APIKey.Reveal())
		}
	})

	t.Run("missing key file", func(t *testing.T) {
		t.Parallel()
		_, _, err := server.Config{
			StashboxEndpoint: "https://stashdb.org/graphql",
			StashboxKeyFile:  filepath.Join(t.TempDir(), "absent"),
		}.Stashbox()
		if err == nil {
			t.Fatal("Stashbox() accepted a missing key file")
		}
	})
}
