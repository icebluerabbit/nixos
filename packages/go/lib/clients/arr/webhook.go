package arr

import "strings"

// Webhook is the Servarr notification body, over the fields worth keeping.
//
// One struct for all three apps and every event type. The family sends a union
// — a Grab has a release and no files, an ImportComplete has files and a
// release, a HealthIssue has neither — so absence is normal and every section
// is optional.
//
// Shared rather than redeclared per consumer: managarr-sonarr decodes this to
// enrich, and managarr decodes the same bytes again on the far side. Two copies
// of the struct would both keep compiling after a field rename and both go
// silently empty.
type Webhook struct {
	EventType string `json:"eventType"`
	// DownloadID is the infohash, uppercase. Present on grab and import.
	DownloadID     string `json:"downloadId"`
	DownloadClient string `json:"downloadClient"`
	Release        struct {
		ReleaseTitle string `json:"releaseTitle"`
		Indexer      string `json:"indexer"`
		Size         int64  `json:"size"`
		ReleaseGroup string `json:"releaseGroup"`
	} `json:"release"`

	// Import events. Sonarr sends episodeFile (singular) on Download and
	// episodeFiles (plural) on ImportComplete; Radarr sends movieFile; Lidarr
	// sends trackFiles. All of them are the same shape underneath.
	EpisodeFile  *WebhookFile  `json:"episodeFile"`
	EpisodeFiles []WebhookFile `json:"episodeFiles"`
	MovieFile    *WebhookFile  `json:"movieFile"`
	TrackFiles   []WebhookFile `json:"trackFiles"`

	// Health events.
	Level   string `json:"level"`
	Message string `json:"message"`
	Type    string `json:"type"`
	WikiURL string `json:"wikiUrl"`

	InstanceName   string `json:"instanceName"`
	ApplicationURL string `json:"applicationUrl"`
}

// WebhookFile is one imported file as a notification reports it.
type WebhookFile struct {
	ID        int64  `json:"id"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SceneName string `json:"sceneName"`
}

// Webhook event types the family sends.
//
// Compared case-insensitively: Sonarr sends "Grab" while Lidarr has sent
// "grab", and a mismatch here is silent — the event is simply ignored.
const (
	WebhookGrab           = "grab"
	WebhookDownload       = "download"
	WebhookImportComplete = "importcomplete"
	WebhookHealthIssue    = "health"
	WebhookHealthRestored = "healthrestored"
)

// Event returns the normalised event type.
func (w Webhook) Event() string { return strings.ToLower(w.EventType) }

// Files returns every imported file the notification carries, whichever of the
// four shapes the app used.
func (w Webhook) Files() []WebhookFile {
	var out []WebhookFile
	if w.EpisodeFile != nil {
		out = append(out, *w.EpisodeFile)
	}
	if w.MovieFile != nil {
		out = append(out, *w.MovieFile)
	}
	out = append(out, w.EpisodeFiles...)
	out = append(out, w.TrackFiles...)
	return out
}
