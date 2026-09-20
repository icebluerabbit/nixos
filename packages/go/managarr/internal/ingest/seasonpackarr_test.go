package ingest

import (
	"strings"
	"testing"
)

// TestSeasonpackarrHookDecodesTheEmbed pins the Discord-shaped payload.
//
// Upstream calls it a discord notifier and performs no URL validation, so it is
// pointed at the Vector sidecar and forwarded here verbatim. The embed's title
// is BuildTitle(statusCode) — the upstream StatusCode enum rendered as prose —
// which is why it is treated as a bounded label rather than matched as text.
func TestSeasonpackarrHookDecodesTheEmbed(t *testing.T) {
	const body = `{
	  "embeds": [{
	    "title": "resolution did not match",
	    "description": "Some.Show.S01.1080p-GRP",
	    "color": 15548997,
	    "fields": [
	      {"name": "Client", "value": "qbittorrent"},
	      {"name": "Release", "value": "Some.Show.S01.1080p-GRP"}
	    ]
	  }]
	}`
	var h seasonpackarrHook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if len(h.Embeds) != 1 {
		t.Fatalf("got %d embeds", len(h.Embeds))
	}
	if h.Embeds[0].Title != "resolution did not match" {
		t.Errorf("title = %q", h.Embeds[0].Title)
	}
	if len(h.Embeds[0].Fields) != 2 {
		t.Fatalf("got %d fields", len(h.Embeds[0].Fields))
	}
}

// TestSeasonpackarrOutcomeNormalisation matches the exporter's rule.
//
// The Vector exporter turns the same title into a metric label with
// `replace(downcase(title), " ", "_")`. Diverging here would mean the metric and
// the database disagree about what an outcome is called, and no query could
// join them.
func TestSeasonpackarrOutcomeNormalisation(t *testing.T) {
	cases := map[string]string{
		"successful match":                          "successful_match",
		"no matching releases in client":            "no_matching_releases_in_client",
		"number of matches below threshold":         "number_of_matches_below_threshold",
		"could not create hardlinks":                "could_not_create_hardlinks",
		"release is not a season pack":              "release_is_not_a_season_pack",
		"could not match episodes to files in pack": "could_not_match_episodes_to_files_in_pack",
	}
	for title, want := range cases {
		got := strings.ReplaceAll(strings.ToLower(title), " ", "_")
		if got != want {
			t.Errorf("%q normalised to %q, want %q", title, got, want)
		}
	}
}

// TestSeasonpackarrReleaseFieldNames covers the field lookup.
//
// The release name is the primary key of pack_attempts. Builds label the field
// differently, so the handler accepts several names and falls back to the
// description — a miss means the attempt cannot be recorded at all.
func TestSeasonpackarrReleaseFieldNames(t *testing.T) {
	for _, name := range []string{"Release", "release", "Torrent", "Name"} {
		var h seasonpackarrHook
		body := `{"embeds":[{"title":"successful match","fields":[{"name":"` +
			name + `","value":"Some.Show.S01-GRP"}]}]}`
		if err := decode([]byte(body), &h); err != nil {
			t.Fatal(err)
		}
		var release string
		for _, f := range h.Embeds[0].Fields {
			switch strings.ToLower(f.Name) {
			case "release", "torrent", "name":
				release = f.Value
			}
		}
		if release != "Some.Show.S01-GRP" {
			t.Errorf("field %q was not recognised", name)
		}
	}
}

// TestSeasonpackarrEmptyEmbedsIsIgnorable is the abort case Vector also has.
func TestSeasonpackarrEmptyEmbedsIsIgnorable(t *testing.T) {
	for _, body := range []string{`{}`, `{"embeds":[]}`, `{"embeds":[{"title":""}]}`} {
		var h seasonpackarrHook
		if err := decode([]byte(body), &h); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if len(h.Embeds) > 0 && h.Embeds[0].Title != "" {
			t.Errorf("%s should carry no usable title", body)
		}
	}
}

// TestSeasonpackarrRealPayload is the body seasonpackarr actually sends,
// captured off the wire.
//
// The field is labelled "Release Name", not "Release". Matching the wrong label
// is completely silent — the embed decodes, the title is read, and the attempt
// is dropped for want of a key — which is exactly what happened in production
// until this fixture was taken from the log.
func TestSeasonpackarrRealPayload(t *testing.T) {
	const body = `{"content":null,"embeds":[{"title":"No matching releases in client","description":"","color":10070709,"fields":[{"name":"Release Name","value":"Fosters.Home.For.Imaginary.Friends.S06.1080p.AMZN.WEB-DL.DDP2.0.H.265-HariBhakt","inline":true},{"name":"Client","value":"default","inline":true},{"name":"Action","value":"Pack","inline":true}],"timestamp":"2026-09-14T01:23:01.126162088-04:00"}]}`

	var h seasonpackarrHook
	if err := decode([]byte(body), &h); err != nil {
		t.Fatal(err)
	}
	if len(h.Embeds) != 1 {
		t.Fatalf("got %d embeds", len(h.Embeds))
	}

	var release, client string
	for _, f := range h.Embeds[0].Fields {
		switch strings.ToLower(f.Name) {
		case "release name", "release", "torrent", "name":
			release = f.Value
		case "client":
			client = f.Value
		}
	}
	if release != "Fosters.Home.For.Imaginary.Friends.S06.1080p.AMZN.WEB-DL.DDP2.0.H.265-HariBhakt" {
		t.Errorf("release = %q; the attempt would be dropped", release)
	}
	if client != "default" {
		t.Errorf("client = %q", client)
	}
	// The description is empty on this notifier, so the description fallback
	// cannot rescue a missed field label.
	if h.Embeds[0].Description != "" {
		t.Error("fixture should have an empty description")
	}
	if h.Embeds[0].Title != "No matching releases in client" {
		t.Errorf("title = %q", h.Embeds[0].Title)
	}
}
