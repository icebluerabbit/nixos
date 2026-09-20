package slskd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"homelab/httpx"
)

// TestTimeAcceptsBothForms pins the inconsistency that motivated the Time type.
//
// slskd sends an offset on /searches and none on /transfers/downloads. A plain
// time.Time field decodes the first and fails the second, and because the
// failure happens mid-array it aborts the entire listing rather than dropping
// one record.
func TestTimeAcceptsBothForms(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string // RFC 3339, or "" for the zero time
		wantErr bool
	}{
		{
			name: "zoneless as sent by transfers",
			in:   `"2026-09-13T18:39:17.8220349"`,
			want: "2026-09-13T18:39:17Z",
		},
		{
			name: "zoneless without fraction",
			in:   `"2026-09-13T18:39:17"`,
			want: "2026-09-13T18:39:17Z",
		},
		{
			name: "offset as sent by searches",
			in:   `"2026-09-13T18:39:17.8220349-04:00"`,
			want: "2026-09-13T22:39:17Z",
		},
		{name: "null is the zero time", in: `null`},
		{name: "empty is the zero time", in: `""`},
		{name: "garbage is an error", in: `"not a time"`, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got Time
			err := json.Unmarshal([]byte(tc.in), &got)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("decoding %s: want error, got %v", tc.in, got.Time)
				}
				return
			}
			if err != nil {
				t.Fatalf("decoding %s: %v", tc.in, err)
			}
			if tc.want == "" {
				if !got.IsZero() {
					t.Fatalf("decoding %s: want zero time, got %v", tc.in, got.Time)
				}
				return
			}
			if utc := got.UTC().Format("2006-01-02T15:04:05Z"); utc != tc.want {
				t.Errorf("decoding %s: got %s, want %s", tc.in, utc, tc.want)
			}
		})
	}
}

// TestDownloadFileDecodes runs a real /transfers/downloads record through the
// struct, which is what actually broke: the timestamps are nested three levels
// down, so a unit test on Time alone would not have caught a missed field.
func TestDownloadFileDecodes(t *testing.T) {
	const body = `[{
	  "username": "somepeer",
	  "directories": [{
	    "directory": "@@music\\Artist\\Album",
	    "fileCount": 1,
	    "files": [{
	      "id": "8f0e1e1a-0000-4000-8000-000000000000",
	      "filename": "@@music\\Artist\\Album\\01 Track.flac",
	      "size": 38221344,
	      "bytesTransferred": 38221344,
	      "state": "Completed, Succeeded",
	      "requestedAt": "2026-09-13T18:39:17.8220349",
	      "startedAt": "2026-09-13T18:39:18.1030221",
	      "endedAt": "2026-09-13T18:39:44.9910007"
	    }]
	  }]
	}]`

	var got []Download
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("decoding downloads: %v", err)
	}
	if len(got) != 1 || len(got[0].Directory) != 1 || len(got[0].Directory[0].Files) != 1 {
		t.Fatalf("got %+v, want one file under one directory under one peer", got)
	}
	f := got[0].Directory[0].Files[0]
	if f.EndedAt.IsZero() {
		t.Error("endedAt decoded as the zero time")
	}
	if f.BytesTransferred != f.Size {
		t.Errorf("bytesTransferred %d, size %d: want equal for a succeeded transfer", f.BytesTransferred, f.Size)
	}
}

// TestEventDecodesTheDoubleEncodedPayload is the shape trap.
//
// slskd stores the serialised payload in a text column and hands it back
// verbatim, so `data` is a JSON document encoded as a STRING rather than an
// object. Decoding it as an object fails, and the log is the only place a
// download's local path is ever stated.
func TestEventDecodesTheDoubleEncodedPayload(t *testing.T) {
	// Verbatim from this homelab's slskd, trimmed to the fields read here.
	const body = `[{
	  "timestamp": "2026-09-14T03:57:47.3665807Z",
	  "type": "DownloadFileComplete",
	  "id": "182b9507-1497-4b33-9ec5-f87853bd1218",
	  "data": "{\"version\":0,\"localFilename\":\"/mnt/media/complete/soulseek/Some Album/01.flac\",\"remoteFilename\":\"Music\\\\Artist\\\\Some Album\\\\01.flac\",\"transfer\":{\"id\":\"4cd38930\",\"username\":\"elric\",\"direction\":\"Download\",\"filename\":\"Music\\\\Artist\\\\Some Album\\\\01.flac\",\"size\":389776,\"state\":\"Completed, Succeeded\",\"requestedAt\":\"2026-09-14T03:54:34.7022544\",\"startedAt\":\"2026-09-14T03:57:47.2677683Z\",\"endedAt\":\"2026-09-14T03:57:47.3665807Z\",\"bytesTransferred\":389776}}"
	}]`

	var events []Event
	if err := json.Unmarshal([]byte(body), &events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != EventDownloadFileComplete {
		t.Fatalf("events = %+v", events)
	}

	done, err := events[0].AsFileComplete()
	if err != nil {
		t.Fatal(err)
	}
	if done.LocalFilename != "/mnt/media/complete/soulseek/Some Album/01.flac" {
		t.Errorf("local = %q; this is the field nothing else reports", done.LocalFilename)
	}
	// The remote path is the natural key of a download and is backslashed.
	if done.RemoteFilename != `Music\Artist\Some Album\01.flac` {
		t.Errorf("remote = %q", done.RemoteFilename)
	}
	// Username lives on the nested transfer here, not at the top level.
	if done.Peer() != "elric" {
		t.Errorf("peer = %q", done.Peer())
	}
	if done.Transfer.Size != 389776 || done.Transfer.State != "Completed, Succeeded" {
		t.Errorf("transfer = %+v", done.Transfer)
	}
	// requestedAt has no zone while the others do — the reason Time exists.
	if done.Transfer.RequestedAt.IsZero() || done.Transfer.EndedAt.IsZero() {
		t.Errorf("timestamps did not parse: %+v", done.Transfer)
	}
}

// TestEventsPagesByOffset walks to a short page.
func TestEventsPagesByOffset(t *testing.T) {
	var offsets []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offsets = append(offsets, r.URL.Query().Get("offset"))
		switch r.URL.Query().Get("offset") {
		case "0":
			_, _ = w.Write([]byte(`[{"id":"a","type":"X","data":"{}"},{"id":"b","type":"X","data":"{}"}]`))
		default:
			_, _ = w.Write([]byte(`[{"id":"c","type":"X","data":"{}"}]`))
		}
	}))
	defer srv.Close()

	c, err := New(httpx.Config{BaseURL: srv.URL, RetryMax: -1})
	if err != nil {
		t.Fatal(err)
	}

	var seen int
	if err := c.Events(context.Background(), 2, func(page []Event) bool {
		seen += len(page)
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if len(offsets) != 2 || offsets[0] != "0" || offsets[1] != "2" {
		t.Errorf("offsets = %v, want [0 2]", offsets)
	}
	if seen != 3 {
		t.Errorf("saw %d events, want 3", seen)
	}
}

// TestEventsStopsWhenTheCallbackSaysSo is how an incremental pass ends.
//
// The full log is tens of megabytes because every download event embeds its
// whole transfer object, so re-reading it hourly would be the most expensive
// thing the service does.
func TestEventsStopsWhenTheCallbackSaysSo(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`[{"id":"a","type":"X","data":"{}"},{"id":"b","type":"X","data":"{}"}]`))
	}))
	defer srv.Close()

	c, err := New(httpx.Config{BaseURL: srv.URL, RetryMax: -1})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Events(context.Background(), 2, func([]Event) bool { return false }); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Errorf("made %d requests after being told to stop", requests)
	}
}

// TestAsFileCompleteRejectsGarbage keeps one bad row from killing a walk.
func TestAsFileCompleteRejectsGarbage(t *testing.T) {
	e := Event{ID: "x", Type: EventDownloadFileComplete, Data: "{not json"}
	if _, err := e.AsFileComplete(); err == nil {
		t.Fatal("undecodable payload did not error")
	}
}
