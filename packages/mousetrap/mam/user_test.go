package mam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestLoadUserRequest(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"uid":"1","username":"me","classname":"User"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", &memSession{id: "x"})
	if _, err := c.LoadUser(context.Background(), UserParams{SnatchSummary: true, Notif: true}); err != nil {
		t.Fatalf("LoadUser: %v", err)
	}
	if gotPath != "/jsonLoad.php" {
		t.Errorf("path = %q", gotPath)
	}
	if _, ok := gotQuery["snatch_summary"]; !ok {
		t.Error("missing snatch_summary")
	}
	if _, ok := gotQuery["notif"]; !ok {
		t.Error("missing notif")
	}
}

func TestLoadUserSnatchForms(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// unsat as object, seedHnr as a bare int — both must parse.
		_, _ = w.Write([]byte(`{"classname":"User","ratio":"5.0","uploaded":"2.00 TiB","downloaded":"1.00 TiB","wedges":3,"unsat":{"count":11,"limit":20},"seedHnr":2}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", &memSession{id: "x"})
	u, err := c.LoadUser(context.Background(), UserParams{SnatchSummary: true})
	if err != nil {
		t.Fatalf("LoadUser: %v", err)
	}
	if u.Unsat == nil || u.Unsat.Count != 11 || u.Unsat.Limit != 20 {
		t.Errorf("unsat = %+v", u.Unsat)
	}
	if u.SeedHnr == nil || u.SeedHnr.Count != 2 {
		t.Errorf("seedHnr (bare int) = %+v", u.SeedHnr)
	}
	if u.Wedges != 3 {
		t.Errorf("wedges = %d", u.Wedges)
	}
}

func TestLoadUserRatioForms(t *testing.T) {
	// MAM sends ratio as a formatted string for some accounts and a bare number for
	// others; both must decode into the same string form.
	for _, tc := range []struct {
		body string
		want string
	}{
		{`{"classname":"User","ratio":"10.00"}`, "10.00"},
		{`{"classname":"User","ratio":4.71}`, "4.71"},
		{`{"classname":"User","ratio":0}`, "0"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(tc.body))
		}))
		c := New(srv.URL, "", &memSession{id: "x"})
		u, err := c.LoadUser(context.Background(), UserParams{})
		srv.Close()
		if err != nil {
			t.Fatalf("LoadUser(%s): %v", tc.body, err)
		}
		if string(u.Ratio) != tc.want {
			t.Errorf("ratio from %s = %q, want %q", tc.body, u.Ratio, tc.want)
		}
	}
}
