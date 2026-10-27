package main

import (
	"context"
	"testing"

	"homelab/qbt-scripts/internal/peers"
)

// The positional command line is the contract with Vector's exec source,
// which passes the WebUI URL and the cap as bare arguments.
func TestParseArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    []string
		base    string
		cap     int
		wantErr bool
	}{
		{"no arguments", nil, defaultBase, peers.DefaultCap, false},
		{"base only", []string{"http://10.10.30.5:8080"}, "http://10.10.30.5:8080", peers.DefaultCap, false},
		{"base and cap", []string{"http://10.10.30.5:8080", "50"}, "http://10.10.30.5:8080", 50, false},
		{"cap is not a number", []string{"http://x:8080", "lots"}, "", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			base, capN, err := parseArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("parseArgs() error = nil, want a failure")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseArgs() error = %v", err)
			}
			if base != tc.base {
				t.Errorf("base = %q, want %q", base, tc.base)
			}
			if capN != tc.cap {
				t.Errorf("cap = %d, want %d", capN, tc.cap)
			}
		})
	}
}

// A base URL the client cannot be built from must still produce a sample, so
// that the gauges read "the scrape failed" rather than going absent.
func TestScrapeReportsAnUnusableBase(t *testing.T) {
	t.Parallel()

	got := scrape(context.Background(), "not a url", peers.DefaultCap)
	if got.ScrapeOK != 0 {
		t.Errorf("ScrapeOK = %d, want 0", got.ScrapeOK)
	}
}
