package mam

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDownload(t *testing.T) {
	torrent := []byte("d8:announce...e") // stand-in .torrent bytes
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = w.Write(torrent)
	}))
	defer srv.Close()

	c := New(srv.URL, "", &memSession{id: "x"})

	// wedge → fl present
	data, err := c.Download(context.Background(), 12345, true)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if !bytes.Equal(data, torrent) {
		t.Errorf("bytes = %q", data)
	}
	if gotPath != "/tor/download.php" {
		t.Errorf("path = %q", gotPath)
	}
	if gotQuery.Get("tid") != "12345" {
		t.Errorf("tid = %q", gotQuery.Get("tid"))
	}
	if _, ok := gotQuery["fl"]; !ok {
		t.Error("wedge grab must include fl")
	}

	// no wedge → no fl
	if _, err := c.Download(context.Background(), 12345, false); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if _, ok := gotQuery["fl"]; ok {
		t.Error("non-wedge grab must not include fl")
	}
}
