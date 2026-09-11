package parse_test

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"homelab/stashtrap/internal/parse"
	"homelab/stashtrap/internal/store"
)

// Environment for the corpus run. Absent, the test skips: the recorded
// announces live on the server, not in the repo, and a unit test suite that
// needs a 200 MB database to run is no longer a unit test suite.
const (
	envDB       = "STASHTRAP_CORPUS_DB"
	envEndpoint = "STASHTRAP_CORPUS_ENDPOINT"
	envSamples  = "STASHTRAP_CORPUS_SAMPLES"
	// envOut names a file to write one JSON object per release to. Hit rates
	// say how often the parser produced an answer; only checking those
	// answers against the service says whether they were right, and that
	// check needs the answers in a form something else can read.
	envOut = "STASHTRAP_CORPUS_OUT"
	// envSplit selects half the corpus: "tune" or "holdout".
	//
	// A pattern set adjusted until it scores well on the names it was
	// adjusted against has learned those names, not the conventions behind
	// them — and the service will spend its life on names nobody has seen.
	// Tune on one half, believe only the other.
	envSplit = "STASHTRAP_CORPUS_SPLIT"
)

const defaultEndpoint = "https://stashdb.org/graphql"

// Buckets a release falls into once scored.
const (
	bucketPerformer = "performer"
	bucketAmbiguous = "ambiguous"
	bucketStudio    = "studio-only"
	bucketNone      = "no match"
)

// TestCorpus reports what the parser makes of every announce recorded so far.
//
// This is a measurement, not an assertion. There is no correct hit rate to
// check against — the point is that changing a pattern or a weight becomes an
// experiment with a number attached, rather than reading a handful of examples
// and calling them convincing. It prints per-bucket counts, which pattern won,
// and samples of each outcome including the failures.
//
// Point it at a *copy*: opening a store runs migrations, and the live database
// belongs to the service.
//
//	STASHTRAP_CORPUS_DB=/tmp/corpus.db go test -run TestCorpus -v ./internal/parse/
func TestCorpus(t *testing.T) {
	path := os.Getenv(envDB)
	if path == "" {
		t.Skipf("set %s to a copy of a stashtrap database to run the corpus report", envDB)
	}
	endpoint := os.Getenv(envEndpoint)
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	samples := 10
	if raw := os.Getenv(envSamples); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("%s=%q: %v", envSamples, raw, err)
		}
		samples = n
	}

	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open(%q): %v", path, err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	names, err := st.AnnounceNames(ctx)
	if err != nil {
		t.Fatalf("AnnounceNames: %v", err)
	}
	if len(names) == 0 {
		t.Fatalf("%s holds no announces", path)
	}

	if split := os.Getenv(envSplit); split != "" {
		before := len(names)
		names, err = half(names, split)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("split %q: %d of %d announces", split, len(names), before)
	}

	performers, studios, scenes, err := st.CatalogueCounts(ctx, endpoint)
	if err != nil {
		t.Fatalf("CatalogueCounts: %v", err)
	}
	t.Logf("catalogue at %s: %d performers, %d studios, %d scenes", endpoint, performers, studios, scenes)

	var out *json.Encoder
	if path := os.Getenv(envOut); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
		defer func() { _ = f.Close() }()
		out = json.NewEncoder(f)
	}

	catalogue := st.NewCatalogue(endpoint)
	counts := map[string]int{}
	byPattern := map[string]int{}
	examples := map[string][]string{}
	chunked := 0

	for _, name := range names {
		if len(parse.Read(name).Chunks) > 0 {
			chunked++
		}

		res, ok, err := parse.Identify(ctx, catalogue, store.Normalise, name)
		if err != nil {
			t.Fatalf("Identify(%q): %v", name, err)
		}

		bucket := bucketOf(res, ok)
		counts[bucket]++
		if ok {
			byPattern[evidence(res)]++
		}
		if len(examples[bucket]) < samples {
			examples[bucket] = append(examples[bucket], fmt.Sprintf("%s\n      %s", name, describe(res, ok)))
		}

		if out != nil {
			if err := out.Encode(record{
				Name:         name,
				Bucket:       bucket,
				Pattern:      evidence(res),
				Score:        res.Score,
				Studio:       strings.Join(res.StudioIDs, ","),
				StudioIDs:    res.StudioIDs,
				Performers:   strings.Join(res.Names(), ", "),
				PerformerIDs: res.PerformerIDs,
				Date:         res.Date,
				Title:        title(res),
				SceneID:      res.SceneID,
				Corroborated: res.Corroborated,
			}); err != nil {
				t.Fatalf("write result: %v", err)
			}
		}
	}

	total := len(names)
	t.Logf("announces %d", total)
	t.Logf("  chunked             %s", share(chunked, total))
	for _, bucket := range []string{bucketPerformer, bucketAmbiguous, bucketStudio, bucketNone} {
		t.Logf("  %-19s %s", bucket, share(counts[bucket], total))
	}

	t.Log("winning pattern:")
	for _, p := range ranked(byPattern) {
		t.Logf("  %-26s %d", p.key, p.n)
	}

	for _, bucket := range []string{bucketPerformer, bucketAmbiguous, bucketStudio, bucketNone} {
		if len(examples[bucket]) == 0 {
			continue
		}
		t.Logf("%s examples:", bucket)
		for _, e := range examples[bucket] {
			t.Logf("    %s", e)
		}
	}
}

// half splits the corpus deterministically by a hash of the name, so the two
// halves stay the same set as announces are added and a number from one run is
// comparable with a number from the next.
func half(names []string, split string) ([]string, error) {
	var want uint32
	switch split {
	case "tune":
		want = 0
	case "holdout":
		want = 1
	default:
		return nil, fmt.Errorf("%s=%q: want \"tune\" or \"holdout\"", envSplit, split)
	}

	var out []string
	for _, n := range names {
		h := fnv.New32a()
		_, _ = h.Write([]byte(n))
		if h.Sum32()%2 == want {
			out = append(out, n)
		}
	}
	return out, nil
}

// record is one release and what the parser made of it, for the verifier.
type record struct {
	Name         string   `json:"name"`
	Bucket       string   `json:"bucket"`
	Pattern      string   `json:"pattern,omitempty"`
	Score        float64  `json:"score"`
	Studio       string   `json:"studio,omitempty"`
	StudioIDs    []string `json:"studio_ids,omitempty"`
	Performers   string   `json:"performers,omitempty"`
	PerformerIDs []string `json:"performer_ids,omitempty"`
	Date         string   `json:"date,omitempty"`
	Title        string   `json:"title,omitempty"`
	SceneID      string   `json:"scene_id,omitempty"`
	Corroborated bool     `json:"corroborated"`
}

// evidence names what the reading rested on: which parts of the name the
// catalogue recognised, and whether they agreed with each other.
func evidence(res parse.Result) string {
	switch {
	case res.SceneID != "" && res.Corroborated:
		return "scene (corroborated)"
	case res.SceneID != "":
		return "scene"
	case len(res.PerformerIDs) > 0 && len(res.StudioIDs) > 0:
		return "studio+performer"
	case len(res.PerformerIDs) > 0:
		return "performer"
	case len(res.StudioIDs) > 0:
		return "studio"
	default:
		return "none"
	}
}

// title is what the verifier should try to identify the release by: the body,
// which is the name with the header, date and technical tokens stripped.
func title(res parse.Result) string {
	return res.Body
}

func bucketOf(res parse.Result, ok bool) string {
	switch {
	case !ok:
		return bucketNone
	case len(res.PerformerIDs) > 1:
		return bucketAmbiguous
	case len(res.PerformerIDs) == 1:
		return bucketPerformer
	default:
		return bucketStudio
	}
}

func describe(res parse.Result, ok bool) string {
	if !ok {
		return "no reading resolved"
	}
	return res.String()
}

func share(n, total int) string {
	return fmt.Sprintf("%4d (%2.0f%%)", n, 100*float64(n)/float64(total))
}

type tally struct {
	key string
	n   int
}

func ranked(counts map[string]int) []tally {
	out := make([]tally, 0, len(counts))
	for k, n := range counts {
		out = append(out, tally{k, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].n != out[j].n {
			return out[i].n > out[j].n
		}
		return out[i].key < out[j].key
	})
	return out
}
