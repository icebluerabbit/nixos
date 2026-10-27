package config

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	flag "github.com/spf13/pflag"

	"homelab/mousetrap/internal/types"
)

// everyFlag sets every flag mousetrap declares to a value distinguishable from
// its default. It is the input to both tests below: one asserts where each
// value lands, the other asserts that no flag is missing from this list.
//
// This is the load-bearing test of the whole configuration layer. Flags are
// declared, not bound — lib/config matches a flag to a field by turning the
// flag's name into a key and looking up the json tag — so a typo on either
// side does not fail to compile, it silently leaves the field at its default.
// For an authority that spends wedges and bonus points on a real account, a
// silently ignored --shop-reserved is the worst kind of bug.
var everyFlag = []string{
	"--listen", "127.0.0.1",
	"--port", "9000",
	"--state-dir", "/tmp/mousetrap",

	"--mam-api-base", "http://mam.test",
	"--mam-seed-path", "/run/secrets/mam",
	"--mam-media-type-name", "2=Ebook",
	"--mam-media-type-name", "13=Audiobook",

	"--client-url", "http://qbt.test:8080",
	"--client-category", "Ebook=books",
	"--client-category", "Audiobook=audiobooks",
	"--client-paused=true",

	"--webhook-url", "http://hook.test",
	"--webhook-template", "{{ .Type }}",
	"--webhook-header", "X-Token=abc",
	"--webhook-event", "grab",
	"--webhook-event", "spend",

	"--torznab-apikey", "key",
	"--torznab-base-url", "http://mousetrap.test:8080",
	"--torznab-link-ttl", "90m",

	"--accept-media-type", "Ebook",
	"--accept-category-allow", "Ebooks - Fantasy",
	"--accept-category-deny", "Ebooks - Romance",
	"--accept-filetype-allow", "epub",
	"--accept-filetype-deny", "pdf",
	"--accept-fiction-type", "Fiction",
	"--accept-language", "english",
	"--accept-size-min", "1024",
	"--accept-size-max", "2048",
	"--accept-freeleech-only=true",
	"--accept-vip-only=true",
	"--accept-exclude-vip=true",
	"--accept-name-include", "^good",
	"--accept-name-exclude", "bad$",

	"--grab-unsat-margin", "7",
	"--grab-inflight-window", "45m",
	"--grab-buffer-min", "4096",
	"--grab-dry-run=false",

	"--wedge-enable=true",
	"--wedge-reserve", "3",
	"--wedge-proactive-media-type", "Audiobook",
	"--wedge-proactive-min-size", "8192",
	"--wedge-require=true",
	"--wedge-rescue=true",
	"--wedge-rescue-min-size", "16384",

	"--shop-dry-run=false",
	"--shop-reserved", "100",
	"--shop-vip-enable=true",
	"--shop-vip-cost", "4000",
	"--shop-vip-reserve-multiple", "3",
	"--shop-vip-buyable-class", "User",
	"--shop-upload-enable=true",
	"--shop-upload-chunk-gib", "25",
	"--shop-upload-points-per-gib", "250",
}

// wantEveryFlag is where each of the values above must land.
var wantEveryFlag = types.Config{
	Listen:   "127.0.0.1",
	Port:     "9000",
	StateDir: "/tmp/mousetrap",
	MAM: types.MAMConfig{
		APIBase:        "http://mam.test",
		SeedPath:       "/run/secrets/mam",
		MediaTypeNames: map[string]string{"2": "Ebook", "13": "Audiobook"},
	},
	Client: types.ClientConfig{
		URL:        "http://qbt.test:8080",
		Categories: map[string]string{"Ebook": "books", "Audiobook": "audiobooks"},
		AddPaused:  true,
	},
	Webhook: types.WebhookConfig{
		URL:      "http://hook.test",
		Template: "{{ .Type }}",
		Headers:  map[string]string{"X-Token": "abc"},
		Events:   []string{"grab", "spend"},
	},
	Torznab: types.TorznabConfig{
		APIKey:  "key",
		BaseURL: "http://mousetrap.test:8080",
		LinkTTL: 90 * time.Minute,
	},
	Accept: types.AcceptConfig{
		MediaTypes:       []string{"Ebook"},
		CategoriesAllow:  []string{"Ebooks - Fantasy"},
		CategoriesDeny:   []string{"Ebooks - Romance"},
		FiletypesAllow:   []string{"epub"},
		FiletypesDeny:    []string{"pdf"},
		FictionTypes:     []string{"Fiction"},
		LanguagesAllow:   []string{"english"},
		SizeMinBytes:     1024,
		SizeMaxBytes:     2048,
		FreeleechOnly:    true,
		VIPOnly:          true,
		ExcludeVIP:       true,
		NameIncludeRegex: "^good",
		NameExcludeRegex: "bad$",
	},
	Grab: types.GrabConfig{
		UnsatMargin:    7,
		InFlightWindow: 45 * time.Minute,
		BufferMinBytes: 4096,
		DryRun:         false,
	},
	Wedge: types.WedgeConfig{
		Enable:                true,
		Reserve:               3,
		ProactiveMediaTypes:   []string{"Audiobook"},
		ProactiveMinSizeBytes: 8192,
		RequireWedge:          true,
		RescueEnable:          true,
		RescueMinSizeBytes:    16384,
	},
	Shop: types.ShopConfig{
		DryRun:             false,
		Reserved:           100,
		VIPEnable:          true,
		VIPCost:            4000,
		VIPReserveMultiple: 3,
		VIPBuyableClasses:  []string{"User"},
		UploadEnable:       true,
		UploadChunkGiB:     25,
		UploadPointsPerGiB: 250,
	},
}

func TestEveryFlagReachesItsField(t *testing.T) {
	t.Parallel()

	got, err := Parse(everyFlag)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// An unset repeatable flag resolves to an empty slice rather than a nil
	// one, which every reader of these fields treats identically — the rule
	// is off either way. Every value asserted here is non-empty, so this
	// cannot hide a flag that failed to reach its field.
	if diff := cmp.Diff(wantEveryFlag, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("config mismatch (-want +got):\n%s", diff)
	}
}

// A flag added without a line in everyFlag would leave the mapping test above
// passing while proving nothing about the new flag. This fails instead.
func TestEveryFlagIsExercised(t *testing.T) {
	t.Parallel()

	given := map[string]bool{}
	for _, arg := range everyFlag {
		if len(arg) < 3 || arg[:2] != "--" {
			continue
		}
		name := arg[2:]
		if i := indexByte(name, '='); i >= 0 {
			name = name[:i]
		}
		given[name] = true
	}

	fs := flag.NewFlagSet("mousetrap", flag.ContinueOnError)
	Flags(fs)
	fs.VisitAll(func(f *flag.Flag) {
		if !given[f.Name] {
			t.Errorf("flag --%s is declared but never exercised by everyFlag", f.Name)
		}
	})
}

// Defaults and the flag declarations state the same fact twice, so they are
// asserted equal: a run with no flags must produce exactly Defaults.
func TestParseWithoutFlagsIsDefaults(t *testing.T) {
	t.Parallel()

	got, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if diff := cmp.Diff(Defaults(), got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("defaults mismatch (-want +got):\n%s", diff)
	}
}

func indexByte(s string, b byte) int {
	for i := range len(s) {
		if s[i] == b {
			return i
		}
	}
	return -1
}
