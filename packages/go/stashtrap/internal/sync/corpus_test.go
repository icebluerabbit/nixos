package sync_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"homelab/clients/stashbox"
	"homelab/secret"
	"homelab/stashtrap/internal/store"
	"homelab/stashtrap/internal/sync"
)

// Environment for a real crawl. Absent, the test skips: everything else in this
// package runs against a stub, and a suite that talks to StashDB on every run
// is a suite that fails when StashDB is down.
const (
	envCorpusDB       = "STASHTRAP_CORPUS_DB"
	envCorpusKey      = "STASHTRAP_CORPUS_KEY"
	envCorpusEndpoint = "STASHTRAP_CORPUS_ENDPOINT"
	envCorpusInterval = "STASHTRAP_CORPUS_INTERVAL"
)

// TestCorpusFill runs a real crawl into a copy of a stashtrap database.
//
// It exists to make the parser's corpus report answerable off the server: the
// report is only as good as the catalogue it resolves against, and measuring a
// studio change against eight studios measures nothing.
//
//	STASHTRAP_CORPUS_DB=/tmp/corpus.db STASHTRAP_CORPUS_KEY=… \
//	  go test -run TestCorpusFill -v -timeout 60m ./internal/sync/
func TestCorpusFill(t *testing.T) {
	path := os.Getenv(envCorpusDB)
	key := os.Getenv(envCorpusKey)
	if path == "" || key == "" {
		t.Skipf("set %s and %s to fill a catalogue from a live stash-box", envCorpusDB, envCorpusKey)
	}
	endpoint := os.Getenv(envCorpusEndpoint)
	if endpoint == "" {
		endpoint = "https://stashdb.org/graphql"
	}
	interval := 250 * time.Millisecond
	if raw := os.Getenv(envCorpusInterval); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			t.Fatalf("%s=%q: %v", envCorpusInterval, raw, err)
		}
		interval = d
	}

	client, err := stashbox.New(stashbox.Config{
		Endpoint:    endpoint,
		APIKey:      secret.New(key),
		MinInterval: interval,
	})
	if err != nil {
		t.Fatalf("stashbox.New: %v", err)
	}
	t.Cleanup(client.Close)

	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open(%q): %v", path, err)
	}
	t.Cleanup(func() { _ = st.Close() })

	s, err := sync.New(sync.Deps{
		Client:   client,
		Store:    st,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		Endpoint: endpoint,
	})
	if err != nil {
		t.Fatalf("sync.New: %v", err)
	}

	res, err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v (partial: %+v)", err, res)
	}
	t.Logf("crawled %d performers, %d studios, %d scenes in %s",
		res.Performers, res.Studios, res.Scenes, res.Duration)

	performers, studios, scenes, err := st.CatalogueCounts(context.Background(), endpoint)
	if err != nil {
		t.Fatalf("CatalogueCounts: %v", err)
	}
	t.Logf("catalogue now holds %d performers, %d studios, %d scenes", performers, studios, scenes)
}
