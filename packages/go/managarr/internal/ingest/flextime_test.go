package ingest

import (
	"encoding/json"
	"testing"
	"time"
)

// The bug this fixes, from the live error.
//
// unpackerr hands Go's time.Time to its notification template, producing
// String() output with a monotonic clock reading on the end. encoding/json
// accepts only RFC3339, so a plain time.Time field failed the ENTIRE
// notification on that one value — every unpackerr event was dropped, about one
// a minute, with the extraction record it carried.
//
// Extractions are the one relationship no later pass can recover: an extracted
// file is a new inode written from an archive, sharing nothing with it, so
// unpackerr is the only witness that the two are connected.
func TestFlexTimeParsesUnpackerrsFormat(t *testing.T) {
	const raw = `"2026-09-15 00:53:00.578075324 -0400 EDT m=+180.010956413"`

	var got flexTime
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("the format unpackerr actually sends must parse: %v", err)
	}
	if got.IsZero() {
		t.Fatal("parsed to the zero time")
	}

	want := time.Date(2026, 9, 15, 4, 53, 0, 578075324, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got.Time, want)
	}
}

func TestFlexTimeAcceptsEveryObservedShape(t *testing.T) {
	tests := []struct {
		name string
		json string
		zero bool
	}{
		{"go String with monotonic", `"2026-09-15 00:53:00.578075324 -0400 EDT m=+180.01"`, false},
		{"go String without monotonic", `"2026-09-15 00:53:00.578075324 -0400 EDT"`, false},
		{"go String whole seconds", `"2026-09-15 00:53:00 -0400 EDT"`, false},
		{"rfc3339", `"2026-09-15T04:53:00Z"`, false},
		{"rfc3339 nano", `"2026-09-15T04:53:00.578075324Z"`, false},
		{"bare local", `"2026-09-15 00:53:00"`, false},
		{"epoch seconds", `1789445030`, false},

		// Absence is not a failure. A queued event has no start time, and
		// losing the extraction over it would be the original bug again.
		{"null", `null`, true},
		{"empty string", `""`, true},
		{"epoch zero", `0`, true},
		{"unrecognised", `"last Tuesday"`, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got flexTime
			if err := json.Unmarshal([]byte(tc.json), &got); err != nil {
				t.Fatalf("%s must not fail the event: %v", tc.json, err)
			}
			if got.IsZero() != tc.zero {
				t.Errorf("IsZero() = %v, want %v (got %v)", got.IsZero(), tc.zero, got.Time)
			}
		})
	}
}

// A value that is neither a string nor a number is a malformed payload rather
// than a missing timestamp, and worth surfacing.
func TestFlexTimeRejectsNonScalars(t *testing.T) {
	for _, raw := range []string{`{"a":1}`, `["x"]`, `true`} {
		var got flexTime
		if err := json.Unmarshal([]byte(raw), &got); err == nil {
			t.Errorf("%s was accepted", raw)
		}
	}
}

// The whole point: the surrounding notification survives.
func TestUnpackerrEventSurvivesItsTimestamp(t *testing.T) {
	const body = `{
	  "path": "/mnt/media/complete/tv/Some.Release-GRP",
	  "app": "Sonarr",
	  "event": "extracted",
	  "ids": {"downloadId": "ABC123", "title": "Some.Release"},
	  "data": {
	    "archives": {"/mnt/media/complete/tv/Some.Release-GRP": ["x.rar"]},
	    "output": "/mnt/media/complete/tv/Some.Release-GRP",
	    "start": "2026-09-15 00:53:00.578075324 -0400 EDT m=+180.010956413",
	    "bytes": 1234
	  }}`

	var hook unpackerrHook
	if err := decode([]byte(body), &hook); err != nil {
		t.Fatalf("the notification was rejected over its timestamp: %v", err)
	}
	if hook.IDs.DownloadID != "ABC123" {
		t.Errorf("downloadId = %q", hook.IDs.DownloadID)
	}
	if hook.Data.Start.IsZero() {
		t.Error("start time was lost")
	}
	if got := hook.archives(); len(got) != 1 {
		t.Errorf("archives = %v", got)
	}
}
