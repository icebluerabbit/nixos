package types_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"homelab/stashtrap/internal/types"
)

// autobrr sends categories positionally — torznab.go appends
// [Name, strconv.Itoa(ID)] per category — so the de-interleave is by index,
// not by content. A name that looks like a number must still be read as a
// name.
func TestParseCategories(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		raw  []string
		want []types.Category
	}{
		"bitporn's usual shape": {
			raw: []string{"XXX", "6000", "Fansite", "100020"},
			want: []types.Category{
				{ID: 6000, Name: "XXX"},
				{ID: 100020, Name: "Fansite"},
			},
		},
		"single pair": {
			raw:  []string{"XXX", "6000"},
			want: []types.Category{{ID: 6000, Name: "XXX"}},
		},
		"three pairs": {
			raw: []string{"a", "1", "b", "2", "c", "3"},
			want: []types.Category{
				{ID: 1, Name: "a"}, {ID: 2, Name: "b"}, {ID: 3, Name: "c"},
			},
		},
		// Prowlarr's BitPorn definition leaves an unbalanced double quote on
		// every category description (`desc: Fansite"`), and the name is a
		// metrics label and a UI string — carrying the defect through would
		// make `Fansite"` what everything downstream calls it.
		"strips a trailing quote": {
			raw:  []string{`Fansite"`, "100020"},
			want: []types.Category{{ID: 100020, Name: "Fansite"}},
		},
		"strips quotes at both ends and surrounding space": {
			raw:  []string{` "Compilation" `, "100015"},
			want: []types.Category{{ID: 100015, Name: "Compilation"}},
		},
		// A quote inside the name is content, not delimiter.
		"keeps an interior quote": {
			raw:  []string{`Studio"X`, "100012"},
			want: []types.Category{{ID: 100012, Name: `Studio"X`}},
		},
		"numeric-looking name": {
			raw:  []string{"2160", "100020"},
			want: []types.Category{{ID: 100020, Name: "2160"}},
		},
		"empty":   {raw: nil, want: []types.Category{}},
		"no pair": {raw: []string{"XXX"}, want: []types.Category{}},
		// Position is the only thing identifying which half is which, so a
		// slice that cannot be realigned is discarded, not guessed at.
		"unparseable id is dropped": {
			raw:  []string{"XXX", "notanumber", "Fansite", "100020"},
			want: []types.Category{{ID: 100020, Name: "Fansite"}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if diff := cmp.Diff(tc.want, types.ParseCategories(tc.raw)); diff != "" {
				t.Errorf("ParseCategories() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
