package client

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mousetrap/types"
)

func TestAddOk(t *testing.T) {
	var gotCategory, gotAutoTMM, gotPaused string
	var gotFile []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/torrents/add" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		_ = r.ParseMultipartForm(1 << 20)
		gotCategory = r.FormValue("category")
		gotAutoTMM = r.FormValue("autoTMM")
		gotPaused = r.FormValue("paused")
		if f, _, err := r.FormFile("torrents"); err == nil {
			gotFile, _ = io.ReadAll(f)
		}
		_ = params
		_, _ = w.Write([]byte("Ok."))
	}))
	defer srv.Close()

	c := New(types.ClientConfig{URL: srv.URL, Categories: map[string]string{"ebook": "books"}})
	if err := c.Add(context.Background(), []byte("torrentdata"), "My Book", types.KindEbook); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if gotCategory != "books" {
		t.Errorf("category = %q, want books", gotCategory)
	}
	if gotAutoTMM != "true" {
		t.Errorf("autoTMM = %q", gotAutoTMM)
	}
	if gotPaused != "false" {
		t.Errorf("paused = %q", gotPaused)
	}
	if string(gotFile) != "torrentdata" {
		t.Errorf("file = %q", gotFile)
	}
}

func TestAddRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Fails."))
	}))
	defer srv.Close()

	c := New(types.ClientConfig{URL: srv.URL})
	err := c.Add(context.Background(), []byte("x"), "n", types.KindEbook)
	if !errors.Is(err, ErrRejected) {
		t.Errorf("err = %v, want ErrRejected", err)
	}
}

func TestAddHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Forbidden"))
	}))
	defer srv.Close()

	c := New(types.ClientConfig{URL: srv.URL})
	err := c.Add(context.Background(), []byte("x"), "n", types.KindEbook)
	if err == nil || errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v, want 403 transport error", err)
	}
}
