package mam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIP(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ip":"10.0.0.1","ASN":1234,"AS":"Some Org"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", &memSession{id: "x"})
	info, err := c.IP(context.Background())
	if err != nil {
		t.Fatalf("IP: %v", err)
	}
	if gotPath != "/json/jsonIp.php" {
		t.Errorf("path = %q", gotPath)
	}
	if info.IP != "10.0.0.1" || info.ASN != 1234 || info.AS != "Some Org" {
		t.Errorf("info = %+v", info)
	}
}
