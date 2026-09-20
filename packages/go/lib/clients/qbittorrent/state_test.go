package qbittorrent_test

import (
	"testing"

	"homelab/clients/qbittorrent"
)

// Every state the serializer can emit, from
// src/webui/api/serialize/serialize_torrent.cpp — torrentStateToString.
//
// The vocabulary is not what the documentation implies: current builds emit
// "stoppedUP"/"stoppedDL" where older ones and most write-ups say "pausedUP"
// and "pausedDL". Matching on the wrong spelling finds nothing and says
// nothing.
func TestKnownStatesCoverSerializer(t *testing.T) {
	t.Parallel()

	for _, s := range []string{
		"error", "missingFiles", "uploading", "stoppedUP", "queuedUP",
		"stalledUP", "checkingUP", "forcedUP", "downloading", "metaDL",
		"forcedMetaDL", "stoppedDL", "queuedDL", "stalledDL", "checkingDL",
		"forcedDL", "checkingResumeData", "moving", "unknown",
	} {
		if !qbittorrent.IsKnownState(s) {
			t.Errorf("state %q not recognised", s)
		}
	}
}

// A state qBittorrent might add later must read as unknown so the reconcile can
// flag it, rather than being stored as though it were understood.
func TestUnrecognisedState(t *testing.T) {
	t.Parallel()

	for _, s := range []string{"pausedUP", "pausedDL", "somethingNew", ""} {
		if qbittorrent.IsKnownState(s) {
			t.Errorf("state %q recognised, want unknown", s)
		}
	}
}

// Seeding is the set of upload-side states, not a substring test: "uploading"
// has no "UP" suffix and "stoppedUP" does.
func TestIsSeeding(t *testing.T) {
	t.Parallel()

	seeding := map[string]bool{
		"uploading": true, "stalledUP": true, "queuedUP": true,
		"forcedUP": true, "checkingUP": true,

		"stoppedUP": false, "downloading": false, "stalledDL": false,
		"error": false, "missingFiles": false, "moving": false,
		"checkingResumeData": false, "metaDL": false,
	}
	for state, want := range seeding {
		if got := qbittorrent.IsSeeding(state); got != want {
			t.Errorf("IsSeeding(%q) = %v, want %v", state, got, want)
		}
	}
}

// The tracker states are their own enum in trackerentrystatus.h —
// NotContacted 1, Working 2, NotWorking 4, TrackerError 5, Unreachable 6.
// 0 and 3 are WebUI-layer values: "disabled" for the DHT/PeX/LSD pseudo-entries
// and "updating" derived from a separate isUpdating flag. There is no 3 in the
// enum at all.
func TestTrackerStatusConstants(t *testing.T) {
	t.Parallel()

	for name, got := range map[string]int{
		"disabled":     qbittorrent.TrackerDisabled,
		"notContacted": qbittorrent.TrackerNotContacted,
		"working":      qbittorrent.TrackerWorking,
		"updating":     qbittorrent.TrackerUpdating,
		"notWorking":   qbittorrent.TrackerNotWorking,
		"trackerError": qbittorrent.TrackerError,
	} {
		want := map[string]int{
			"disabled": 0, "notContacted": 1, "working": 2,
			"updating": 3, "notWorking": 4, "trackerError": 5,
		}[name]
		if got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

// Unregistered is recognised across the phrasings trackers actually use. None
// of this is standardised, so the list is observational — but a torrent the
// tracker has dropped is the single most actionable signal there is.
func TestIsUnregistered(t *testing.T) {
	t.Parallel()

	for msg, want := range map[string]bool{
		"Torrent has been deleted.":                true,
		"Unregistered torrent":                     true,
		"torrent not registered with this tracker": true,
		"Torrent not found":                        true,

		"Connection reset by peer":          false,
		"Tracker Busy":                      false,
		"Rate limit exceeded. Please wait.": false,
		"":                                  false,
	} {
		if got := qbittorrent.IsUnregistered(msg); got != want {
			t.Errorf("IsUnregistered(%q) = %v, want %v", msg, got, want)
		}
	}
}
