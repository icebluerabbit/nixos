package qbittorrent

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"testing"
)

// upload is one decoded add request.
type upload struct {
	category    string
	autoTMM     string
	filename    string
	partType    string
	content     string
	contentType string
}

// readUpload decodes the multipart body, which is the only way to check that
// what went on the wire is what qBittorrent expects.
func readUpload(t *testing.T, r *http.Request) upload {
	t.Helper()

	got := upload{contentType: r.Header.Get("Content-Type")}
	_, params, err := mime.ParseMediaType(got.contentType)
	if err != nil {
		t.Fatalf("parse Content-Type: %v", err)
	}

	reader := multipart.NewReader(r.Body, params["boundary"])
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part %q: %v", part.FormName(), err)
		}
		switch part.FormName() {
		case "category":
			got.category = string(body)
		case "autoTMM":
			got.autoTMM = string(body)
		case "torrents":
			got.filename = part.FileName()
			got.partType = part.Header.Get("Content-Type")
			got.content = string(body)
		default:
			t.Errorf("unexpected form field %q", part.FormName())
		}
	}
	return got
}

func TestAddTorrentFileSendsMultipart(t *testing.T) {
	var got upload
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != pathTorrentsAdd {
			t.Errorf("path = %s, want %s", r.URL.Path, pathTorrentsAdd)
		}
		got = readUpload(t, r)
		_, _ = io.WriteString(w, `{"success_count":1}`)
	}))

	accepted, err := c.AddTorrentFile(context.Background(), "Some.Release-GRP.torrent",
		[]byte("d8:announce20:https://t.test/anne"), AddOptions{Category: "tv", AutoTMM: true})
	if err != nil {
		t.Fatalf("AddTorrentFile() error = %v", err)
	}
	if !accepted {
		t.Error("AddTorrentFile() = false, want true")
	}

	if got.category != "tv" {
		t.Errorf("category = %q, want tv", got.category)
	}
	if got.autoTMM != "true" {
		t.Errorf("autoTMM = %q, want true", got.autoTMM)
	}
	if got.filename != "Some.Release-GRP.torrent" {
		t.Errorf("filename = %q", got.filename)
	}
	if got.partType != torrentContentType {
		t.Errorf("file part Content-Type = %q, want %s", got.partType, torrentContentType)
	}
	if got.content != "d8:announce20:https://t.test/anne" {
		t.Errorf("content = %q", got.content)
	}
}

// A torrent already in the client comes back refused. That is the normal
// outcome of re-importing a directory, not a failure.
func TestAddTorrentFileReportsRejection(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success_count":0}`)
	}))

	accepted, err := c.AddTorrentFile(context.Background(), "a.torrent", nil, AddOptions{})
	if err != nil {
		t.Fatalf("AddTorrentFile() error = %v", err)
	}
	if accepted {
		t.Error("AddTorrentFile() = true, want false")
	}
}

func TestAddTorrentFileReportsServerFailure(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "nope")
	}))

	if _, err := c.AddTorrentFile(context.Background(), "a.torrent", nil, AddOptions{}); err == nil {
		t.Fatal("AddTorrentFile() error = nil, want a failure")
	}
}

// The endpoint has had two answers. Before 5.1 it replied with the bare
// string "Ok."; from 5.1 it replies with a JSON summary. Reading the old one
// as a rejection would leave every import to be retried forever.
func TestAddAccepted(t *testing.T) {
	cases := []struct {
		answer string
		want   bool
	}{
		{"Ok.", true},
		{"ok.", true},
		{`{"success_count":1}`, true},
		{`{"success_count":2}`, true},
		{`{"success_count":0}`, false},
		{"Fails.", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.answer, func(t *testing.T) {
			if got := addAccepted(tc.answer); got != tc.want {
				t.Errorf("addAccepted(%q) = %v, want %v", tc.answer, got, tc.want)
			}
		})
	}
}
