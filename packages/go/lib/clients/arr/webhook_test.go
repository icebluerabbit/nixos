package arr_test

import (
	"encoding/json"
	"testing"

	"homelab/clients/arr"
)

// The family sends a union — a Grab has a release and no files, an
// ImportComplete has files and a release, a HealthIssue has neither — so every
// section is optional and absence is normal.
func TestWebhookDecodesEverySection(t *testing.T) {
	const body = `{
		"eventType":"Grab","downloadId":"ABC123","downloadClient":"qBittorrent",
		"release":{"releaseTitle":"R","indexer":"tracker","size":123,"releaseGroup":"GRP"},
		"instanceName":"Sonarr","applicationUrl":"https://sonarr.example"
	}`

	var got arr.Webhook
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}

	if got.DownloadID != "ABC123" || got.DownloadClient != "qBittorrent" {
		t.Errorf("webhook = %+v", got)
	}
	if got.Release.ReleaseTitle != "R" || got.Release.Size != 123 {
		t.Errorf("release = %+v", got.Release)
	}
	if got.InstanceName != "Sonarr" {
		t.Errorf("instance = %q", got.InstanceName)
	}
}

// Sonarr sends "Grab" and Lidarr has sent "grab". A case-sensitive comparison
// drops one of them silently.
func TestWebhookEventIsCaseInsensitive(t *testing.T) {
	for _, raw := range []string{"Grab", "grab", "GRAB"} {
		if got := (arr.Webhook{EventType: raw}).Event(); got != arr.WebhookGrab {
			t.Errorf("Event(%q) = %q, want %q", raw, got, arr.WebhookGrab)
		}
	}
}

// Four shapes for the same thing: Sonarr sends episodeFile on Download and
// episodeFiles on ImportComplete, Radarr sends movieFile, Lidarr sends
// trackFiles. Missing one means an import silently records no files.
func TestWebhookFilesCoversEveryShape(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"sonarr download", `{"episodeFile":{"id":1,"path":"/a.mkv"}}`, 1},
		{"sonarr import", `{"episodeFiles":[{"id":1},{"id":2}]}`, 2},
		{"radarr", `{"movieFile":{"id":1,"path":"/m.mkv"}}`, 1},
		{"lidarr", `{"trackFiles":[{"id":1},{"id":2},{"id":3}]}`, 3},
		{"a grab has none", `{"eventType":"Grab","release":{"releaseTitle":"R"}}`, 0},
		{"health has none", `{"eventType":"Health","level":"warning"}`, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var w arr.Webhook
			if err := json.Unmarshal([]byte(tc.body), &w); err != nil {
				t.Fatal(err)
			}
			if got := len(w.Files()); got != tc.want {
				t.Errorf("Files() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestWebhookFilePreservesFields(t *testing.T) {
	var w arr.Webhook
	err := json.Unmarshal([]byte(
		`{"episodeFile":{"id":7,"path":"/a.mkv","size":99,"sceneName":"S"}}`), &w)
	if err != nil {
		t.Fatal(err)
	}

	files := w.Files()
	if len(files) != 1 {
		t.Fatalf("got %d files", len(files))
	}
	want := arr.WebhookFile{ID: 7, Path: "/a.mkv", Size: 99, SceneName: "S"}
	if files[0] != want {
		t.Errorf("file = %+v, want %+v", files[0], want)
	}
}

// A notification carrying both a singular and a plural file must yield both
// rather than picking one.
func TestWebhookFilesCombinesShapes(t *testing.T) {
	var w arr.Webhook
	err := json.Unmarshal([]byte(
		`{"episodeFile":{"id":1},"episodeFiles":[{"id":2},{"id":3}]}`), &w)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(w.Files()); got != 3 {
		t.Errorf("Files() = %d, want 3", got)
	}
}
