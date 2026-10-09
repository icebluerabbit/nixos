package mam

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type memSession struct{ id string }

func (m *memSession) Load() (string, error) { return m.id, nil }
func (m *memSession) Save(id string) error  { m.id = id; return nil }

func TestSeedAndRotate(t *testing.T) {
	var sent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ck, err := r.Cookie("mam_id"); err == nil {
			sent = ck.Value
		}
		http.SetCookie(w, &http.Cookie{Name: "mam_id", Value: "rotated"})
		_, _ = w.Write([]byte(`{"ip":"1.2.3.4","ASN":42,"AS":"Org"}`))
	}))
	defer srv.Close()

	seed := filepath.Join(t.TempDir(), "seed")
	if err := os.WriteFile(seed, []byte("mam_id=initial;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sess := &memSession{}
	c := New(srv.URL, seed, sess)

	info, err := c.IP(context.Background())
	if err != nil {
		t.Fatalf("IP: %v", err)
	}
	if info.ASN != 42 {
		t.Errorf("ASN = %d", info.ASN)
	}
	if sent != "initial" {
		t.Errorf("sent cookie = %q, want seeded 'initial'", sent)
	}
	if sess.id != "rotated" {
		t.Errorf("session = %q, want rotated", sess.id)
	}
}

func TestHTTPErrorCarriesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotAcceptable)
		_, _ = w.Write([]byte("Not Acceptable (Account issue)"))
	}))
	defer srv.Close()

	c := New(srv.URL, "", &memSession{id: "x"})
	_, err := c.Download(context.Background(), 5, false)
	var he *HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("want *HTTPError, got %v", err)
	}
	if he.Code != http.StatusNotAcceptable || !strings.Contains(he.Body, "Account issue") {
		t.Errorf("he = %+v", he)
	}
}

func TestNoSessionNoSeed(t *testing.T) {
	c := New("http://example.invalid", "", &memSession{})
	if _, err := c.IP(context.Background()); err == nil {
		t.Error("expected error with no session and no seed path")
	}
}
