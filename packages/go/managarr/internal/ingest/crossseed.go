package ingest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// crossSeedHook is cross-seed's notification body.
//
// The shape is {title, body, extra}: the first two are prose for a chat sink
// and everything machine-readable is under extra.
type crossSeedHook struct {
	Extra struct {
		Event    string `json:"event"`
		Name     string `json:"name"`
		InfoHash string `json:"infoHash"`
		// InfoHashes is every candidate cross-seed produced from one searchee.
		// A single announce routinely matches on several trackers at once, and
		// older builds send only the singular InfoHash above.
		InfoHashes []string `json:"infoHashes"`
		// Result is SAVED, INJECTED or FAILURE — what cross-seed did with a
		// match, which its SQLite decision log does not record.
		Result    string   `json:"result"`
		Decision  string   `json:"decision"`
		Decisions []string `json:"decisions"`
		Trackers  []string `json:"trackers"`
		Source    string   `json:"source"`
		// Searchee is the torrent the search ran FROM, and it carries the
		// source INFOHASH.
		//
		// This is the only place in the entire stack that states the pair.
		// cross-seed's SQLite has a searchee table of (id, name) with no hash,
		// so its decision log can say what came out and never what went in —
		// which left the source resolved by name, refused whenever a name
		// covered several torrents, and recorded for barely a quarter of
		// candidates.
		//
		// Here it is, full length, from cross-seed itself, at the moment of the
		// match. Nothing needs to be inferred from a title, a file list or an
		// inode.
		Searchee struct {
			Name     string `json:"name"`
			InfoHash string `json:"infoHash"`
			// Source is "torrentClient" or "dataDir". A dataDir match ran
			// against a path rather than a torrent, so it has no infohash and
			// there is genuinely no pair to record.
			Source     string   `json:"source"`
			ClientHost string   `json:"clientHost"`
			Category   string   `json:"category"`
			Tags       []string `json:"tags"`
			Trackers   []string `json:"trackers"`
			Length     int64    `json:"length"`
			Path       *string  `json:"path"`
		} `json:"searchee"`
	} `json:"extra"`
}

// candidates returns every infohash cross-seed produced, in either shape.
func (x crossSeedHook) candidates() []string {
	seen := make(map[string]struct{})
	var out []string
	for _, h := range append([]string{x.Extra.InfoHash}, x.Extra.InfoHashes...) {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			continue
		}
		if _, dup := seen[h]; dup {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	return out
}

// crossSeed records an injection.
//
// cross-seed's webhook fires only on a match, so this handler sees a fraction
// of what its database holds — the rejections are four fifths of the record and
// come from the SQLite pass. What the webhook adds is the outcome: whether the
// match was injected, merely saved, or failed. The decision table records that
// a candidate matched and never says what happened next.
//
// The offer is keyed exactly as the SQLite pass keys it, so the two write the
// same row rather than two rows for one event.
func (i *Ingest) crossSeed(ctx context.Context, body []byte) error {
	var hook crossSeedHook
	if err := decode(body, &hook); err != nil {
		return fmt.Errorf("cross-seed: decode: %w", err)
	}
	x := hook.Extra
	candidates := hook.candidates()
	if len(candidates) == 0 || i.cfg.CrossSeed == "" {
		return nil
	}

	searchee := x.Searchee.Name
	if searchee == "" {
		// Older cross-seed builds put the searchee in `source`.
		searchee = x.Source
	}
	if searchee == "" {
		return nil
	}

	// The source torrent, by identifier.
	//
	// Empty for a dataDir match, where the search ran against a path and there
	// was no torrent to name — that is a real absence rather than a lookup
	// that failed, so the offer is still recorded, just without the edge.
	var sourceHash *string
	if h := strings.ToLower(strings.TrimSpace(x.Searchee.InfoHash)); h != "" {
		sourceHash = &h
	}

	if _, err := i.lookups.Service(ctx, i.cfg.CrossSeed, "cross-seed"); err != nil {
		return err
	}
	indexer, err := i.lookups.Indexer(ctx, i.cfg.CrossSeed)
	if err != nil || indexer == nil {
		return err
	}
	releaseID, err := i.lookups.Release(ctx, x.Name)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	outcome := crossSeedOutcome(x.Result)

	// One announce matching on several trackers is one searchee and several
	// candidates. Each is its own offer — they are different torrents on
	// different trackers — and they all name the same source.
	offers := make([]model.Offer, 0, len(candidates))
	refs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		ref := searchee + "|" + candidate
		refs = append(refs, ref)
		offers = append(offers, model.Offer{
			Kind:         model.OfferCrossSeedCandidate,
			IndexerName:  *indexer,
			SourceRef:    ref,
			DiscoveredAt: now,
			ReleaseID:    releaseID,
			// The candidate's own hash. Not a foreign key: cross-seed proposes
			// torrents it does not have and frequently never obtains.
			Infohash:       &candidate,
			SourceInfohash: sourceHash,
		})
	}

	// discovered_at is left alone on conflict: the SQLite pass knows when the
	// decision was first made, and this handler only knows when it was told.
	//
	// source_infohash IS updated, because this handler is the only writer that
	// has it — the SQLite pass has to resolve a name and gives up whenever one
	// covers more than one torrent.
	if err := dao.Upsert(ctx, i.dao, offers,
		[]string{"indexer_name", "kind", "source_ref"},
		[]string{"release_id", "infohash", "source_infohash"}); err != nil {
		return fmt.Errorf("cross-seed: offers: %w", err)
	}

	o := i.dao.Q().Offer
	stored, err := o.WithContext(ctx).
		Select(o.ID, o.SourceRef).
		Where(
			o.IndexerName.Eq(*indexer),
			o.Kind.Eq(model.OfferCrossSeedCandidate),
			o.SourceRef.In(refs...),
		).
		Find()
	if err != nil {
		return fmt.Errorf("cross-seed: reread offers: %w", err)
	}

	decisions := make([]model.Decision, 0, len(stored))
	for _, o := range stored {
		decisions = append(decisions, model.Decision{
			OfferID:   o.ID,
			Decider:   i.cfg.CrossSeed,
			Stage:     model.StagePush,
			Outcome:   outcome,
			Status:    strPtr(x.Result),
			DecidedAt: &now,
		})
	}
	return dao.Upsert(ctx, i.dao, decisions,
		[]string{"offer_id", "decider", "stage"},
		[]string{"outcome", "status", "decided_at"})
}

// crossSeedOutcome maps an injection result.
//
// FAILURE is errored rather than rejected: cross-seed wanted the candidate and
// could not act on it, which is a broken integration and not a judgement about
// the release.
func crossSeedOutcome(result string) string {
	switch strings.ToUpper(result) {
	case "INJECTED", "SAVED":
		return model.OutcomeAccepted
	case "FAILURE":
		return model.OutcomeErrored
	default:
		return model.OutcomeAccepted
	}
}
