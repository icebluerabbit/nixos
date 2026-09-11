package parse

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
)

// Catalogue is what the parser asks about the chunks it produced. The store
// implements it; a test supplies its own, which is what keeps scoring free of a
// database.
type Catalogue interface {
	// Lookup matches every probe against every kind of entry.
	Lookup(ctx context.Context, probes []string) ([]Hit, error)
	// Scenes returns the relations of these scenes, for checking whether the
	// parts of a reading agree with one another.
	Scenes(ctx context.Context, ids []string) ([]Scene, error)
	// ScenesOn returns scenes released on a date by these studios or with
	// these performers.
	ScenesOn(ctx context.Context, date string, studioIDs, performerIDs []string) ([]Scene, error)
	// SearchScenes returns the scenes whose titles most resemble text, best
	// first, with a relevance score for each.
	SearchScenes(ctx context.Context, text string, limit int) ([]SceneMatch, error)
}

// SceneMatch is a scene the title search returned, with its relevance.
type SceneMatch struct {
	SceneID string
	Score   float64
}

// Normaliser folds text into the form the catalogue's aliases are stored in.
type Normaliser func(string) string

// Hit is one catalogue entry a probe matched.
type Hit struct {
	Probe  string
	Kind   string
	ID     string
	Source string
	// Shared is how many entities of this kind answer to this probe.
	Shared int
}

// Scene is a scene's relations.
type Scene struct {
	ID          string
	Title       string
	StudioID    string
	ParentID    string
	ReleaseDate string
	Performers  []string
}

// Kinds and sources, mirroring the store's.
const (
	KindScene     = "scene"
	KindPerformer = "performer"
	KindStudio    = "studio"

	SourceSceneTitle = "scene_title"
	SourceAlias      = "alias"
	SourceCredit     = "credit"
)

// Evidence weights. What each kind of recognition is worth before specificity
// and corroboration are applied.
const (
	// A scene title identifies the release outright.
	weightScene = 6.0
	// A performer is what a release is usually about.
	weightPerformer = 4.0
	// A studio repeats across thousands of releases, so recognising one says
	// less about this release than recognising anybody in it.
	weightStudio = 2.5
)

// Source multipliers: where the alias was written.
var sourceWeight = map[string]float64{
	SourceSceneTitle: 1.0,
	SourceAlias:      1.0,
	SourceCredit:     0.8,
}

// Corroboration bonuses, applied when the catalogue's own relations agree with
// the reading — a join, not an estimate.
const (
	bonusPerformerInScene = 3.0
	bonusStudioOfScene    = 2.0
	bonusDateOfScene      = 2.0
	// An exactly reproduced title is rare and nearly conclusive.
	bonusExactTitle = 3.0
	// maxTitleBonus caps what a title search can contribute, since bm25 is
	// unbounded and a long shared phrase would otherwise outweigh the facts.
	maxTitleBonus = 3.0
	// minSceneAgreement is how much has to line up before a candidate is
	// called this release's scene. Above one signal: a site releases several
	// scenes a day, so a date and a studio alone describe a handful of them.
	minSceneAgreement = 3.5
	// searchDepth is how many title-search candidates to consider.
	searchDepth = 10
)

// minSceneTitleWords is how many words a title needs before matching it can
// identify a scene.
//
// A scene whose whole title is "Alina" exists, and so does a performer called
// Alina, and a release that mentions the word is neither. Short titles are
// matched by accident far more often than on purpose.
const minSceneTitleWords = 3

// titlePinsWords is how long a title must be to identify a scene on its own,
// with no date, studio or named performer agreeing with it.
const titlePinsWords = 6

// minConfidence is the score below which a reading is not worth reporting.
const minConfidence = 1.0

// mononymDiscount is what a single-word performer name is worth relative to a
// full one. Not zero: a distinctive stage name that only one performer answers
// to is still evidence, and specificity already discounts the shared ones.
const mononymDiscount = 0.5

// Match is one recognised entity and what recognised it.
type Match struct {
	Kind   string
	ID     string
	Probe  string
	Source string
	Shared int
	Score  float64
	// Start and End are the words the probe covered.
	Start, End int
}

// Result is what a release name turned out to be.
type Result struct {
	// Name is the release name as announced.
	Name string
	// Body is the name with header, date and technical tokens removed.
	Body string
	// Date is the date lifted out of the name.
	Date string
	// SceneID is the scene the release was identified as, if any.
	SceneID string
	// SceneStudioID is that scene's studio. Reported, never treated as a
	// studio the release named.
	SceneStudioID string
	// PerformerIDs and StudioIDs are the entities the reading settled on.
	PerformerIDs []string
	StudioIDs    []string
	// Matches are the individual recognitions behind those ids.
	Matches []Match
	// Score is the total evidence, corroboration included.
	Score float64
	// Dropped are matches that were found but not believed — one-word names
	// nothing corroborated. Kept for the logs, since a miss caused by this
	// rule should be visible rather than silent.
	Dropped []Match
	// Corroborated reports whether the parts agreed with one another.
	Corroborated bool

	// sceneCast is the cast of the scene this was identified as, used to
	// decide which weakly-named performers to believe.
	sceneCast []string
	// scenePinned reports that the scene was fixed by the release's own date
	// or exact title, rather than by resemblance.
	scenePinned bool
}

// Names returns the text that resolved to performers, in the release's own
// spelling, for logs.
func (r Result) Names() []string {
	var out []string
	for _, m := range r.Matches {
		if m.Kind == KindPerformer && !slices.Contains(out, m.Probe) {
			out = append(out, m.Probe)
		}
	}
	return out
}

// String renders a result for a log line.
func (r Result) String() string {
	return fmt.Sprintf("score=%.1f scene=%q performers=%d studios=%d corroborated=%v",
		r.Score, r.SceneID, len(r.PerformerIDs), len(r.StudioIDs), r.Corroborated)
}

// Identify reads a release name and returns what the catalogue recognised in
// it.
//
// The steps are: partition the name, ask about every chunk whole, ask again
// about word runs inside the chunks that resolved to nothing, then look for a
// scene that the date and the recognised entities agree on. What survives is
// scored by how specific it was and how well the parts corroborate each other.
func Identify(ctx context.Context, c Catalogue, norm Normaliser, name string) (Result, bool, error) {
	reading := Read(name)
	out := Result{Name: name, Body: reading.Body, Date: reading.Date}
	if len(reading.Chunks) == 0 {
		return out, false, nil
	}

	matches, err := probeChunks(ctx, c, norm, reading.Chunks)
	if err != nil {
		return Result{}, false, err
	}
	if len(matches) == 0 && reading.Date == "" {
		return out, false, nil
	}

	out.Matches = resolveOverlaps(matches)
	for _, m := range out.Matches {
		switch m.Kind {
		case KindPerformer:
			out.PerformerIDs = appendUnique(out.PerformerIDs, []string{m.ID})
		case KindStudio:
			out.StudioIDs = appendUnique(out.StudioIDs, []string{m.ID})
		}
	}

	if err := out.identifyScene(ctx, c); err != nil {
		return Result{}, false, err
	}
	out.dropUncorroboratedMononyms()

	for _, m := range out.Matches {
		out.Score += m.Score
	}
	return out, out.Score >= minConfidence, nil
}

// dropUncorroboratedMononyms removes one-word performer matches that nothing
// backs up.
//
// A first name is a candidate, not an identification: "Kendra" and "Alina" are
// words in a title as readily as they are people, and an alias rare enough to
// survive the specificity discount can still belong to the wrong person — which
// is how a release got credited to a followed performer who was not in it.
// Believed only when the scene this release was identified as actually credits
// them.
func (r *Result) dropUncorroboratedMononyms() {
	credited := map[string]bool{}
	for _, id := range r.sceneCast {
		credited[id] = true
	}

	kept := r.Matches[:0]
	for _, m := range r.Matches {
		if m.Kind == KindPerformer && m.End-m.Start < 2 && !credited[m.ID] {
			r.Dropped = append(r.Dropped, m)
			continue
		}
		kept = append(kept, m)
	}
	r.Matches = kept

	r.PerformerIDs = nil
	for _, m := range r.Matches {
		if m.Kind == KindPerformer {
			r.PerformerIDs = appendUnique(r.PerformerIDs, []string{m.ID})
		}
	}
	// The identified scene's cast stands in for names the release did not
	// write out — but only when the scene was pinned by the release's own
	// date or its exact title. Adopting the cast of a scene that merely
	// resembled the text is how six LegalPorno releases came to claim a
	// performer none of them mention.
	if len(r.PerformerIDs) == 0 && r.scenePinned {
		r.PerformerIDs = r.sceneCast
	}
}

// probeChunks asks the catalogue about each chunk, whole first.
//
// A chunk came from a boundary the release itself wrote, so the whole of it is
// the question worth asking: "Jane Doe" is a name, and splitting it up again
// would only invite a match on half of it. Word runs are the fallback for the
// chunks that resolved to nothing — the 29% of bodies with no delimiter, where
// a name and a title run together and only the catalogue knows where one ends.
func probeChunks(ctx context.Context, c Catalogue, norm Normaliser, chunks []Chunk) ([]Match, error) {
	probes := make([]string, 0, len(chunks))
	for _, ch := range chunks {
		probes = append(probes, norm(ch.Text))
	}

	hits, err := c.Lookup(ctx, probes)
	if err != nil {
		return nil, err
	}
	byProbe := index(hits)

	var matches []Match
	var unresolved []Chunk
	for _, ch := range chunks {
		found := byProbe[norm(ch.Text)]
		if len(found) == 0 {
			unresolved = append(unresolved, ch)
			continue
		}
		matches = append(matches, score(found, ch.Start, ch.End)...)
	}
	if len(unresolved) == 0 {
		return matches, nil
	}

	runs, spans := runProbes(unresolved, norm)
	if len(runs) == 0 {
		return matches, nil
	}
	runHits, err := c.Lookup(ctx, runs)
	if err != nil {
		return nil, err
	}
	for _, h := range index(runHits) {
		for _, hit := range h {
			span := spans[hit.Probe]
			matches = append(matches, score([]Hit{hit}, span[0], span[1])...)
		}
	}
	return matches, nil
}

// maxRun caps how many words one name can span.
const maxRun = 6

// runProbes builds the consecutive word runs inside chunks that resolved to
// nothing, with the span each covers.
func runProbes(chunks []Chunk, norm Normaliser) (probes []string, spans map[string][2]int) {
	spans = map[string][2]int{}
	for _, ch := range chunks {
		if len(ch.Words) < 2 {
			continue
		}
		for start := 0; start < len(ch.Words); start++ {
			for end := start + 2; end <= len(ch.Words) && end-start <= maxRun; end++ {
				probe := norm(strings.Join(ch.Words[start:end], " "))
				if probe == "" {
					continue
				}
				if _, seen := spans[probe]; seen {
					continue
				}
				spans[probe] = [2]int{ch.Start + start, ch.Start + end}
				probes = append(probes, probe)
			}
		}
	}
	return probes, spans
}

// score turns hits into scored matches.
//
// Specificity is the whole of it: an alias forty performers answer to is a
// first name and worth almost nothing, while one that names a single entity is
// an identification. That is inverse document frequency, and it replaces the
// hand-written rule that used to special-case one-word names.
func score(hits []Hit, start, end int) []Match {
	out := make([]Match, 0, len(hits))
	for _, h := range hits {
		base, ok := map[string]float64{
			KindScene:     weightScene,
			KindPerformer: weightPerformer,
			KindStudio:    weightStudio,
		}[h.Kind]
		if !ok {
			continue
		}
		// A short title identifies nothing; see minSceneTitleWords.
		if h.Kind == KindScene && end-start < minSceneTitleWords {
			continue
		}
		source := sourceWeight[h.Source]
		if source == 0 {
			source = 1
		}
		specificity := 1 / (1 + math.Log(float64(max(h.Shared, 1))))
		// A one-word *person* is a first name; a one-word studio is simply
		// what studios are called. Discounting both punished "brazzers" and
		// "analvids" for being single words, which is most of the catalogue.
		length := 1.0
		if h.Kind == KindPerformer && end-start < 2 {
			length = mononymDiscount
		}

		out = append(out, Match{
			Kind:   h.Kind,
			ID:     h.ID,
			Probe:  h.Probe,
			Source: h.Source,
			Shared: h.Shared,
			Score:  base * source * specificity * length,
			Start:  start,
			End:    end,
		})
	}
	return out
}

// resolveOverlaps keeps the best match for any span of words.
//
// Chunks overlap by construction — the whole body and each of its parts are
// all chunks — so the same words can be recognised several times. Counting
// that as several pieces of evidence would reward nothing but the partitioning.
func resolveOverlaps(in []Match) []Match {
	slices.SortFunc(in, func(a, b Match) int {
		switch {
		case a.Score > b.Score:
			return -1
		case a.Score < b.Score:
			return 1
		default:
			return strings.Compare(a.ID, b.ID)
		}
	})

	var out []Match
	claimed := map[int]string{} // word offset -> kind that claimed it
	for _, m := range in {
		conflict := false
		for i := m.Start; i < m.End; i++ {
			if kind, taken := claimed[i]; taken && kind == m.Kind {
				conflict = true
				break
			}
		}
		if conflict {
			continue
		}
		for i := m.Start; i < m.End; i++ {
			claimed[i] = m.Kind
		}
		out = append(out, m)
	}
	return out
}

// identifyScene settles which scene the release is, and how well the parts of
// the reading agree.
//
// Candidates come from three directions — a title that matched exactly, a
// title search, and the date — and none of them is trusted alone. A site
// releases several scenes a day, so "this studio, that date" describes a
// handful of scenes and picking one of them at random was wrong two times in
// five. What settles it is agreement: the scene whose cast, studio, date and
// title line up best with the rest of the reading.
func (r *Result) identifyScene(ctx context.Context, c Catalogue) error {
	// A title pins a scene only when it is long enough to be that scene's
	// title rather than a phrase.
	//
	// Counting how many scenes share it would be the better test, but the
	// count is taken over a catalogue holding only followed performers'
	// scenes — "Girls Night Out" appears once here and many times on
	// StashDB, so local uniqueness says nothing. Length does: a thirteen-word
	// title identifies a scene, three words identify a genre.
	titled := map[string]bool{}
	var exact []string
	for _, m := range r.Matches {
		if m.Kind != KindScene {
			continue
		}
		exact = append(exact, m.ID)
		if m.End-m.Start >= titlePinsWords {
			titled[m.ID] = true
		}
	}

	candidates, err := c.Scenes(ctx, exact)
	if err != nil {
		return err
	}
	dated, err := c.ScenesOn(ctx, r.Date, r.StudioIDs, r.PerformerIDs)
	if err != nil {
		return err
	}
	candidates = append(candidates, dated...)

	// Title search over the body, which is the part of the name that is not
	// the site, the date or the encoding.
	relevance := map[string]float64{}
	found, err := c.SearchScenes(ctx, r.Body, searchDepth)
	if err != nil {
		return err
	}
	var searched []string
	for _, m := range found {
		relevance[m.SceneID] = m.Score
		searched = append(searched, m.SceneID)
	}
	facts, err := c.Scenes(ctx, searched)
	if err != nil {
		return err
	}
	candidates = append(candidates, facts...)
	if len(candidates) == 0 {
		return nil
	}

	// Where each scene's title matched, so a performer found in the same
	// words cannot be counted as agreeing with it.
	titleSpan := map[string][2]int{}
	for _, m := range r.Matches {
		if m.Kind == KindScene {
			titleSpan[m.ID] = [2]int{m.Start, m.End}
		}
	}

	best := ""
	bestScore := 0.0
	for _, s := range candidates {
		// A date the release states and the scene contradicts is decisive:
		// whatever else lined up, this is a different scene.
		if r.Date != "" && s.ReleaseDate != "" && !sameDate(r.Date, s.ReleaseDate) {
			continue
		}
		if !identifies(r, s, titled, titleSpan) {
			continue
		}

		bonus := 0.0
		for _, m := range r.Matches {
			if m.Kind != KindPerformer || !slices.Contains(s.Performers, m.ID) {
				continue
			}
			// Corroboration has to come from somewhere else in the name.
			// One word matching both a performer and a scene title is a
			// single fact agreeing with itself.
			if span, ok := titleSpan[s.ID]; ok && overlaps(span, [2]int{m.Start, m.End}) {
				continue
			}
			bonus += bonusPerformerInScene
		}
		if slices.Contains(r.StudioIDs, s.StudioID) || (s.ParentID != "" && slices.Contains(r.StudioIDs, s.ParentID)) {
			bonus += bonusStudioOfScene
		}
		if r.Date != "" && s.ReleaseDate != "" && sameDate(r.Date, s.ReleaseDate) {
			bonus += bonusDateOfScene
		}
		if titled[s.ID] {
			bonus += bonusExactTitle
		}
		if rel, ok := relevance[s.ID]; ok {
			bonus += math.Min(rel, maxTitleBonus)
		}
		if bonus > bestScore {
			best, bestScore = s.ID, bonus
		}
	}
	// One signal alone does not identify a scene: a studio releases many on a
	// date and a title search always returns its best guess.
	if best == "" || bestScore < minSceneAgreement {
		return nil
	}

	r.SceneID = best
	r.Score += bestScore
	r.Corroborated = bestScore >= bonusPerformerInScene
	for _, s := range candidates {
		if s.ID != best {
			continue
		}
		r.sceneCast = s.Performers
		r.scenePinned = titled[s.ID] ||
			(r.Date != "" && s.ReleaseDate != "" && sameDate(r.Date, s.ReleaseDate))
		// The scene's studio is recorded but never adopted as the release's.
		// A release that says "victorcaserta.com" has not said "Fit18",
		// however plausibly the scene behind it belongs to one — and a rule
		// that grabs on studios must act on what the release states.
		r.SceneStudioID = s.StudioID
	}
	return nil
}

// identifies reports whether a candidate scene is tied to this release by
// something better than resemblance.
//
// The catalogue is not a neutral index: every scene in it belongs to a followed
// performer or studio, so a title search always has a favourite's scene to
// offer and "the words looked similar" is the weakest possible reason to
// believe one. A claim needs a fact the release itself states — the title
// reproduced, the date agreed, or somebody named in the text actually being in
// the scene.
func identifies(r *Result, s Scene, titled map[string]bool, titleSpan map[string][2]int) bool {
	if titled[s.ID] {
		return true
	}
	if r.Date != "" && s.ReleaseDate != "" && sameDate(r.Date, s.ReleaseDate) {
		return true
	}
	for _, m := range r.Matches {
		if m.Kind != KindPerformer || !slices.Contains(s.Performers, m.ID) {
			continue
		}
		// A name that is also where the title matched is not independent.
		if span, ok := titleSpan[s.ID]; ok && overlaps(span, [2]int{m.Start, m.End}) {
			continue
		}
		// A first name backed by nothing else is not a fact either.
		if m.End-m.Start < 2 {
			continue
		}
		return true
	}
	return false
}

// overlaps reports whether two word spans share any word.
func overlaps(a, b [2]int) bool {
	return a[0] < b[1] && b[0] < a[1]
}

// index groups hits by the probe that found them.
func index(hits []Hit) map[string][]Hit {
	out := map[string][]Hit{}
	for _, h := range hits {
		out[h.Probe] = append(out[h.Probe], h)
	}
	return out
}

// appendUnique adds the ids not already present, preserving order.
func appendUnique(to, ids []string) []string {
	for _, id := range ids {
		if !slices.Contains(to, id) {
			to = append(to, id)
		}
	}
	return to
}
