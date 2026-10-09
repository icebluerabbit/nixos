package policy

import (
	"testing"

	"homelab/mousetrap/internal/types"
)

func mustEngine(t *testing.T, a types.AcceptConfig) *Engine {
	t.Helper()
	e, err := New(a, types.GrabConfig{}, types.WedgeConfig{}, types.ShopConfig{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e
}

// base config exercises most rules; each case mutates a good release to trip one.
func TestAcceptRules(t *testing.T) {
	cfg := types.AcceptConfig{
		MediaTypes:       []string{"Ebook", "Audiobook"},
		CategoriesAllow:  []string{"Fantasy"},
		CategoriesDeny:   []string{"Romance"},
		FiletypesAllow:   []string{"epub", "m4b"},
		FictionTypes:     []string{"Fiction"},
		LanguagesAllow:   []string{"english"},
		SizeMinBytes:     100,
		SizeMaxBytes:     1000,
		NameExcludeRegex: "sample",
	}
	e := mustEngine(t, cfg)

	good := types.Release{
		MediaType:  "Ebook",
		Categories: []string{"Fantasy"},
		Filetypes:  []string{"epub"},
		Fiction:    "Fiction",
		Languages:  []string{"English"},
		Size:       500,
		Name:       "A Great Book",
	}

	cases := []struct {
		name   string
		mutate func(*types.Release)
		wantD  types.AcceptDecision
		wantR  types.AcceptReason
	}{
		{"accept", nil, types.Accept, types.AcceptOK},
		{"media_type", func(r *types.Release) { r.MediaType = "Musicology" }, types.Reject, types.AcceptMediaType},
		{"category_deny", func(r *types.Release) { r.Categories = []string{"Romance"} }, types.Reject, types.AcceptCategoryDeny},
		// a release carrying BOTH an allowed and a denied category is a deny: deny is any-of too
		{"category_deny_mixed", func(r *types.Release) { r.Categories = []string{"Fantasy", "Romance"} }, types.Reject, types.AcceptCategoryDeny},
		{"category_allow", func(r *types.Release) { r.Categories = []string{"Horror"} }, types.Reject, types.AcceptCategoryAllow},
		{"category_allow_any", func(r *types.Release) { r.Categories = []string{"Horror", "Fantasy"} }, types.Accept, types.AcceptOK},
		{"category_empty", func(r *types.Release) { r.Categories = nil }, types.Reject, types.AcceptCategoryAllow},
		{"filetype_allow", func(r *types.Release) { r.Filetypes = []string{"pdf"} }, types.Reject, types.AcceptFiletypeAllow},
		// the multi-format case that exact matching used to reject: "azw3, epub"
		{"filetype_allow_any", func(r *types.Release) { r.Filetypes = []string{"azw3", "epub"} }, types.Accept, types.AcceptOK},
		{"media_type_fold", func(r *types.Release) { r.MediaType = "EBOOK" }, types.Accept, types.AcceptOK},
		{"fiction", func(r *types.Release) { r.Fiction = "Non-Fiction" }, types.Reject, types.AcceptFiction},
		// search results carry no flag; rejecting them would break the torznab path
		{"fiction_empty", func(r *types.Release) { r.Fiction = "" }, types.Accept, types.AcceptOK},
		{"language", func(r *types.Release) { r.Languages = []string{"German"} }, types.Reject, types.AcceptLanguage},
		{"language_empty", func(r *types.Release) { r.Languages = nil }, types.Reject, types.AcceptLanguage},
		{"size_min", func(r *types.Release) { r.Size = 50 }, types.Reject, types.AcceptSizeMin},
		{"size_max", func(r *types.Release) { r.Size = 2000 }, types.Reject, types.AcceptSizeMax},
		{"name_exclude", func(r *types.Release) { r.Name = "A Great Book (sample)" }, types.Reject, types.AcceptNameExclude},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rel := good
			if tc.mutate != nil {
				tc.mutate(&rel)
			}
			d, r := e.Accept(rel)
			if d != tc.wantD || r != tc.wantR {
				t.Errorf("Accept = %s/%s, want %s/%s", d, r, tc.wantD, tc.wantR)
			}
		})
	}
}

// deny is checked before allow: a category in both is rejected as a deny.
func TestAcceptDenyBeatsAllow(t *testing.T) {
	e := mustEngine(t, types.AcceptConfig{
		CategoriesAllow: []string{"Fantasy"},
		CategoriesDeny:  []string{"Fantasy"},
	})
	_, r := e.Accept(types.Release{Categories: []string{"Fantasy"}})
	if r != types.AcceptCategoryDeny {
		t.Errorf("reason = %s, want category_deny", r)
	}
}

func TestAcceptFiletypeDenyCaseInsensitive(t *testing.T) {
	e := mustEngine(t, types.AcceptConfig{FiletypesDeny: []string{"pdf"}})
	if _, r := e.Accept(types.Release{Filetypes: []string{"PDF"}}); r != types.AcceptFiletypeDeny {
		t.Errorf("reason = %s, want filetype_deny", r)
	}
}

func TestAcceptFreeleechAndVIP(t *testing.T) {
	fl := mustEngine(t, types.AcceptConfig{FreeleechOnly: true})
	if _, r := fl.Accept(types.Release{Freeleech: false}); r != types.AcceptFreeleech {
		t.Errorf("freeleech reason = %s", r)
	}
	if d, _ := fl.Accept(types.Release{Freeleech: true}); d != types.Accept {
		t.Error("freeleech release should accept")
	}

	only := mustEngine(t, types.AcceptConfig{VIPOnly: true})
	if _, r := only.Accept(types.Release{VIP: false}); r != types.AcceptVIPOnly {
		t.Errorf("vip_only reason = %s", r)
	}
	excl := mustEngine(t, types.AcceptConfig{ExcludeVIP: true})
	if _, r := excl.Accept(types.Release{VIP: true}); r != types.AcceptVIPExclude {
		t.Errorf("vip_exclude reason = %s", r)
	}
}

func TestAcceptNameInclude(t *testing.T) {
	e := mustEngine(t, types.AcceptConfig{NameIncludeRegex: "brandon sanderson"})
	if _, r := e.Accept(types.Release{Name: "Some Other Author"}); r != types.AcceptNameInclude {
		t.Errorf("reason = %s, want name_include", r)
	}
	// case-insensitive match passes
	if d, _ := e.Accept(types.Release{Name: "Mistborn - Brandon SANDERSON"}); d != types.Accept {
		t.Error("case-insensitive include should accept")
	}
}

func TestAcceptEmptyConfigAcceptsAll(t *testing.T) {
	e := mustEngine(t, types.AcceptConfig{})
	if d, r := e.Accept(types.Release{}); d != types.Accept || r != types.AcceptOK {
		t.Errorf("empty config = %s/%s, want accept/ok", d, r)
	}
}

func TestNewBadRegex(t *testing.T) {
	if _, err := New(types.AcceptConfig{NameIncludeRegex: "("}, types.GrabConfig{}, types.WedgeConfig{}, types.ShopConfig{}); err == nil {
		t.Error("expected error for invalid include regex")
	}
	if _, err := New(types.AcceptConfig{NameExcludeRegex: "["}, types.GrabConfig{}, types.WedgeConfig{}, types.ShopConfig{}); err == nil {
		t.Error("expected error for invalid exclude regex")
	}
}
