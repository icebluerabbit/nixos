package qbittorrent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestPeersDecodesFlags(t *testing.T) {
	var gotQuery string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathSyncTorrentPeers {
			t.Errorf("path = %s, want %s", r.URL.Path, pathSyncTorrentPeers)
		}
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"peers":{"10.0.0.1:51413":{
		  "flags": "U I P",
		  "up_speed": 94000,
		  "connection": "μTP"
		}}}`)
	}))

	got, err := c.Peers(context.Background(), "aaaa")
	if err != nil {
		t.Fatalf("Peers() error = %v", err)
	}

	// rid=0 asks for a full snapshot rather than a delta against a
	// previous response id, which is what a stateless sampler needs.
	if gotQuery != "hash=aaaa&rid=0" {
		t.Errorf("query = %q, want hash=aaaa&rid=0", gotQuery)
	}

	peer, ok := got.Peers["10.0.0.1:51413"]
	if !ok {
		t.Fatalf("peers = %v, want the one key", got.Peers)
	}
	if peer.Flags != "U I P" {
		t.Errorf("Flags = %q", peer.Flags)
	}
	if peer.UpSpeed != 94000 {
		t.Errorf("UpSpeed = %d, want 94000", peer.UpSpeed)
	}
	if peer.Connection != "μTP" {
		t.Errorf("Connection = %q, want μTP", peer.Connection)
	}
}

func TestPeersRequiresAHash(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the WebUI should not have been called")
	}))

	if _, err := c.Peers(context.Background(), ""); !errors.Is(err, ErrNoHash) {
		t.Fatalf("Peers() error = %v, want ErrNoHash", err)
	}
}

func TestPeersReportsServerFailure(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	if _, err := c.Peers(context.Background(), "aaaa"); err == nil {
		t.Fatal("Peers() error = nil, want a failure")
	}
}
