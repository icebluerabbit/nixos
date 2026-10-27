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

func TestLoadUserNestedSnatchSummary(t *testing.T) {
	// Since Sep 2026 MAM nests the breakdown under "snatch_summary" with cache
	// timestamps. The buckets must still reach the top-level fields, or the
	// account snapshot decodes as all-zero and the grab gate holds forever.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"classname":"VIP","ratio":"34.2","uploaded":"2.00 TiB","downloaded":"67.00 GiB","wedges":4,
			"snatch_summary":{"created":1790181000,"created_at":"2026-09-23 16:30:00","unsat":{"count":147,"limit":150},"sSat":{"count":1316},"seedHnr":1,"leeching":0}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", &memSession{id: "x"})
	u, err := c.LoadUser(context.Background(), UserParams{SnatchSummary: true})
	if err != nil {
		t.Fatalf("LoadUser: %v", err)
	}
	if u.Unsat == nil || u.Unsat.Count != 147 || u.Unsat.Limit != 150 {
		t.Errorf("unsat = %+v", u.Unsat)
	}
	if u.SSat == nil || u.SSat.Count != 1316 {
		t.Errorf("sSat = %+v", u.SSat)
	}
	if u.SeedHnr == nil || u.SeedHnr.Count != 1 {
		t.Errorf("seedHnr (bare int, nested) = %+v", u.SeedHnr)
	}
	if u.Wedges != 4 {
		t.Errorf("wedges = %d", u.Wedges)
	}
	if u.Summary == nil || u.Summary.Created != 1790181000 {
		t.Errorf("summary = %+v", u.Summary)
	}
}

func TestLoadUserTopLevelWinsOverNested(t *testing.T) {
	// Both shapes at once: the top-level bucket is the older, authoritative form.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"classname":"User","unsat":{"count":1,"limit":10},
			"snatch_summary":{"unsat":{"count":9,"limit":99},"sSat":{"count":5}}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", &memSession{id: "x"})
	u, err := c.LoadUser(context.Background(), UserParams{SnatchSummary: true})
	if err != nil {
		t.Fatalf("LoadUser: %v", err)
	}
	if u.Unsat == nil || u.Unsat.Count != 1 || u.Unsat.Limit != 10 {
		t.Errorf("unsat = %+v", u.Unsat)
	}
	if u.SSat == nil || u.SSat.Count != 5 {
		t.Errorf("sSat (nested only) = %+v", u.SSat)
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
