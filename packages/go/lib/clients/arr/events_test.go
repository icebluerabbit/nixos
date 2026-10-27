package arr_test

import (
	"testing"

	"homelab/clients/arr"
)

// Every value of every app's history enum must map to a meaning.
//
// Taken from the enum declarations, not from observed traffic:
//
//	Sonarr  src/NzbDrone.Core/History/EpisodeHistory.cs  EpisodeHistoryEventType
//	Radarr  src/NzbDrone.Core/History/History.cs         MovieHistoryEventType
//	Lidarr  src/NzbDrone.Core/History/EntityHistory.cs   EntityHistoryEventType
//
// Lidarr is the one that matters: it has no downloadFolderImported, so code
// written against Sonarr's payloads links nothing for music and says nothing
// about it.
func TestEventKindCoversEveryDeclaredEvent(t *testing.T) {
	t.Parallel()

	cases := map[string]arr.EventKind{
		// Sonarr
		"unknown":                arr.EventUnknown,
		"grabbed":                arr.EventGrabbed,
		"seriesFolderImported":   arr.EventImported,
		"downloadFolderImported": arr.EventImported,
		"downloadFailed":         arr.EventFailed,
		"episodeFileDeleted":     arr.EventDeleted,
		"episodeFileRenamed":     arr.EventRenamed,
		"downloadIgnored":        arr.EventIgnored,

		// Radarr
		"movieFileDeleted":    arr.EventDeleted,
		"movieFolderImported": arr.EventImported,
		"movieFileRenamed":    arr.EventRenamed,

		// Lidarr
		"artistFolderImported":  arr.EventImported,
		"trackFileImported":     arr.EventImported,
		"trackFileDeleted":      arr.EventDeleted,
		"trackFileRenamed":      arr.EventRenamed,
		"albumImportIncomplete": arr.EventImportIncomplete,
		"downloadImported":      arr.EventImported,
		"trackFileRetagged":     arr.EventRetagged,
	}

	for raw, want := range cases {
		got := arr.HistoryRecord{EventType: raw}.Kind()
		if got != want {
			t.Errorf("%q -> %q, want %q", raw, got, want)
		}
	}
}

// An event type nobody has seen must not be silently dropped: it becomes
// EventUnknown, which the reconcile counts.
func TestUnrecognisedEventIsUnknown(t *testing.T) {
	t.Parallel()

	if got := (arr.HistoryRecord{EventType: "somethingNewUpstream"}).Kind(); got != arr.EventUnknown {
		t.Errorf("unrecognised event -> %q, want %q", got, arr.EventUnknown)
	}
}

// Every import spelling has to resolve to EventImported. This is the assertion
// that would have caught music never linking.
func TestAllImportSpellings(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"downloadFolderImported", "seriesFolderImported", "movieFolderImported",
		"artistFolderImported", "trackFileImported", "downloadImported",
	} {
		if got := (arr.HistoryRecord{EventType: raw}).Kind(); got != arr.EventImported {
			t.Errorf("%q -> %q, want imported", raw, got)
		}
	}
}
