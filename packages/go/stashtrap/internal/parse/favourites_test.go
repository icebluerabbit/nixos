package parse_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"homelab/stashtrap/internal/parse"
	"homelab/stashtrap/internal/store"
)

// TestFavourites measures the only thing the decision turns on: whether a
// release featuring a followed performer or studio is recognised as one, and
// whether anything else is mistaken for one.
//
// Everything it needs is local. The followed entities and their aliases are
// mirrored, so nothing here asks StashDB anything — which is what makes it a
// seconds-long check that can run on every change rather than a ten-minute one
// that cannot.
//
//	STASHTRAP_CORPUS_DB=/tmp/corpus.db go test -run TestFavourites -v ./internal/parse/
func TestFavourites(t *testing.T) {
	path := os.Getenv(envDB)
	if path == "" {
		t.Skipf("set %s to a copy of a stashtrap database", envDB)
	}
	endpoint := os.Getenv(envEndpoint)
	if endpoint == "" {
		endpoint = defaultEndpoint
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

	aliases, err := st.FavouriteAliases(ctx, endpoint)
	if err != nil {
		t.Fatalf("FavouriteAliases: %v", err)
	}
	net := newNet(aliases)
	t.Logf("%d announces, %d aliases across %d followed entities",
		len(names), len(aliases), net.entities())

	catalogue := st.NewCatalogue(endpoint)
	var (
		agreed   []string // both the sweep and the matcher say favourite
		missed   []string // the sweep found a favourite the matcher did not
		claimed  []string // the matcher claims a favourite the name does not contain
		viaScene []string // identified by scene title, so the sweep cannot see it
	)

	for _, name := range names {
		res, ok, err := parse.Identify(ctx, catalogue, store.Normalise, name)
		if err != nil {
			t.Fatalf("Identify(%q): %v", name, err)
		}

		swept := net.find(name)
		matched := map[string]bool{}
		if ok {
			for _, id := range append(append([]string{}, res.PerformerIDs...), res.StudioIDs...) {
				if who, isFav := net.byID[id]; isFav {
					matched[id] = true
					_ = who
				}
			}
		}

		switch {
		case len(swept) > 0 && len(matched) > 0:
			agreed = append(agreed, name)
		case len(swept) > 0:
			missed = append(missed, fmt.Sprintf("%s\n      name contains %s", name, net.describe(swept)))
		case len(matched) > 0 && res.SceneID != "":
			// The release named the scene rather than the cast, so the
			// sweep could not see the favourite and the identification did.
			// Not a wrong claim — the case the scene index exists for.
			viaScene = append(viaScene, fmt.Sprintf("%s\n      identified scene %s, cast includes %s",
				name, res.SceneID, net.describe(keys(matched))))
		case len(matched) > 0:
			claimed = append(claimed, fmt.Sprintf("%s\n      claims %s, which the name does not contain",
				name, net.describe(keys(matched))))
		}
	}

	t.Logf("favourites")
	t.Logf("  recognised by name   %d", len(agreed))
	t.Logf("  recognised by scene  %d", len(viaScene))
	t.Logf("  missed               %d", len(missed))
	t.Logf("  claimed wrongly      %d", len(claimed))

	// Every disagreement is printed: there are few enough that each one is
	// worth reading, and a rate alone would hide which rule caused it.
	for _, group := range []struct {
		label string
		rows  []string
	}{{"missed", missed}, {"claimed wrongly", claimed}, {"via scene", viaScene}} {
		for _, row := range group.rows {
			t.Logf("  %s: %s", group.label, row)
		}
	}
}

// net is a deliberately loose sweep for followed entities: any run of words in
// a release name that exactly matches one of their aliases.
//
// Looser than the matcher on purpose. It ignores where in the name the words
// sat, whether anything corroborates them, and how many people share the alias
// — so anything it finds that the matcher did not is a candidate miss worth
// reading, and the gap between the two is exactly what the rules are costing.
type net struct {
	byNormalised map[string][]store.FavouriteAlias
	byID         map[string]string
}

func newNet(aliases []store.FavouriteAlias) *net {
	n := &net{byNormalised: map[string][]store.FavouriteAlias{}, byID: map[string]string{}}
	for _, a := range aliases {
		if a.Normalised == "" {
			continue
		}
		n.byNormalised[a.Normalised] = append(n.byNormalised[a.Normalised], a)
		n.byID[a.ID] = a.Name
	}
	return n
}

func (n *net) entities() int { return len(n.byID) }

// find returns the ids of followed entities whose alias appears in the name.
func (n *net) find(name string) []string {
	words := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})

	var found []string
	for start := range words {
		for end := start + 1; end <= len(words) && end-start <= 6; end++ {
			probe := store.Normalise(strings.Join(words[start:end], ""))
			for _, a := range n.byNormalised[probe] {
				// One-word aliases are first names and match everything;
				// the sweep would report every release otherwise.
				if end-start < 2 && len(a.Normalised) < 8 {
					continue
				}
				if !contains(found, a.ID) {
					found = append(found, a.ID)
				}
			}
		}
	}
	return found
}

func (n *net) describe(ids []string) string {
	var out []string
	for _, id := range ids {
		out = append(out, n.byID[id])
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
