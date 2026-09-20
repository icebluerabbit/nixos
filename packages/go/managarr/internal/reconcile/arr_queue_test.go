package reconcile

import "testing"

// TestTrackedStatus joins the two halves of the *arr's tracking state.
//
// They answer different questions — status is how bad it is (ok / warning /
// error), state is where it is (downloading / importPending / importBlocked) —
// and either alone is ambiguous. "warning" does not say what is wrong;
// "importBlocked" does not say whether the *arr has given up.
func TestTrackedStatus(t *testing.T) {
	cases := []struct {
		status, state, want string
	}{
		{"warning", "importBlocked", "warning/importBlocked"},
		{"ok", "downloading", "ok/downloading"},
		{"", "downloading", "downloading"},
		{"warning", "", "warning"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := trackedStatus(c.status, c.state); got != c.want {
			t.Errorf("trackedStatus(%q, %q) = %q, want %q",
				c.status, c.state, got, c.want)
		}
	}
}

// statusMessage matches the anonymous struct on arr.QueueItem.
type statusMessage = struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// TestQueueMessagePrefersTheHardError covers the precedence.
//
// errorMessage is set for outright failures; statusMessages carries the softer
// "why is this not importing" text. When both exist the hard error is the one
// worth reading.
func TestQueueMessagePrefersTheHardError(t *testing.T) {
	got := queueMessage("Download client unavailable", []statusMessage{
		{Title: "x", Messages: []string{"soft"}},
	})
	if got != "Download client unavailable" {
		t.Errorf("got %q", got)
	}
}

// TestQueueMessageFlattensStatusMessages is the common case.
//
// This is where the sentence that exists nowhere else lives — "Found archive
// file, might need to be extracted" — so losing it loses the only explanation
// of a stuck queue item.
func TestQueueMessageFlattensStatusMessages(t *testing.T) {
	got := queueMessage("", []statusMessage{
		{Title: "Release", Messages: []string{"Found archive file, might need to be extracted"}},
		{Title: "Other", Messages: []string{"One or more episodes expected in this release were not imported"}},
	})
	const want = "Found archive file, might need to be extracted; " +
		"One or more episodes expected in this release were not imported"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// TestQueueMessageEmpty is the healthy queue item.
func TestQueueMessageEmpty(t *testing.T) {
	if got := queueMessage("", nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	// A status message with an empty Messages array contributes nothing.
	if got := queueMessage("", []statusMessage{{Title: "x"}}); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
