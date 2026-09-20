// Package redact strips credentials out of text before it is stored or logged.
//
// Three subsystems need this and none of them should own it: the reconcile
// passes copy URLs out of service APIs, the ingest handlers copy them out of
// webhook bodies, and the HTTP server logs request bodies verbatim. It lived in
// internal/reconcile first, which is why the log path leaked for as long as it
// did — the function existed and was unreachable from the package that needed
// it most.
//
// What counts as a credential here is specific rather than general. Private
// trackers authenticate a download with a per-account key, and this stack has
// seen it in three shapes:
//
//	?passkey=<hex>                       query parameter
//	/rss/download/<id>/<20-hex>/         path segment      (TorrentLeech)
//	/torrent/download/<id>.<32-hex>      dot-suffixed      (BitPorn)
//	https://tracker.tld/<32-hex>/announce path segment      (announce URLs)
//
// The last is the important one: for most private trackers the announce URL IS
// the passkey, so a tracker list is a credential store.
package redact

import "regexp"

// Placeholder replaces a redacted value. Fixed rather than length-preserving so
// a redacted column is obvious at a glance in psql.
const Placeholder = "REDACTED"

// reSecretParam matches a credential-bearing query parameter and its value.
//
// Deliberately narrow about what it does NOT match: id=, torrent_id= and
// tmdbid= are provenance, and redacting them would destroy the ability to say
// what a record refers to.
var reSecretParam = regexp.MustCompile(
	`(?i)([?&](?:passkey|torrent_pass|torrent_password|authkey|auth|apikey|api_key|api_token|rss_key|rsskey|secret|token|session|cookie)=)[^&\s"']+`)

// reHexToken matches a long hexadecimal run, which is the shape every
// path-embedded tracker key here takes.
//
// 20 is the threshold: real path components — numeric ids, slugs, short
// hashes — are shorter, and every observed key is 20 or more.
var reHexToken = regexp.MustCompile(`[A-Fa-f0-9]{20,}`)

// reBtih matches the info-hash component of a magnet URI.
//
// A v1 infohash is 40 hex and would otherwise be eaten by reHexToken. It is
// not a secret — it is the torrent's identity and the join key to downloads —
// so it is protected from redaction rather than subjected to it.
var reBtih = regexp.MustCompile(`(?i)(xt=urn:btih:)([A-Za-z0-9]+)`)

// Secrets strips credential query-parameter values from free text.
//
// Use for error messages and anything that is not specifically a URL. For URL
// fields use URL, which also handles the path forms.
func Secrets(s string) string {
	if s == "" {
		return s
	}
	return reSecretParam.ReplaceAllString(s, "${1}"+Placeholder)
}

// URL strips credentials from a URL, in the query string and in the path.
//
// The magnet info-hash is preserved: it is an identifier, not a secret, and
// losing it would break the link from a magnet to its download.
func URL(s string) string {
	if s == "" {
		return s
	}
	// Protect the btih before the blanket hex pass, then restore it.
	const guard = "\x00BTIH\x00"
	var saved string
	s = reBtih.ReplaceAllStringFunc(s, func(m string) string {
		parts := reBtih.FindStringSubmatch(m)
		saved = parts[2]
		return parts[1] + guard
	})

	s = reHexToken.ReplaceAllString(Secrets(s), Placeholder)

	if saved != "" {
		s = replaceOnce(s, guard, saved)
	}
	return s
}

// Text strips credentials from arbitrary text that may contain URLs.
//
// Applied to webhook bodies before logging: the body is JSON rather than a
// single URL, but tracker download links are embedded in it as values.
func Text(s string) string {
	if s == "" {
		return s
	}
	return reHexToken.ReplaceAllString(Secrets(s), Placeholder)
}

// replaceOnce replaces the first occurrence of old with replacement.
func replaceOnce(s, old, replacement string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + replacement + s[i+len(old):]
		}
	}
	return s
}
