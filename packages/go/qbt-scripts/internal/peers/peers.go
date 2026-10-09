// Package peers aggregates qBittorrent's per-peer choke and interest state
// into a handful of global numbers.
//
// The WebAPI exposes no global peer list. Peer flags are reachable only
// per-torrent through /sync/torrentPeers, so the aggregate has to be built by
// walking torrents. Cost is bound to the ACTIVE set rather than the library:
// a client holding ten thousand torrents may have thirty transferring, and
// only those have peers worth counting.
//
// Two further bounds keep a bad moment from turning into a stuck scrape — a
// cap on how many torrents are visited, and a wall-clock budget. Both are
// reported in the output, so a sampled scrape is never mistaken for a
// complete one.
package peers

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"time"

	"homelab/clients/qbittorrent"
)

// Bounds on one scrape.
const (
	// DefaultCap is how many active torrents are visited when the caller
	// names no other number.
	DefaultCap = 200
	// Budget is the wall-clock allowance for the per-torrent loop. It sits
	// well inside the 60s scrape interval so a slow pass cannot overlap
	// the next one.
	Budget = 15 * time.Second
	// CallTimeout bounds one WebAPI call. Short: the client is in the same
	// container, and a peer list that has not arrived in two seconds is
	// not worth waiting for when there are hundreds more to fetch.
	CallTimeout = 2 * time.Second
)

// Client is the part of the WebAPI a scrape uses.
type Client interface {
	TorrentsFiltered(ctx context.Context, filter string) ([]qbittorrent.Torrent, error)
	Peers(ctx context.Context, hash string) (qbittorrent.PeerSync, error)
}

// Stats is one scrape, serialised as a single JSON line for Vector's exec
// source to turn into gauges.
//
// The field names are a hard contract: the log_to_metric transform in
// modules/hosts/server/containers/qbittorrent/_exporter.nix names every one
// of them, and a renamed field becomes a gauge that silently stops being
// emitted rather than an error anyone sees.
type Stats struct {
	// ScrapeOK is 0 when the pass failed outright. The counters below are
	// whatever had been gathered when it failed, so a dashboard must gate
	// on this rather than trust a zero.
	ScrapeOK int `json:"scrape_ok"`
	// TorrentsActive is the size of the active set; TorrentsScraped is how
	// many of them were actually visited. Capped is 1 when either bound
	// was hit, which is what makes the gap between the two meaningful.
	TorrentsActive  int `json:"torrents_active"`
	TorrentsScraped int `json:"torrents_scraped"`
	Capped          int `json:"capped"`

	PeersTotal int `json:"peers_total"`
	// Uploading counts peers we have unchoked; InterestedChoked counts
	// peers that want data and are being refused. The second is the
	// interesting one: it is upload demand that exists and is not being
	// served.
	Uploading        int `json:"uploading"`
	InterestedChoked int `json:"interested_choked"`
	Downloading      int `json:"downloading"`
	Optimistic       int `json:"optimistic"`
	Snubbed          int `json:"snubbed"`
	// UploadActive counts peers currently receiving anything at all, which
	// is a smaller number than Uploading: an unchoked peer that is not
	// asking for pieces still counts as unchoked.
	UploadActive int `json:"upload_active"`
	// Incoming counts peers that dialled us. A healthy ratio here is how
	// port forwarding proves itself.
	Incoming int `json:"incoming"`
	Outgoing int `json:"outgoing"`
	// PeerUpMax is the fastest single peer, bytes/s.
	PeerUpMax int64 `json:"peer_up_max"`

	// Connection mix and the upload bytes flowing over each transport.
	ConnBT  int `json:"conn_bt"`
	ConnUTP int `json:"conn_utp"`
	ConnWeb int `json:"conn_web"`

	UpBytesTCP int64 `json:"up_bytes_tcp"`
	UpBytesUTP int64 `json:"up_bytes_utp"`
	UpBytesWeb int64 `json:"up_bytes_web"`
}

// Scrape gathers one sample.
//
// It returns no error. A failure is reported in the sample itself, because
// the caller's only output is one JSON line and an exporter that printed
// nothing would leave Vector with no event at all — which reads as "the
// scrape has not run" rather than "the scrape failed".
func Scrape(ctx context.Context, client Client, capN int, budget time.Duration) Stats {
	var out Stats

	active, err := client.TorrentsFiltered(ctx, qbittorrent.FilterActive)
	if err != nil {
		return out
	}
	out.TorrentsActive = len(active)

	// Rank by upload rate so that a capped pass looks at the torrents
	// carrying the traffic. Stable, so equal-rate torrents keep the order
	// the client listed them in.
	sort.SliceStable(active, func(i, j int) bool {
		return active[i].UpSpeed > active[j].UpSpeed
	})
	if len(active) > capN {
		out.Capped = 1
		active = active[:capN]
	}

	deadline := time.Now().Add(budget)
	for _, torrent := range active {
		if time.Now().After(deadline) {
			out.Capped = 1
			break
		}
		if torrent.Hash == "" {
			continue
		}

		call, cancel := context.WithTimeout(ctx, CallTimeout)
		sync, err := client.Peers(call, torrent.Hash)
		cancel()
		if err != nil {
			// One torrent's peer list is not worth failing the
			// sample for; the scraped count records the loss.
			continue
		}

		out.TorrentsScraped++
		for _, peer := range sync.Peers {
			out.count(peer)
		}
	}

	out.ScrapeOK = 1
	return out
}

// count folds one peer into the totals.
func (s *Stats) count(peer qbittorrent.Peer) {
	s.PeersTotal++

	// Each flag is independent, so these are separate tests rather than a
	// switch: a peer can be unchoked, optimistic and snubbed at once.
	if strings.Contains(peer.Flags, "U") {
		s.Uploading++
	}
	if strings.Contains(peer.Flags, "u") {
		s.InterestedChoked++
	}
	if strings.Contains(peer.Flags, "D") {
		s.Downloading++
	}
	if strings.Contains(peer.Flags, "O") {
		s.Optimistic++
	}
	if strings.Contains(peer.Flags, "S") {
		s.Snubbed++
	}
	if strings.Contains(peer.Flags, "I") {
		s.Incoming++
	} else {
		s.Outgoing++
	}

	if peer.UpSpeed > 0 {
		s.UploadActive++
	}
	if peer.UpSpeed > s.PeerUpMax {
		s.PeerUpMax = peer.UpSpeed
	}

	conn := strings.ToLower(peer.Connection)
	switch {
	case strings.Contains(conn, "web"):
		s.ConnWeb++
		s.UpBytesWeb += peer.UpSpeed
	// Both spellings: the WebUI reports "μTP" on some builds and "uTP" on
	// others, and lowercasing does not unify them.
	case strings.Contains(conn, "utp"), strings.Contains(conn, "μtp"):
		s.ConnUTP++
		s.UpBytesUTP += peer.UpSpeed
	default:
		s.ConnBT++
		s.UpBytesTCP += peer.UpSpeed
	}
}

// Emit writes the sample as one JSON line.
func Emit(w io.Writer, stats Stats) error {
	encoded, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}
