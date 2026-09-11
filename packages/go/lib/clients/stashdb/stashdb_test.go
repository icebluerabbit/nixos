package stashdb_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"homelab/clients/stashdb"
	"homelab/secret"
)

const testKey = "stashdb-key"

// request is the GraphQL envelope the client sends, captured so tests can
// assert the operation and its variables rather than only the result.
type request struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// serve stands in for StashDB, replying with body and recording what it was
// asked. body is written verbatim, so a test can return a GraphQL errors array
// with a 200 exactly as the real server does.
func serve(t *testing.T, body string) (*stashdb.Client, *request, *http.Header) {
	t.Helper()

	var got request
	var headers http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	c, err := stashdb.New(stashdb.Config{
		Endpoint: srv.URL,
		APIKey:   secret.New(testKey),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c, &got, &headers
}

func TestNewRequiresAnAPIKey(t *testing.T) {
	t.Parallel()

	if _, err := stashdb.New(stashdb.Config{}); !errors.Is(err, stashdb.ErrNoAPIKey) {
		t.Fatalf("New() error = %v, want %v", err, stashdb.ErrNoAPIKey)
	}
}

// StashDB authenticates with a bare ApiKey header. Authorization: Bearer is
// rejected by the real server, so sending the wrong one would fail every
// request — verified against stashdb.org, and pinned here.
func TestAuthenticatesWithApiKeyHeader(t *testing.T) {
	t.Parallel()

	c, _, headers := serve(t, `{"data":{"searchScenes":{"count":0,"scenes":[]}}}`)

	if _, _, err := c.SearchScenes(context.Background(), "term", 5); err != nil {
		t.Fatalf("SearchScenes() error = %v", err)
	}
	if got := headers.Get("ApiKey"); got != testKey {
		t.Errorf("ApiKey = %q, want %q", got, testKey)
	}
	if got := headers.Get("Authorization"); got != "" {
		t.Errorf("Authorization = %q, want it unset", got)
	}
}

// A release title can carry quotes and backslashes; variables keep it out of
// the query text entirely.
func TestSearchSendsTermAsAVariable(t *testing.T) {
	t.Parallel()

	c, got, _ := serve(t, `{"data":{"searchScenes":{"count":0,"scenes":[]}}}`)

	awkward := `Site.com - "Quoted" \ Title`
	if _, _, err := c.SearchScenes(context.Background(), awkward, 3); err != nil {
		t.Fatalf("SearchScenes() error = %v", err)
	}

	if got.Variables["term"] != awkward {
		t.Errorf("term variable = %v, want %q", got.Variables["term"], awkward)
	}
	if got.Variables["limit"] != float64(3) {
		t.Errorf("limit variable = %v, want 3", got.Variables["limit"])
	}
	if !strings.Contains(got.Query, "searchScenes(term: $term, limit: $limit)") {
		t.Errorf("query does not use variables:\n%s", got.Query)
	}
	if strings.Contains(got.Query, awkward) {
		t.Errorf("term was interpolated into the query:\n%s", got.Query)
	}
}

func TestSearchScenesFlattensTheGraphQLShape(t *testing.T) {
	t.Parallel()

	body := `{"data":{"searchScenes":{"count":1211,"scenes":[
	  {"id":"e030","title":"A Scene","code":"BP-1","release_date":"2012-03-14","duration":1800,
	   "studio":{"name":"Example Studio"},
	   "performers":[{"performer":{"id":"p1","name":"Alice","disambiguation":"II"}},
	                 {"performer":{"id":"p2","name":"Bob","disambiguation":""}}],
	   "urls":[{"url":"https://example.test/1"}]}
	]}}}`

	c, _, _ := serve(t, body)

	scenes, count, err := c.SearchScenes(context.Background(), "term", 0)
	if err != nil {
		t.Fatalf("SearchScenes() error = %v", err)
	}
	if count != 1211 {
		t.Errorf("count = %d, want 1211", count)
	}

	want := []stashdb.Scene{{
		ID:          "e030",
		Title:       "A Scene",
		Code:        "BP-1",
		Studio:      "Example Studio",
		ReleaseDate: "2012-03-14",
		Duration:    1800,
		Performers: []stashdb.Performer{
			{ID: "p1", Name: "Alice", Disambiguation: "II"},
			{ID: "p2", Name: "Bob"},
		},
		URLs: []string{"https://example.test/1"},
	}}
	if diff := cmp.Diff(want, scenes); diff != "" {
		t.Errorf("scenes mismatch (-want +got):\n%s", diff)
	}
}

// StashDB returns null for a scene with no studio and for most codes; neither
// may crash the flattening.
func TestSearchScenesToleratesNulls(t *testing.T) {
	t.Parallel()

	body := `{"data":{"searchScenes":{"count":1,"scenes":[
	  {"id":"x","title":"No Studio","code":null,"release_date":null,
	   "studio":null,"performers":[],"urls":[]}
	]}}}`

	c, _, _ := serve(t, body)

	scenes, _, err := c.SearchScenes(context.Background(), "term", 1)
	if err != nil {
		t.Fatalf("SearchScenes() error = %v", err)
	}
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
	if scenes[0].Studio != "" || scenes[0].Code != "" {
		t.Errorf("null fields did not flatten to empty: %+v", scenes[0])
	}
}

// The failure mode this package exists to prevent: GraphQL answers 200 with an
// errors array, so a client that only checks the status code reports "no
// results" for a query that in fact failed.
func TestGraphQLErrorsOn200AreFailures(t *testing.T) {
	t.Parallel()

	c, _, _ := serve(t, `{"errors":[{"message":"something broke","path":["searchScenes"]}],"data":null}`)

	_, _, err := c.SearchScenes(context.Background(), "term", 1)
	if err == nil {
		t.Fatal("SearchScenes() error = nil, want the GraphQL error surfaced")
	}
	if !strings.Contains(err.Error(), "something broke") {
		t.Errorf("error = %v, want it to carry the server's message", err)
	}
}

// A rejected key is a deployment fault, not a transient one, so it gets its own
// sentinel — retrying will never fix it.
func TestUnauthorizedIsASentinel(t *testing.T) {
	t.Parallel()

	c, _, _ := serve(t, `{"errors":[{"message":"not authorized","path":["searchScenes"]}],"data":null}`)

	if _, _, err := c.SearchScenes(context.Background(), "term", 1); !errors.Is(err, stashdb.ErrUnauthorized) {
		t.Fatalf("SearchScenes() error = %v, want %v", err, stashdb.ErrUnauthorized)
	}
}

func TestSceneByID(t *testing.T) {
	t.Parallel()

	t.Run("found", func(t *testing.T) {
		t.Parallel()

		body := `{"data":{"findScene":{"id":"e030","title":"A Scene","code":"BP-1",
		  "release_date":"2012-03-14","duration":600,"studio":{"name":"S"},
		  "performers":[],"urls":[]}}}`
		c, got, _ := serve(t, body)

		scene, err := c.Scene(context.Background(), "e030")
		if err != nil {
			t.Fatalf("Scene() error = %v", err)
		}
		if scene.ID != "e030" || scene.Studio != "S" {
			t.Errorf("scene = %+v, want id e030 and studio S", scene)
		}
		if got.Variables["id"] != "e030" {
			t.Errorf("id variable = %v, want e030", got.Variables["id"])
		}
	})

	// findScene returns null rather than an error for an unknown id.
	t.Run("unknown id", func(t *testing.T) {
		t.Parallel()

		c, _, _ := serve(t, `{"data":{"findScene":null}}`)

		if _, err := c.Scene(context.Background(), "nope"); !errors.Is(err, stashdb.ErrNotFound) {
			t.Fatalf("Scene() error = %v, want %v", err, stashdb.ErrNotFound)
		}
	})
}

func TestSearchPerformers(t *testing.T) {
	t.Parallel()

	body := `{"data":{"searchPerformers":{"count":2,"performers":[
	  {"id":"p1","name":"Alice","disambiguation":"II"},
	  {"id":"p2","name":"Bob","disambiguation":""}
	]}}}`
	c, _, _ := serve(t, body)

	performers, err := c.SearchPerformers(context.Background(), "alice", 5)
	if err != nil {
		t.Fatalf("SearchPerformers() error = %v", err)
	}
	want := []stashdb.Performer{
		{ID: "p1", Name: "Alice", Disambiguation: "II"},
		{ID: "p2", Name: "Bob"},
	}
	if diff := cmp.Diff(want, performers); diff != "" {
		t.Errorf("performers mismatch (-want +got):\n%s", diff)
	}
}

func TestEmptyTermsAreRejectedBeforeAnyRequest(t *testing.T) {
	t.Parallel()

	c, got, _ := serve(t, `{"data":{}}`)

	if _, _, err := c.SearchScenes(context.Background(), "   ", 1); err == nil {
		t.Error("SearchScenes() error = nil, want a rejection")
	}
	if _, err := c.SearchPerformers(context.Background(), "", 1); err == nil {
		t.Error("SearchPerformers() error = nil, want a rejection")
	}
	if _, err := c.Scene(context.Background(), ""); err == nil {
		t.Error("Scene() error = nil, want a rejection")
	}
	if got.Query != "" {
		t.Errorf("a request was sent despite an empty term: %s", got.Query)
	}
}

// The key is a secret.Value, so it must not be recoverable from the client by
// any ordinary rendering.
func TestClientDoesNotRenderTheKey(t *testing.T) {
	t.Parallel()

	c, _, _ := serve(t, `{"data":{}}`)

	if rendered := fmt.Sprintf("%+v", c); strings.Contains(rendered, testKey) {
		t.Errorf("client rendering leaked the key: %s", rendered)
	}
}
