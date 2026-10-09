package mam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetDynamicSeedboxSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Success":true,"msg":"Completed","ip":"10.2.3.4","ASN":1234,"AS":"Org"}`))
	}))
	defer srv.Close()

	c := New("http://unused", "", &memSession{id: "x"}, WithTrackerUrl(srv.URL))
	res, err := c.SetDynamicSeedbox(context.Background())
	if err != nil {
		t.Fatalf("SetDynamicSeedbox: %v", err)
	}
	if !res.Success || res.Msg != SeedboxCompleted {
		t.Errorf("res = %+v", res)
	}
}

// A 403/429 carries a JSON body and is a normal outcome, not a transport error.
func TestSetDynamicSeedboxForbiddenParsesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"Success":false,"msg":"Invalid session - IP mismatch","ip":"10.2.3.4","ASN":1234,"AS":"Org"}`))
	}))
	defer srv.Close()

	c := New("http://unused", "", &memSession{id: "x"}, WithTrackerUrl(srv.URL))
	res, err := c.SetDynamicSeedbox(context.Background())
	if err != nil {
		t.Fatalf("SetDynamicSeedbox should not error on 403: %v", err)
	}
	if res.Success || res.Msg != SeedboxIPMismatch {
		t.Errorf("res = %+v", res)
	}
}

func TestSeedboxOutcomes(t *testing.T) {
	cases := map[SeedboxMsg]int{
		SeedboxNoChange:      http.StatusOK,
		SeedboxCompleted:     http.StatusOK,
		SeedboxTooRecent:     http.StatusTooManyRequests,
		SeedboxNoCookie:      http.StatusForbidden,
		SeedboxNonAPISession: http.StatusForbidden,
	}
	for msg, code := range cases {
		o, ok := SeedboxOutcomes[msg]
		if !ok {
			t.Errorf("no outcome for %q", msg)
			continue
		}
		if o.Code != code {
			t.Errorf("%q code = %d, want %d", msg, o.Code, code)
		}
	}
}
