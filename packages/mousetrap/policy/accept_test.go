package policy

import (
	"testing"

	"mousetrap/types"
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
		Kinds:            []string{"ebook", "audiobook"},
		CategoriesAllow:  []string{"Ebooks - Fantasy"},
		CategoriesDeny:   []string{"Ebooks - Romance"},
		FiletypesAllow:   []string{"epub", "m4b"},
		LanguagesAllow:   []string{"english"},
		SizeMinBytes:     100,
		SizeMaxBytes:     1000,
		NameExcludeRegex: "sample",
	}
	e := mustEngine(t, cfg)

	good := types.Release{
		Kind:     types.KindEbook,
		Category: "Ebooks - Fantasy",
		Filetype: "epub",
		Languages: []string{"English"},
		Size:     500,
		Name:     "A Great Book",
	}

	cases := []struct {
		name   string
		mutate func(*types.Release)
		wantD  types.AcceptDecision
		wantR  types.AcceptReason
	}{
		{"accept", nil, types.Accept, types.AcceptOK},
		{"kind", func(r *types.Release) { r.Kind = types.KindOther }, types.Reject, types.AcceptKind},
		{"category_deny", func(r *types.Release) { r.Category = "Ebooks - Romance" }, types.Reject, types.AcceptCategoryDeny},
		{"category_allow", func(r *types.Release) { r.Category = "Ebooks - Horror" }, types.Reject, types.AcceptCategoryAllow},
		{"filetype_allow", func(r *types.Release) { r.Filetype = "pdf" }, types.Reject, types.AcceptFiletypeAllow},
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
		CategoriesAllow: []string{"Ebooks - Fantasy"},
		CategoriesDeny:  []string{"Ebooks - Fantasy"},
	})
	_, r := e.Accept(types.Release{Category: "Ebooks - Fantasy"})
	if r != types.AcceptCategoryDeny {
		t.Errorf("reason = %s, want category_deny", r)
	}
}

func TestAcceptFiletypeDenyCaseInsensitive(t *testing.T) {
	e := mustEngine(t, types.AcceptConfig{FiletypesDeny: []string{"pdf"}})
	if _, r := e.Accept(types.Release{Filetype: "PDF"}); r != types.AcceptFiletypeDeny {
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
