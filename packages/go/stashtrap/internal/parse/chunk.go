package parse

import (
	"strings"
	"unicode"
)

// Position records where in the name a chunk came from. It is data, not a
// claim: a header holds a studio on most releases and a performer on plenty of
// others ("[Manyvids: Sweetie Fox]"), so nothing downstream may assume a chunk
// is one kind of thing because of where it sat.
type Position string

// Where a chunk came from.
const (
	PositionHeader Position = "header"
	PositionBody   Position = "body"
)

// Chunk is one partition of a release name, with the span of words it covers.
//
// The span is what keeps evidence honest: chunks overlap by construction — the
// whole body and each of its dash-separated parts are all chunks — so two hits
// on the same words have to be recognised as one piece of evidence rather than
// two.
type Chunk struct {
	// Text is the chunk as the release wrote it.
	Text string
	// Words are its alphanumeric words, lower-cased.
	Words []string
	// Start and End are word offsets into the whole name, End exclusive.
	Start, End int
	// Position is where the chunk came from.
	Position Position
}

// Reading is a name taken apart: the chunks to ask the catalogue about, plus
// the date lifted out of it.
type Reading struct {
	// Name is the release name as announced.
	Name string
	// Chunks are the partitions, longest first.
	Chunks []Chunk
	// Date is the release date as written, where the name carried one.
	Date string
	// Body is the name with the header, date and technical tokens removed —
	// what is left to identify the release by.
	Body string
}

// separators end a chunk. Everything here divides one thing from another in at
// least some releases; none of them divides reliably in all, which is why a
// split produces a candidate rather than a conclusion.
const separators = `-–—,&+/|:;()[]{}`

// joiners are words that separate rather than name. A TLD is one of them: the
// catalogue knows "adulttime", the release writes "AdultTime.com", and the dot
// cannot be a separator in general because "jane.doe" is one name.
var joiners = map[string]bool{
	"and": true, "with": true, "feat": true, "featuring": true, "vs": true,
	"com": true, "net": true, "org": true, "tv": true, "xxx": true, "cc": true, "io": true,
}

// Read takes a release name apart.
func Read(name string) Reading {
	header, body := detachHeader(name)

	date := ""
	for _, re := range dates {
		if m := re.FindStringSubmatch(body); m != nil {
			date = m[1]
			body = strings.Replace(body, m[0], " ", 1)
			break
		}
	}
	body = prune(body)
	header = prune(header)

	out := Reading{Name: name, Date: date, Body: body}
	// The header is tokenised first so body spans continue its numbering and
	// a span is unique across the whole name.
	headerTokens := tokenise(header)
	bodyTokens := tokenise(body)

	out.Chunks = append(out.Chunks, chunksOf(headerTokens, 0, PositionHeader)...)
	out.Chunks = append(out.Chunks, chunksOf(bodyTokens, len(headerTokens.words), PositionBody)...)
	return out
}

// token stream: the words, and the raw text separating each pair of them.
type tokens struct {
	words []string
	// gaps[i] is what sat between words[i] and words[i+1].
	gaps []string
}

// tokenise splits text into words and the gaps between them.
func tokenise(text string) tokens {
	var out tokens
	var word strings.Builder
	var gap strings.Builder

	flush := func() {
		if word.Len() == 0 {
			return
		}
		if len(out.words) > 0 {
			out.gaps = append(out.gaps, gap.String())
		}
		gap.Reset()
		out.words = append(out.words, word.String())
		word.Reset()
	}

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			word.WriteRune(unicode.ToLower(r))
			continue
		}
		if word.Len() > 0 {
			flush()
		}
		gap.WriteRune(r)
	}
	flush()
	return out
}

// chunksOf partitions a token stream: the whole of it, and every run between
// the separators it contains.
//
// Both, deliberately. A delimiter is a real boundary often enough to be worth
// splitting on and unreliable often enough that the unsplit reading has to
// survive alongside it — so they are offered together and the catalogue picks.
func chunksOf(t tokens, offset int, pos Position) []Chunk {
	if len(t.words) == 0 {
		return nil
	}

	var out []Chunk
	add := func(start, end int) {
		if end-start < 1 {
			return
		}
		words := t.words[start:end]
		// A run of joiners alone names nothing.
		if allJoiners(words) {
			return
		}
		out = append(out, Chunk{
			Text:     strings.Join(words, " "),
			Words:    words,
			Start:    offset + start,
			End:      offset + end,
			Position: pos,
		})
	}

	add(0, len(t.words))

	start := 0
	for i, gap := range t.gaps {
		if !divides(gap) && !joiners[t.words[i+1]] {
			continue
		}
		add(start, i+1)
		start = i + 1
		// A joining word belongs to neither side.
		if joiners[t.words[i+1]] {
			start = i + 2
		}
	}
	if start < len(t.words) && start > 0 {
		add(start, len(t.words))
	}
	return dedupe(out)
}

// divides reports whether a gap separates two things rather than two words of
// one thing. A bare space does not; a space around a dash does.
func divides(gap string) bool {
	return strings.ContainsAny(gap, separators)
}

func allJoiners(words []string) bool {
	for _, w := range words {
		if !joiners[w] {
			return false
		}
	}
	return true
}

// dedupe drops chunks covering the same span, which happens whenever a name
// has no separators and the whole of it is also its only run.
func dedupe(in []Chunk) []Chunk {
	seen := map[[2]int]bool{}
	var out []Chunk
	for _, c := range in {
		key := [2]int{c.Start, c.End}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

// detachHeader splits a leading bracket or parenthesis off the front.
func detachHeader(name string) (header, body string) {
	if m := headerPattern.FindStringSubmatch(name); m != nil {
		return strings.TrimSpace(m[1]), strings.TrimSpace(name[len(m[0]):])
	}
	return "", strings.TrimSpace(name)
}
