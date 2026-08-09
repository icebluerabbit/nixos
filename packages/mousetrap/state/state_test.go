package state

import (
	"testing"

	"mousetrap/mam"
)

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"":          0,
		"6.0 MiB":   6 * (1 << 20),
		"1.50 TiB":  int64(1.5 * (1 << 40)),
		"1,024 KiB": 1024 * (1 << 10),
		"500 B":     500,
		"2 GiB":     2 * (1 << 30),
		"garbage":   0,
		"10":        10, // bare number → bytes
		"1.5 GB":    1_500_000_000,
	}
	for in, want := range cases {
		if got := parseSize(in); got != want {
			t.Errorf("parseSize(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestUserToAccount(t *testing.T) {
	u := &mam.User{
		ClassName:  "Power User",
		Ratio:      "14.42",
		Uploaded:   "2.00 TiB",
		Downloaded: "1.00 TiB",
		Wedges:     12,
		Unsat:      &mam.SnatchCount{Count: 15, Limit: 20},
	}
	a := userToAccount(u)
	if a.ClassName != "Power User" || a.Wedges != 12 {
		t.Errorf("class/wedges = %q/%d", a.ClassName, a.Wedges)
	}
	if a.Ratio != 14.42 {
		t.Errorf("ratio = %v", a.Ratio)
	}
	if a.UnsatCount != 15 || a.UnsatLimit != 20 {
		t.Errorf("unsat = %d/%d", a.UnsatCount, a.UnsatLimit)
	}
	if a.UploadedBytes != 2*(1<<40) || a.DownloadedBytes != 1*(1<<40) {
		t.Errorf("up/down = %d/%d", a.UploadedBytes, a.DownloadedBytes)
	}
	if a.Buffer() != 1*(1<<40) {
		t.Errorf("buffer = %d, want 1 TiB", a.Buffer())
	}
	if a.VIP() {
		t.Error("Power User should not be VIP")
	}
}
