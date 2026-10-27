package peers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"homelab/clients/qbittorrent"
	"homelab/qbt-scripts/internal/peers"
)

// fake is a stand-in client. The peer walk is driven from a map keyed by
// hash, so a test says which torrents exist and what each one is connected to
// without standing up a server.
type fake struct {
	active []qbittorrent.Torrent
	// listErr fails the active listing.
	listErr error
	peers   map[string][]qbittorrent.Peer
	// peerErr fails the peer call for these hashes.
	peerErr map[string]bool
	// visited records the order torrents were walked in, which is what
	// proves the ranking.
	visited []string
	// delay is slept before each peer call, to drive the budget.
	delay time.Duration
}

func (f *fake) TorrentsFiltered(_ context.Context, filter string) ([]qbittorrent.Torrent, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if filter != qbittorrent.FilterActive {
		return nil, errors.New("expected the active filter, got " + filter)
	}
	return f.active, nil
}

func (f *fake) Peers(_ context.Context, hash string) (qbittorrent.PeerSync, error) {
	time.Sleep(f.delay)
	f.visited = append(f.visited, hash)
	if f.peerErr[hash] {
		return qbittorrent.PeerSync{}, errors.New("peer list unavailable")
	}
	rows := make(map[string]qbittorrent.Peer, len(f.peers[hash]))
	for i, p := range f.peers[hash] {
		rows[string(rune('a'+i))] = p
	}
	return qbittorrent.PeerSync{Peers: rows}, nil
}

func torrent(hash string, upSpeed int64) qbittorrent.Torrent {
	return qbittorrent.Torrent{Hash: hash, UpSpeed: upSpeed}
}

// Every flag is independent and a peer can carry several at once, so the
// counters are tested together on one peer set rather than one flag at a
// time.
func TestScrapeCountsFlagsAndTransports(t *testing.T) {
	t.Parallel()

	f := &fake{
		active: []qbittorrent.Torrent{torrent("aaaa", 10)},
		peers: map[string][]qbittorrent.Peer{"aaaa": {
			{Flags: "U I", UpSpeed: 1000, Connection: "BT"},
			{Flags: "u", UpSpeed: 0, Connection: "μTP"},
			{Flags: "D S", UpSpeed: 5, Connection: "uTP"},
			{Flags: "U O I", UpSpeed: 7000, Connection: "Web"},
			{Flags: "", UpSpeed: 0, Connection: ""},
		}},
	}

	got := peers.Scrape(context.Background(), f, peers.DefaultCap, peers.Budget)

	want := peers.Stats{
		ScrapeOK:         1,
		TorrentsActive:   1,
		TorrentsScraped:  1,
		PeersTotal:       5,
		Uploading:        2,
		InterestedChoked: 1,
		Downloading:      1,
		Optimistic:       1,
		Snubbed:          1,
		UploadActive:     3,
		Incoming:         2,
		Outgoing:         3,
		PeerUpMax:        7000,
		ConnBT:           2,
		ConnUTP:          2,
		ConnWeb:          1,
		UpBytesTCP:       1000,
		UpBytesUTP:       5,
		UpBytesWeb:       7000,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Scrape() (-want +got):\n%s", diff)
	}
}

// A capped pass must look at the torrents carrying the traffic, because a
// sample taken from the idle end of the list describes nothing.
func TestScrapeVisitsFastestFirstAndCaps(t *testing.T) {
	t.Parallel()

	f := &fake{active: []qbittorrent.Torrent{
		torrent("slow", 1),
		torrent("fast", 9000),
		torrent("middle", 500),
	}}

	got := peers.Scrape(context.Background(), f, 2, peers.Budget)

	if diff := cmp.Diff([]string{"fast", "middle"}, f.visited); diff != "" {
		t.Errorf("visited (-want +got):\n%s", diff)
	}
	if got.Capped != 1 {
		t.Errorf("Capped = %d, want 1", got.Capped)
	}
	if got.TorrentsActive != 3 {
		t.Errorf("TorrentsActive = %d, want 3 — the full set, not the visited one", got.TorrentsActive)
	}
	if got.TorrentsScraped != 2 {
		t.Errorf("TorrentsScraped = %d, want 2", got.TorrentsScraped)
	}
}

// Equal upload rates must keep the client's own order. Without a stable sort
// the visit order would vary run to run on an idle client, where every active
// torrent reports zero.
func TestScrapeKeepsClientOrderForEqualRates(t *testing.T) {
	t.Parallel()

	f := &fake{active: []qbittorrent.Torrent{
		torrent("first", 0), torrent("second", 0), torrent("third", 0),
	}}

	peers.Scrape(context.Background(), f, peers.DefaultCap, peers.Budget)

	if diff := cmp.Diff([]string{"first", "second", "third"}, f.visited); diff != "" {
		t.Errorf("visited (-want +got):\n%s", diff)
	}
}

func TestScrapeStopsAtTheBudget(t *testing.T) {
	t.Parallel()

	f := &fake{
		active: []qbittorrent.Torrent{torrent("a", 3), torrent("b", 2), torrent("c", 1)},
		delay:  20 * time.Millisecond,
	}

	got := peers.Scrape(context.Background(), f, peers.DefaultCap, 10*time.Millisecond)

	if got.Capped != 1 {
		t.Errorf("Capped = %d, want 1", got.Capped)
	}
	if got.TorrentsScraped != 1 {
		t.Errorf("TorrentsScraped = %d, want 1 — the budget is checked before each call", got.TorrentsScraped)
	}
	if got.ScrapeOK != 1 {
		t.Errorf("ScrapeOK = %d, want 1 — a budgeted pass is a complete pass, just a sampled one", got.ScrapeOK)
	}
}

// One torrent's peer list going missing costs that torrent, not the sample.
func TestScrapeSkipsUnreadablePeerLists(t *testing.T) {
	t.Parallel()

	f := &fake{
		active:  []qbittorrent.Torrent{torrent("good", 2), torrent("bad", 1)},
		peers:   map[string][]qbittorrent.Peer{"good": {{Flags: "U"}}},
		peerErr: map[string]bool{"bad": true},
	}

	got := peers.Scrape(context.Background(), f, peers.DefaultCap, peers.Budget)

	if got.TorrentsScraped != 1 {
		t.Errorf("TorrentsScraped = %d, want 1", got.TorrentsScraped)
	}
	if got.ScrapeOK != 1 {
		t.Errorf("ScrapeOK = %d, want 1", got.ScrapeOK)
	}
	if got.PeersTotal != 1 {
		t.Errorf("PeersTotal = %d, want 1", got.PeersTotal)
	}
}

func TestScrapeSkipsTorrentsWithoutAHash(t *testing.T) {
	t.Parallel()

	f := &fake{active: []qbittorrent.Torrent{torrent("", 5), torrent("aaaa", 1)}}

	got := peers.Scrape(context.Background(), f, peers.DefaultCap, peers.Budget)

	if diff := cmp.Diff([]string{"aaaa"}, f.visited); diff != "" {
		t.Errorf("visited (-want +got):\n%s", diff)
	}
	if got.TorrentsScraped != 1 {
		t.Errorf("TorrentsScraped = %d, want 1", got.TorrentsScraped)
	}
}

// A failed listing is reported in the sample, not as an error: the caller's
// only output is one JSON line, and printing nothing would read as "the
// scrape has not run".
func TestScrapeReportsFailureInTheSample(t *testing.T) {
	t.Parallel()

	f := &fake{listErr: errors.New("connection refused")}

	got := peers.Scrape(context.Background(), f, peers.DefaultCap, peers.Budget)

	if diff := cmp.Diff(peers.Stats{}, got); diff != "" {
		t.Errorf("Scrape() (-want +got):\n%s", diff)
	}
}

// The field names are the contract with Vector's log_to_metric transform in
// modules/hosts/server/containers/qbittorrent/_exporter.nix. A renamed field
// does not fail anywhere: the gauge simply stops being emitted.
func TestEmitKeys(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := peers.Emit(&buf, peers.Stats{}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	line := buf.String()
	if !strings.HasSuffix(line, "\n") {
		t.Error("Emit() wrote no trailing newline; the exec source reads lines")
	}
	if strings.Count(line, "\n") != 1 {
		t.Errorf("Emit() wrote %d lines, want exactly 1", strings.Count(line, "\n"))
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(line), &decoded); err != nil {
		t.Fatalf("the emitted line is not JSON: %v", err)
	}

	want := []string{
		"capped", "conn_bt", "conn_utp", "conn_web", "downloading",
		"incoming", "interested_choked", "optimistic", "outgoing",
		"peer_up_max", "peers_total", "scrape_ok", "snubbed",
		"torrents_active", "torrents_scraped", "up_bytes_tcp",
		"up_bytes_utp", "up_bytes_web", "upload_active", "uploading",
	}
	got := make([]string, 0, len(decoded))
	for key := range decoded {
		got = append(got, key)
	}
	sort.Strings(got)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("emitted keys (-want +got):\n%s", diff)
	}
}

func TestEmitReportsWriteFailure(t *testing.T) {
	t.Parallel()

	if err := peers.Emit(failWriter{}, peers.Stats{}); err == nil {
		t.Fatal("Emit() error = nil, want a failure")
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }
