package reconcile

import (
	"testing"
	"time"

	"homelab/clients/autobrr"
	"homelab/managarr/internal/model"
)

// TestFilterOutcome maps autobrr's filter verdict.
//
// The default matters more than the happy path: autobrr has several rejection
// statuses (FILTER_REJECTED, FILTER_PENDING and, historically, an empty string
// on records written before the column existed) and every one of them means the
// release was not taken. Defaulting to accepted would turn a hundred thousand
// refusals into approvals.
func TestFilterOutcome(t *testing.T) {
	cases := map[string]string{
		"FILTER_APPROVED": model.OutcomeAccepted,
		"FILTER_REJECTED": model.OutcomeRejected,
		"FILTER_PENDING":  model.OutcomeRejected,
		"":                model.OutcomeRejected,
		"nonsense":        model.OutcomeRejected,
	}
	for status, want := range cases {
		if got := filterOutcome(status); got != want {
			t.Errorf("filterOutcome(%q) = %q, want %q", status, got, want)
		}
	}
}

// TestPushOutcome keeps a broken integration distinguishable from a policy.
//
// PUSH_ERROR is errored, not rejected: the target never judged the release, the
// push itself failed. Folding it into rejected would make a dead *arr read as a
// filtering decision, and "why did nothing get grabbed last night" would have
// the wrong answer.
func TestPushOutcome(t *testing.T) {
	cases := map[string]string{
		"PUSH_APPROVED": model.OutcomeAccepted,
		"PUSH_REJECTED": model.OutcomeRejected,
		"PUSH_ERROR":    model.OutcomeErrored,
		"":              model.OutcomeErrored,
		"PUSH_PENDING":  model.OutcomeErrored,
	}
	for status, want := range cases {
		if got := pushOutcome(status); got != want {
			t.Errorf("pushOutcome(%q) = %q, want %q", status, got, want)
		}
	}
}

// TestOutcomesAreTheDeclaredVocabulary stops a typo becoming a silent third
// value.
//
// Nothing constrains these columns at the database level, so a misspelled
// "acccepted" would store cleanly and simply never match a query again.
func TestOutcomesAreTheDeclaredVocabulary(t *testing.T) {
	valid := map[string]bool{
		model.OutcomeAccepted: true,
		model.OutcomeRejected: true,
		model.OutcomeErrored:  true,
	}
	for _, s := range []string{"FILTER_APPROVED", "FILTER_REJECTED", "", "x"} {
		if !valid[filterOutcome(s)] {
			t.Errorf("filterOutcome(%q) produced %q", s, filterOutcome(s))
		}
	}
	for _, s := range []string{"PUSH_APPROVED", "PUSH_REJECTED", "PUSH_ERROR", "", "x"} {
		if !valid[pushOutcome(s)] {
			t.Errorf("pushOutcome(%q) produced %q", s, pushOutcome(s))
		}
	}
}

// TestReleaseRowsCarryTheParsedFields covers the columns that exist only here.
//
// autobrr is the only service in this stack that parses a release NAME into its
// parts, and it does it on everything it sees — including the four fifths it
// rejects, which no *arr ever looked at. Dropping these on the floor is what
// left the hinge table holding nothing but a title.
func TestReleaseRowsCarryTheParsedFields(t *testing.T) {
	got := releaseRow(autobrr.Release{
		Name:       "Some.Show.S01E02.2026.1080p.WEB-DL.DV.HDR10.H265-GRP",
		Resolution: "1080p",
		Source:     "WEB-DL",
		// Both are arrays upstream: one file routinely carries several.
		Codec:     []string{"H.265", "x265"},
		HDR:       []string{"DV", "HDR10"},
		Container: "mkv",
		Group:     "GRP",
		Edition:   []string{"Director's Cut"},
		Proper:    false,
		Repack:    true,
		Year:      2026,
		Season:    1,
		Episode:   2,
		Size:      123456789,
	})

	if got.Title == "" {
		t.Fatal("title is the natural key and must be set")
	}
	if deref(got.Resolution) != "1080p" || deref(got.Source) != "WEB-DL" {
		t.Errorf("resolution/source = %q / %q", deref(got.Resolution), deref(got.Source))
	}
	if deref(got.Codec) != "H.265, x265" {
		t.Errorf("codec = %q; the array must be flattened, not truncated", deref(got.Codec))
	}
	if deref(got.HDR) != "DV, HDR10" {
		t.Errorf("hdr = %q", deref(got.HDR))
	}
	if deref(got.ReleaseGroup) != "GRP" || deref(got.Edition) != "Director's Cut" {
		t.Errorf("group/edition = %q / %q", deref(got.ReleaseGroup), deref(got.Edition))
	}
	// Proper and Repack are the difference between two files of the same
	// quality, so false must be stored as false rather than collapsed to NULL.
	if got.Proper == nil || *got.Proper {
		t.Errorf("proper = %v, want a non-nil false", got.Proper)
	}
	if got.Repack == nil || !*got.Repack {
		t.Errorf("repack = %v", got.Repack)
	}
	if got.Year == nil || *got.Year != 2026 {
		t.Errorf("year = %v", got.Year)
	}
	if got.SizeBytes == nil || *got.SizeBytes != 123456789 {
		t.Errorf("size = %v", got.SizeBytes)
	}
}

// TestReleaseRowUnparsedIsAllNull is the case COALESCE exists for.
//
// A padded name, or an announce the indexer definition did not fully match,
// yields a record with nothing parsed. Every column must come out NULL so the
// upsert leaves whatever a cleaner announce of the same release established.
func TestReleaseRowUnparsedIsAllNull(t *testing.T) {
	got := releaseRow(autobrr.Release{Name: "some release nobody can parse"})

	for name, v := range map[string]*string{
		"resolution": got.Resolution, "source": got.Source, "codec": got.Codec,
		"hdr": got.HDR, "container": got.Container,
		"release_group": got.ReleaseGroup, "edition": got.Edition,
	} {
		if v != nil {
			t.Errorf("%s = %q, want NULL", name, *v)
		}
	}
	for name, v := range map[string]*int{
		"year": got.Year, "season": got.Season, "episode": got.Episode,
	} {
		if v != nil {
			t.Errorf("%s = %d, want NULL", name, *v)
		}
	}
	if got.SizeBytes != nil {
		t.Errorf("size = %d, want NULL", *got.SizeBytes)
	}
	// content_id stays NULL here on purpose: it is resolved from grabs by the
	// resolve pass, and writing it from this side would undo that every pass.
	if got.ContentID != nil {
		t.Error("content_id must not be written by the autobrr pass")
	}
}

// TestReleaseRowsSkipNamelessRecords keeps the natural key satisfiable.
func TestReleaseRowsSkipNamelessRecords(t *testing.T) {
	rows := releaseRows([]autobrr.Release{
		{Name: "Some.Release-GRP"},
		{Name: ""},
		{Name: "Other.Release-GRP"},
	})
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	for _, r := range rows {
		if r.Title == "" {
			t.Error("a nameless release was kept")
		}
	}
}

// TestReleaseRowsDedupesByTitle is the 21000 failure.
//
// One INSERT ... ON CONFLICT DO UPDATE cannot touch the same row twice —
// Postgres raises "command cannot affect row a second time" — and a page
// routinely holds one title more than once, because the same release announced
// on two trackers is two autobrr records with one name. That is the
// cross-seeding case, so it is the normal case.
func TestReleaseRowsDedupesByTitle(t *testing.T) {
	const title = "Lion.Fist.2026.1080p.BluRay.H264-RiSEHD"

	rows := releaseRows([]autobrr.Release{
		{Name: title, Resolution: "1080p", Source: "BluRay"},
		{Name: title, Resolution: "1080p", Source: "BluRay"},
		{Name: "Other.Release-GRP"},
	})
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.Title] {
			t.Fatalf("title %q appears twice; the statement would fail", r.Title)
		}
		seen[r.Title] = true
	}
	if rows[0].Title != title {
		t.Errorf("first row = %q, want the first-seen title", rows[0].Title)
	}
}

// TestReleaseRowsMergesDuplicates keeps the better parse.
//
// The duplicates are not identical: autobrr parses each announce separately and
// one tracker's padded name yields fewer fields than another's. Taking whichever
// came last would let the worse parse win — the same thing UpsertCoalesce
// prevents across passes, applied inside the batch where SQL cannot reach.
func TestReleaseRowsMergesDuplicates(t *testing.T) {
	const title = "Some.Film.2026.1080p.BluRay-GRP"

	rows := releaseRows([]autobrr.Release{
		{Name: title, Resolution: "1080p"},
		{Name: title, Source: "BluRay", Codec: []string{"x265"},
			Group: "GRP", Year: 2026, Repack: true, Size: 500},
	})
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	got := rows[0]

	if deref(got.Resolution) != "1080p" {
		t.Errorf("resolution = %q; the first row's value was lost", deref(got.Resolution))
	}
	if deref(got.Source) != "BluRay" || deref(got.Codec) != "x265" {
		t.Errorf("source=%q codec=%q; the second row did not fill the gaps",
			deref(got.Source), deref(got.Codec))
	}
	if got.Year == nil || *got.Year != 2026 {
		t.Errorf("year = %v", got.Year)
	}
	if got.SizeBytes == nil || *got.SizeBytes != 500 {
		t.Errorf("size = %v", got.SizeBytes)
	}
	if got.Repack == nil || !*got.Repack {
		t.Errorf("repack = %v, want true", got.Repack)
	}
}

// TestDedupeDecisionsKeepsTheLatest covers the other batch that can collide.
//
// A filter with two actions of the same name produces two pushes keyed
// identically. A repeated action is a retry, so the later verdict is what
// actually happened.
func TestDedupeDecisionsKeepsTheLatest(t *testing.T) {
	id := model.NewID()
	early := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	late := time.Date(2026, 9, 13, 12, 5, 0, 0, time.UTC)

	got := dedupeDecisions([]model.Decision{
		{OfferID: id, Decider: "autobrr", Stage: model.StageFilter,
			Outcome: model.OutcomeAccepted, DecidedAt: &early},
		{OfferID: id, Decider: "sonarr", Stage: model.StagePush,
			Outcome: model.OutcomeErrored, DecidedAt: &early},
		{OfferID: id, Decider: "sonarr", Stage: model.StagePush,
			Outcome: model.OutcomeAccepted, DecidedAt: &late},
	})
	if len(got) != 2 {
		t.Fatalf("got %d decisions, want 2: %+v", len(got), got)
	}
	var push model.Decision
	for _, d := range got {
		if d.Stage == model.StagePush {
			push = d
		}
	}
	if push.Outcome != model.OutcomeAccepted {
		t.Errorf("push outcome = %q; the retry should have won", push.Outcome)
	}
	if got[0].Stage != model.StageFilter {
		t.Errorf("first = %+v, want the filter decision kept in order", got[0])
	}
}

// TestDedupeDecisionsHandlesMissingTimestamps keeps arrival order.
func TestDedupeDecisionsHandlesMissingTimestamps(t *testing.T) {
	id := model.NewID()
	at := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

	got := dedupeDecisions([]model.Decision{
		{OfferID: id, Decider: "x", Stage: model.StagePush, Outcome: "a"},
		{OfferID: id, Decider: "x", Stage: model.StagePush, Outcome: "b", DecidedAt: &at},
	})
	if len(got) != 1 || got[0].Outcome != "b" {
		t.Errorf("got %+v, want the timestamped record", got)
	}

	got = dedupeDecisions([]model.Decision{
		{OfferID: id, Decider: "x", Stage: model.StagePush, Outcome: "a", DecidedAt: &at},
		{OfferID: id, Decider: "x", Stage: model.StagePush, Outcome: "b"},
	})
	if len(got) != 1 || got[0].Outcome != "a" {
		t.Errorf("got %+v, want the timestamped record retained", got)
	}
}
