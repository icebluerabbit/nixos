package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"homelab/app"
	"homelab/beets-lidarr-import/internal/importer"
)

// The flag must reach its config field. lib/app proves the general mapping
// from a flag name to a config key; what it cannot know is whether this
// tool's flag name matches this tool's json tag. A mismatch is silent — the
// flag parses, the field keeps its zero value, and every rip is refused for
// having no album.
func TestFlagsReachConfig(t *testing.T) {
	t.Parallel()

	cfg := importer.Defaults()
	a := newApp(&cfg)
	a.Args = []string{"--album", "/mnt/media/complete/rips/MEOVV/BITE NOW"}
	a.Stdout = &bytes.Buffer{}
	a.Stderr = &bytes.Buffer{}
	a.Run = func(context.Context, *app.Env) error { return nil }

	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := importer.Config{Album: "/mnt/media/complete/rips/MEOVV/BITE NOW"}
	if diff := cmp.Diff(want, cfg); diff != "" {
		t.Errorf("config (-want +got):\n%s", diff)
	}
}

// beets passes the album path as one argument. A path with spaces in it is
// the common case for an album title, so it must not be split or dropped.
func TestAlbumPathWithSpaces(t *testing.T) {
	t.Parallel()

	cfg := importer.Defaults()
	a := newApp(&cfg)
	a.Args = []string{"--album=/rips/Some Artist/Some Album (Deluxe)"}
	a.Stdout = &bytes.Buffer{}
	a.Stderr = &bytes.Buffer{}
	a.Run = func(context.Context, *app.Env) error { return nil }

	if err := a.Execute(context.Background()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if cfg.Album != "/rips/Some Artist/Some Album (Deluxe)" {
		t.Errorf("Album = %q", cfg.Album)
	}
}
