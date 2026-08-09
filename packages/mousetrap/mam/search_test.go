package mam

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchRequestAndParse(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"total":1,"total_found":42,"data":[{"id":99,"catname":"Ebooks - Fantasy","size":"6.0 MiB","seeders":12,"my_snatched":0}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "", &memSession{id: "x"})
	res, err := c.Search(context.Background(), SearchParams{
		Text:       "dune",
		SearchType: "all",
		MainCat:    []int{14},
		PerPage:    50,
		MySnatched: true,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if gotPath != "/tor/js/loadSearchJSONbasic.php" {
		t.Errorf("path = %q", gotPath)
	}
	tor, _ := body["tor"].(map[string]any)
	if tor["searchType"] != "all" || tor["text"] != "dune" {
		t.Errorf("tor = %v", tor)
	}
	if body["perpage"] != float64(50) {
		t.Errorf("perpage = %v", body["perpage"])
	}
	if body["my_snatched"] != true {
		t.Errorf("my_snatched = %v", body["my_snatched"])
	}

	if res.TotalFound != 42 || len(res.Data) != 1 {
		t.Fatalf("result = %+v", res)
	}
	if res.Data[0].ID != 99 || res.Data[0].Size != "6.0 MiB" || res.Data[0].Seeders != 12 {
		t.Errorf("row = %+v", res.Data[0])
	}
}
