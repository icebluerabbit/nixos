package redact

import (
	"strings"
	"testing"
)

// Every case is a shape actually observed in this stack's data.
func TestURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "query parameter passkey",
			in:   "https://www.retrotoon.world/download.php?id=5874&torrent_pass=6c989124c965afd79064eb75ffb12070",
			want: "https://www.retrotoon.world/download.php?id=5874&torrent_pass=REDACTED",
		},
		{
			name: "torrentleech path segment",
			in:   "https://www.torrentleech.org/rss/download/241798588/f59bbbcd579506fb504e/Name.torrent",
			want: "https://www.torrentleech.org/rss/download/241798588/REDACTED/Name.torrent",
		},
		{
			name: "bitporn dot-suffixed token",
			in:   "https://bitporn.eu/torrent/download/142099.890c21c553c78744dad9881eab3b7098",
			want: "https://bitporn.eu/torrent/download/142099.REDACTED",
		},
		{
			// For most private trackers the announce URL is the credential.
			name: "announce url path key",
			in:   "https://tracker.digitalcore.club/announce/5a846d4b3069b074e8576aec915ae1b2",
			want: "https://tracker.digitalcore.club/announce/REDACTED",
		},
		{
			name: "cross-seed apikey in error text",
			in:   `Post "http://10.10.120.15:2468/api/announce?apikey=2e83eaafc47c735f1576e79": reset`,
			want: `Post "http://10.10.120.15:2468/api/announce?apikey=REDACTED": reset`,
		},
		{
			name: "plain ids survive",
			in:   "https://t.example/details.php?id=5874&torrent_id=99",
			want: "https://t.example/details.php?id=5874&torrent_id=99",
		},
		{name: "empty", in: "", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := URL(tc.in); got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestURLKeepsInfoHash: a magnet's btih is 40 hex and would be eaten by the
// blanket hex pass. It is the torrent's identity and the join key to
// downloads — redacting it would break the model, not protect anything.
func TestURLKeepsInfoHash(t *testing.T) {
	const hash = "f4b87a0c41b9ce644393a46608db8420505f509a"
	in := "magnet:?xt=urn:btih:" + hash +
		"&dn=Some.Release&tr=https://tracker.tld/9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c/announce"

	got := URL(in)
	if !strings.Contains(got, hash) {
		t.Errorf("info hash was redacted: %q", got)
	}
	if strings.Contains(got, "9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c") {
		t.Errorf("tracker key survived: %q", got)
	}
}

// TestTextRedactsEmbeddedURLs covers the webhook-body case: the payload is
// JSON, and the credential is a value inside it.
func TestTextRedactsEmbeddedURLs(t *testing.T) {
	const key = "890c21c553c78744dad9881eab3b7098"
	body := `{"name":"X","url":"https://bitporn.eu/torrent/download/142099.` + key + `"}`

	got := Text(body)
	if strings.Contains(got, key) {
		t.Errorf("key survived: %q", got)
	}
	if !strings.Contains(got, `"name":"X"`) {
		t.Errorf("non-secret content damaged: %q", got)
	}
}

// A redaction that leaves the secret reachable is not a redaction.
func TestNoSecretSurvives(t *testing.T) {
	secrets := []string{
		"6c989124c965afd79064eb75ffb12070",
		"f59bbbcd579506fb504e",
		"2e83eaafc47c735f1576e7933e1432132bd52cb6ab79d646",
	}
	inputs := []string{
		"https://t.tld/dl?passkey=6c989124c965afd79064eb75ffb12070",
		"https://t.tld/rss/download/1/f59bbbcd579506fb504e/x.torrent",
		"http://h:2468/api/announce?apikey=2e83eaafc47c735f1576e7933e1432132bd52cb6ab79d646",
	}
	for i, in := range inputs {
		if got := URL(in); strings.Contains(got, secrets[i]) {
			t.Errorf("secret %d survived: %q", i, got)
		}
	}
}
