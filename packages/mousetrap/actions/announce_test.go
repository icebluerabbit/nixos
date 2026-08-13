package actions

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mousetrap/app"
	"mousetrap/store"
	"mousetrap/types"
)

// testApp builds a full app pointed at a mock MAM (healthy account: 5/20 unsat) so
// the announce handler's refresh + gate run end-to-end.
func testApp(t *testing.T, accept types.AcceptConfig, grab types.GrabConfig) *app.App {
	t.Helper()
	// mam mock: any path returns the user JSON (also stands in as .torrent bytes for
	// download.php so the added-grab path works).
	mamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"classname":"User","ratio":"10","uploaded":"2.00 TiB","downloaded":"1.00 TiB","wedges":5,"unsat":{"count":5,"limit":20}}`))
	}))
	t.Cleanup(mamSrv.Close)

	qbtSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Ok."))
	}))
	t.Cleanup(qbtSrv.Close)

	seed := filepath.Join(t.TempDir(), "seed")
	if err := os.WriteFile(seed, []byte("initial"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := app.New(types.Config{
		StateDir: t.TempDir(),
		MAM:      types.MAMConfig{APIBase: mamSrv.URL, SeedPath: seed},
		Client:   types.ClientConfig{URL: qbtSrv.URL, Categories: map[string]string{"ebook": "books", "audiobook": "audiobooks"}},
		Accept:   accept,
		Grab:     grab,
	})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func post(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/announce", strings.NewReader(body))
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return m
}

func TestAnnounceReject(t *testing.T) {
	a := testApp(t, types.AcceptConfig{MediaTypes: []string{"Audiobook"}}, types.GrabConfig{})
	w := post(Announce(a), `{"mediaType":"Ebook","torrentId":"5"}`)
	if w.Code != http.StatusNotAcceptable {
		t.Fatalf("status = %d, want 406", w.Code)
	}
	if r := decode(t, w); r["decision"] != string(types.Reject) || r["reason"] != string(types.AcceptMediaType) {
		t.Errorf("body = %v", r)
	}
}

func TestAnnounceAcceptDryRun(t *testing.T) {
	a := testApp(t, types.AcceptConfig{}, types.GrabConfig{UnsatMargin: 5, DryRun: true})
	w := post(Announce(a), `{"mediaType":"Ebook","torrentId":"5","size":"100"}`)

	r := decode(t, w)
	if r["decision"] != string(types.Accept) {
		t.Errorf("decision = %v, want accept", r["decision"])
	}
	if r["result"] != string(types.ResultDryRun) {
		t.Errorf("result = %v, want dryrun", r["result"])
	}
	// dry-run is not an actual grab → 406, and it must not dedup future announces.
	if w.Code != http.StatusNotAcceptable {
		t.Errorf("status = %d, want 406 (dry-run)", w.Code)
	}
	if seen, _ := a.Store.GrabSeen(context.Background(), 5); seen {
		t.Error("dry-run grab must not count as seen")
	}
}

func TestAnnounceGrabAdded(t *testing.T) {
	a := testApp(t, types.AcceptConfig{}, types.GrabConfig{UnsatMargin: 5, DryRun: false})
	w := post(Announce(a), `{"mediaType":"Ebook","torrentId":"7","size":"100"}`)

	r := decode(t, w)
	if r["result"] != string(types.ResultAdded) {
		t.Errorf("result = %v, want added", r["result"])
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if seen, _ := a.Store.GrabSeen(context.Background(), 7); !seen {
		t.Error("added grab should be seen (dedup)")
	}
}

func TestAnnounceDedup(t *testing.T) {
	a := testApp(t, types.AcceptConfig{}, types.GrabConfig{UnsatMargin: 5})
	// a real prior grab of tid 5
	if _, err := a.Store.InsertGrab(context.Background(), store.Grab{
		Source: types.SourceAnnounce, TID: 5, Result: string(types.ResultAdded),
	}); err != nil {
		t.Fatal(err)
	}
	w := post(Announce(a), `{"mediaType":"Ebook","torrentId":"5"}`)
	if r := decode(t, w); r["reason"] != string(types.AcceptDuplicate) {
		t.Errorf("reason = %v, want duplicate", r["reason"])
	}
}

func TestAnnounceBadBody(t *testing.T) {
	a := testApp(t, types.AcceptConfig{}, types.GrabConfig{})
	if w := post(Announce(a), "not json"); w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSplitList(t *testing.T) {
	cases := map[string][]string{
		// MouseBot emits ",  "; MAM's own docs show ", " — both must trim to the same
		"Erotica/Sexual Content,  Romance": {"Erotica/Sexual Content", "Romance"},
		"Fantasy, LGBTQIA+, Romance":       {"Fantasy", "LGBTQIA+", "Romance"},
		"m4b, mp3":                         {"m4b", "mp3"},
		"epub":                             {"epub"},
		"":                                 nil,
		",  ,":                             nil,
	}
	for in, want := range cases {
		got := splitList(in)
		if len(got) != len(want) {
			t.Errorf("splitList(%q) = %q, want %q", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("splitList(%q) = %q, want %q", in, got, want)
				break
			}
		}
	}
}

func TestTruthy(t *testing.T) {
	for _, s := range []string{"true", "1", "yes", "TRUE"} {
		if !truthy(s) {
			t.Errorf("truthy(%q) = false", s)
		}
	}
	for _, s := range []string{"false", "0", "no", ""} {
		if truthy(s) {
			t.Errorf("truthy(%q) = true", s)
		}
	}
}

func TestRelease(t *testing.T) {
	p := announcePayload{
		Title: "Fallback", MediaType: "Audiobook", Categories: "Horror,  Mystery",
		Filetypes: "m4b, mp3", Size: "12345", VIP: "true", Freeleech: "true", TorrentID: "99",
	}
	rel := p.release()
	if rel.Name != "Fallback" || rel.MediaType != "Audiobook" || rel.Size != 12345 ||
		!rel.VIP || !rel.Freeleech || rel.TID != 99 {
		t.Errorf("release = %+v", rel)
	}
	if len(rel.Categories) != 2 || rel.Categories[1] != "Mystery" {
		t.Errorf("categories = %q", rel.Categories)
	}
	if len(rel.Filetypes) != 2 || rel.Filetypes[1] != "mp3" {
		t.Errorf("filetypes = %q", rel.Filetypes)
	}
}

// webhookApp builds an app whose MAM mock returns a healthy account (and doubles as
// .torrent bytes) and whose webhook fires every event, its template emitting just the
// event type so the receiver can record the sequence.
func webhookApp(t *testing.T, hookURL string) *app.App {
	t.Helper()
	mamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"classname":"User","ratio":"10","uploaded":"2.00 TiB","downloaded":"1.00 TiB","wedges":5,"unsat":{"count":5,"limit":20}}`))
	}))
	t.Cleanup(mamSrv.Close)
	qbtSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Ok."))
	}))
	t.Cleanup(qbtSrv.Close)
	seed := filepath.Join(t.TempDir(), "seed")
	if err := os.WriteFile(seed, []byte("initial"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := app.New(types.Config{
		StateDir: t.TempDir(),
		MAM:      types.MAMConfig{APIBase: mamSrv.URL, SeedPath: seed},
		Client:   types.ClientConfig{URL: qbtSrv.URL, Categories: map[string]string{"Ebook": "books"}},
		Accept:   types.AcceptConfig{MediaTypes: []string{"Ebook"}, CategoriesAllow: []string{"Fantasy"}, FiletypesAllow: []string{"epub"}, LanguagesAllow: []string{"english"}},
		Grab:     types.GrabConfig{UnsatMargin: 2, DryRun: false},
		Webhook: types.WebhookConfig{
			URL:      hookURL,
			Template: "{{ .Type }}",
			Events:   []string{"announce", "grab", "reject", "hold", "enqueue", "bonus_received", "spend"},
		},
	})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestAnnounceFiresWebhooks(t *testing.T) {
	var mu sync.Mutex
	var got []string
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, string(b))
		mu.Unlock()
	}))
	defer hook.Close()

	a := webhookApp(t, hook.URL)

	// accepted + grabbed → announce, grab, enqueue
	post(Announce(a), `{"mediaType":"Ebook","categories":"Fantasy","language":["english"],"filetypes":"epub","size":"100","torrentId":"7001","downloadUrl":"x"}`)
	// rejected at Layer-1 (category) → announce, reject
	post(Announce(a), `{"mediaType":"Ebook","categories":"Romance","language":["english"],"filetypes":"epub","size":"100","torrentId":"7002"}`)

	mu.Lock()
	seq := strings.Join(got, ",")
	mu.Unlock()
	for _, want := range []string{"announce", "grab", "enqueue", "reject"} {
		if !strings.Contains(seq, want) {
			t.Errorf("missing %q event; got [%s]", want, seq)
		}
	}
	// grabbed release must NOT emit reject; rejected must NOT emit grab/enqueue.
	if strings.Count(seq, "grab") != 1 || strings.Count(seq, "enqueue") != 1 || strings.Count(seq, "reject") != 1 {
		t.Errorf("event counts off; got [%s]", seq)
	}
}
