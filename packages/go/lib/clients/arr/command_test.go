package arr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestRunCommandPostsTheBody(t *testing.T) {
	t.Parallel()

	var gotPath, gotMethod string
	var gotBody map[string]any
	c := newTestLidarr(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":165885,"name":"ManualImport","status":"queued"}`))
	}))

	got, err := c.RunCommand(context.Background(),
		NewManualImport([]ManualImportItem{resolved("/a/01.flac")}, ImportCopy))
	if err != nil {
		t.Fatalf("RunCommand() error = %v", err)
	}

	if gotMethod != http.MethodPost || gotPath != "/api/v1/command" {
		t.Errorf("request = %s %s, want POST /api/v1/command", gotMethod, gotPath)
	}
	if gotBody["name"] != "ManualImport" {
		t.Errorf("name = %v, want ManualImport", gotBody["name"])
	}
	if got.ID != 165885 {
		t.Errorf("ID = %d, want 165885", got.ID)
	}
}

func TestCommandStatusReadsOneCommand(t *testing.T) {
	t.Parallel()

	var gotPath string
	c := newTestLidarr(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"id":9,"status":"completed","result":"successful"}`))
	}))

	got, err := c.CommandStatus(context.Background(), 9)
	if err != nil {
		t.Fatalf("CommandStatus() error = %v", err)
	}
	if gotPath != "/api/v1/command/9" {
		t.Errorf("path = %q, want /api/v1/command/9", gotPath)
	}
	if got.Status != CommandCompleted || got.Result != CommandSuccessful {
		t.Errorf("command = %+v", got)
	}
}

func TestCommandStatusReportsServerFailure(t *testing.T) {
	t.Parallel()

	c := newTestLidarr(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	if _, err := c.CommandStatus(context.Background(), 9); err == nil {
		t.Fatal("CommandStatus() error = nil, want a failure")
	}
}

func TestRunCommandReportsServerFailure(t *testing.T) {
	t.Parallel()

	c := newTestLidarr(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	if _, err := c.RunCommand(context.Background(), map[string]string{"name": "X"}); err == nil {
		t.Fatal("RunCommand() error = nil, want a failure")
	}
}

func TestCommandFinished(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		CommandQueued:    false,
		CommandStarted:   false,
		CommandCompleted: true,
		CommandFailed:    true,
		CommandAborted:   true,
		"unknown":        false,
	}
	for status, want := range cases {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			if got := (Command{Status: status}).Finished(); got != want {
				t.Errorf("Finished() = %v, want %v", got, want)
			}
		})
	}
}

// Which of the two fields carries a failure's detail varies by command, so
// both are read before giving up.
func TestCommandDetail(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		command Command
		want    string
	}{
		"exception": {Command{Exception: "disk full"}, "disk full"},
		"message":   {Command{Message: "no files"}, "no files"},
		"both":      {Command{Exception: "disk full", Message: "no files"}, "disk full"},
		"neither":   {Command{}, "no detail given"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := tc.command.Detail(); got != tc.want {
				t.Errorf("Detail() = %q, want %q", got, tc.want)
			}
		})
	}
}
