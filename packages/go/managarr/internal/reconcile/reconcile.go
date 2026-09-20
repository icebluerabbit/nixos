// Package reconcile brings the database in line with what the stack actually
// holds, by reading full snapshots rather than waiting for events.
//
// Snapshots are authoritative and events are enrichment, in that order. A
// webhook missed while this service restarts is gone — that happened during
// development — so anything the model must be right about has to be derivable
// from a snapshot. Events only make it current sooner.
package reconcile

import "time"

// Pointer helpers for nullable columns.
//
// A nullable column takes a pointer so "absent" and "present but zero" stay
// distinguishable. Zero counts as absent for the identifiers and counts this
// model carries: an unparsed season or a zero elapsed time is the service
// saying "not applicable", not a measurement.

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtr(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func int64Ptr(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func floatPtr(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}

func boolPtr(v bool) *bool { return &v }

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

// unixTime converts a client's epoch seconds, treating <= 0 as unset.
func unixTime(sec int64) time.Time {
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}
