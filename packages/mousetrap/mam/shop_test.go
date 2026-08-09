package mam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestBonusBuy(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"success_count":1}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", &memSession{id: "x"})
	res, err := c.BonusBuy(context.Background(), BuyParams{SpendType: SpendVIP, Duration: "max"})
	if err != nil {
		t.Fatalf("BonusBuy: %v", err)
	}

	if gotPath != "/json/bonusBuy.php" {
		t.Errorf("path = %q", gotPath)
	}
	if gotQuery.Get("spendtype") != "VIP" || gotQuery.Get("duration") != "max" {
		t.Errorf("query = %v", gotQuery)
	}
	if res.SuccessCount != 1 {
		t.Errorf("success_count = %d", res.SuccessCount)
	}
}
