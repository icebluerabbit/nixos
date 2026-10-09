package mam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestBonusHistory(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		// two rows: a spend (nullable other_*) and a gift (nullable tid); title as int and string.
		_, _ = w.Write([]byte(`[
			{"timestamp":1644804607.09,"amount":-1,"type":"wedgePF","tid":330896,"title":1984,"other_userid":null,"other_name":null},
			{"timestamp":1641914230.10,"amount":-1,"type":"giftWedge","tid":null,"title":null,"other_userid":192566,"other_name":"pezzap7"}
		]`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", &memSession{id: "x"})
	rows, err := c.BonusHistory(context.Background(), BonusParams{Type: []string{"wedgePF", "giftWedge"}})
	if err != nil {
		t.Fatalf("BonusHistory: %v", err)
	}

	if gotPath != "/json/userBonusHistory.php" {
		t.Errorf("path = %q", gotPath)
	}
	if got := gotQuery["type[]"]; len(got) != 2 {
		t.Errorf("type[] = %v", got)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}

	r0 := rows[0]
	if r0.Type != "wedgePF" || r0.Amount != -1 || r0.TID == nil || *r0.TID != 330896 {
		t.Errorf("row0 = %+v", r0)
	}
	if r0.OtherUserID != nil || r0.OtherName != nil {
		t.Errorf("row0 nullables should be nil: %+v", r0)
	}
	if string(r0.Title) != "1984" { // title as int, kept raw
		t.Errorf("row0 title raw = %s", r0.Title)
	}
	r1 := rows[1]
	if r1.TID != nil {
		t.Errorf("row1 tid should be nil")
	}
	if r1.OtherName == nil || *r1.OtherName != "pezzap7" {
		t.Errorf("row1 other_name = %v", r1.OtherName)
	}
}
