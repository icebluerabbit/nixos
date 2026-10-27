package qbittorrent

import (
	"context"
	"fmt"
	"net/url"
)

// pathSyncTorrentPeers is the per-torrent peer list.
const pathSyncTorrentPeers = "/api/v2/sync/torrentPeers"

// Peer is one connected peer of one torrent.
//
// The flags string is the point of this endpoint: choke and interest state
// appear nowhere else in the WebAPI, and there is no global peer list, so
// anything that wants to know how many peers want data and are being refused
// has to walk the torrents.
type Peer struct {
	// Flags is libtorrent's peer flag string. Each letter is independent:
	//
	//	U  peer is interested and we have unchoked it (uploading)
	//	u  peer is interested and we have choked it (demand we refuse)
	//	D  we are downloading from the peer
	//	O  optimistic unchoke
	//	S  snubbed
	//	I  incoming — the peer dialled us rather than us dialling it
	Flags string `json:"flags"`
	// UpSpeed is the instantaneous upload rate to this peer, bytes/s.
	UpSpeed int64 `json:"up_speed"`
	// Connection is the transport: "BT" for TCP, "μTP", "Web" for a web
	// seed. Spelling varies by build, so callers match case-insensitively.
	Connection string `json:"connection"`
}

// PeerSync is one /sync/torrentPeers answer. Peers is keyed by "ip:port".
type PeerSync struct {
	Peers map[string]Peer `json:"peers"`
}

// Peers lists the peers currently connected for one torrent.
//
// rid=0 asks for a full snapshot rather than a delta against a previous
// response id, which is what a stateless sampler wants.
func (c *Client) Peers(ctx context.Context, hash string) (PeerSync, error) {
	if hash == "" {
		return PeerSync{}, ErrNoHash
	}
	var out PeerSync
	path := pathSyncTorrentPeers + "?" + url.Values{"hash": {hash}, "rid": {"0"}}.Encode()
	if err := c.http.Do(ctx, "GET", path, nil, &out); err != nil {
		return PeerSync{}, fmt.Errorf("qbittorrent: peers for %s: %w", hash, err)
	}
	return out, nil
}
