package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"homelab/clients/crossseed"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
	"homelab/sidecar"
)

// CrossSeed replays cross-seed's decision log.
//
// cross-seed's webhook fires only on an injection, so its API and its webhook
// together show the matches and none of the rejections — and the rejections are
// four fifths of the record. SIZE_MISMATCH, FILE_TREE_MISMATCH and
// INFO_HASH_ALREADY_EXISTS are the answer to "why was this not cross-seeded".
//
// They come from managarr-crossseed, the sidecar running inside cross-seed's
// own container, read through the generic sidecar contract rather than a
// cross-seed-specific client. This pass used to read that database file over a
// bind mount, which needed a matching uid and a schema this side had to know.
//
// Each decision becomes an offer of kind cross_seed_candidate — a release
// proposed through an indexer at a time — plus the verdict on it, which is the
// same shape as every other candidate in the model.
type CrossSeed struct {
	client  *sidecar.Client
	walk    *SidecarWalk
	dao     *dao.DAO
	lookups *dao.Lookups
	logger  *slog.Logger
	decider string
}

// NewCrossSeed builds the pass.
func NewCrossSeed(c *sidecar.Client, d *dao.DAO, l *dao.Lookups, logger *slog.Logger, decider string, pageSize int) *CrossSeed {
	return &CrossSeed{
		client:  c,
		walk:    NewSidecarWalk(c, d, logger, crossseed.Source, pageSize),
		dao:     d,
		lookups: l,
		logger:  logger,
		decider: decider,
	}
}

// CrossSeedStats reports what a run did.
type CrossSeedStats struct {
	Offers    int
	Decisions int
	Linked    int
	// Resumed is the cursor the pass started from, empty on a first run.
	// Reported because "wrote 0 offers" means the opposite thing depending on
	// it: caught up, or never started.
	Resumed string
	Took    time.Duration
}

// Run replays every decision.
func (c *CrossSeed) Run(ctx context.Context) (CrossSeedStats, error) {
	var st CrossSeedStats
	started := time.Now()
	c.lookups.Reset()

	if _, err := c.lookups.Service(ctx, c.decider, "cross-seed"); err != nil {
		return st, err
	}

	// cross-seed is not an indexer, but an offer is keyed by one, so its own
	// name stands in. The alternative — the tracker the candidate was found on
	// — is not in the decision row.
	indexerName, err := c.lookups.Indexer(ctx, c.decider)
	if err != nil {
		return st, err
	}
	if indexerName == nil {
		return st, nil
	}

	// The only bridge from a name-keyed searchee to a real torrent. Loaded
	// once: it is a few thousand rows against a query per decision.
	byName, err := c.searcheeHashes(ctx)
	if err != nil {
		return st, err
	}

	// Resumes from the stored cursor, so a steady-state pass reads only what
	// cross-seed decided in the last hour rather than replaying all 48,000
	// decisions. The walker only advances past a page this handler accepted,
	// which is what makes a mid-pass failure cost a re-read instead of a gap.
	walk, err := c.walk.Run(ctx, func(ctx context.Context, page []sidecar.Event) error {
		decisions, err := decodeDecisions(page)
		if err != nil {
			return err
		}
		offers, written, linked, err := c.writePage(ctx, decisions, *indexerName, byName)
		if err != nil {
			return err
		}
		st.Offers += offers
		st.Decisions += written
		st.Linked += linked
		return nil
	})
	if err != nil {
		return st, err
	}
	st.Resumed = walk.Resumed

	st.Took = time.Since(started)
	return st, nil
}

// decodeDecisions unwraps a page of sidecar events.
//
// Events of another kind are skipped rather than refused: a sidecar is free to
// grow a new kind, and a consumer that dies on the first one it does not
// recognise makes every such addition a breaking change.
func decodeDecisions(page []sidecar.Event) ([]crossseed.Decision, error) {
	out := make([]crossseed.Decision, 0, len(page))
	for _, e := range page {
		if e.Kind != crossseed.KindDecision {
			continue
		}
		d, err := sidecar.DecodePayload[crossseed.Decision](e)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// writePage records one page of decisions in a fixed number of statements.
//
// Same shape as the autobrr walk and for the same reason: a decision resolved
// one at a time is an upsert and a read per row, and this table runs to tens of
// thousands.
func (c *CrossSeed) writePage(
	ctx context.Context,
	page []crossseed.Decision,
	indexerName string,
	byName map[string]string,
) (offers, decisions, linked int, err error) {
	titles := make([]string, 0, len(page))
	for _, d := range page {
		titles = append(titles, d.SearcheeName)
	}
	releaseIDs, err := c.lookups.ReleaseIDs(ctx, titles)
	if err != nil {
		return 0, 0, 0, err
	}

	rows := make([]model.Offer, 0, len(page))
	refs := make([]string, 0, len(page))

	for _, d := range page {
		candidate := strings.ToLower(d.InfoHash)
		ref := d.SearcheeName + "|" + candidate

		var sourceHash *string
		if h, ok := byName[d.SearcheeName]; ok {
			// The searchee is a torrent this stack holds, so the offer can say
			// what it was matched against.
			sourceHash, err = c.lookups.Torrent(ctx, h)
			if err != nil {
				return 0, 0, 0, err
			}
			linked++
		}

		row := model.Offer{
			Kind:         model.OfferCrossSeedCandidate,
			IndexerName:  indexerName,
			SourceRef:    ref,
			DiscoveredAt: d.FirstSeen,
			// The candidate's hash. Not a foreign key: a proposed torrent is
			// frequently never obtained, so the hash is real while the torrent
			// is not one this stack holds.
			Infohash:       strPtr(candidate),
			SourceInfohash: sourceHash,
			// Carries the indexer's RSS key; stripped by the model hook.
			DownloadURL: strPtr(d.GUID),
		}
		if id, ok := releaseIDs[d.SearcheeName]; ok {
			row.ReleaseID = &id
		}
		rows = append(rows, row)
		refs = append(refs, ref)
	}

	if err := dao.Upsert(ctx, c.dao, rows,
		[]string{"indexer_name", "kind", "source_ref"},
		[]string{"release_id", "infohash", "source_infohash", "download_url"},
	); err != nil {
		return 0, 0, 0, fmt.Errorf("crossseed: offers: %w", err)
	}

	offerIDs, err := c.offerIDs(ctx, indexerName, refs)
	if err != nil {
		return 0, 0, 0, err
	}

	decisionRows := make([]model.Decision, 0, len(page))
	for _, d := range page {
		ref := d.SearcheeName + "|" + strings.ToLower(d.InfoHash)
		id, ok := offerIDs[ref]
		if !ok {
			continue
		}
		decisionRows = append(decisionRows, model.Decision{
			OfferID:   id,
			Decider:   c.decider,
			Stage:     model.StagePush,
			Outcome:   crossSeedOutcome(d.Decision),
			Status:    strPtr(d.Decision),
			DecidedAt: timePtr(d.LastSeen),
		})
	}
	if err := dao.Upsert(ctx, c.dao, decisionRows,
		[]string{"offer_id", "decider", "stage"},
		[]string{"outcome", "status", "decided_at"},
	); err != nil {
		return 0, 0, 0, fmt.Errorf("crossseed: decisions: %w", err)
	}

	return len(rows), len(decisionRows), linked, nil
}

// offerIDs reads back the page's offers in one query.
func (c *CrossSeed) offerIDs(ctx context.Context, indexerName string, refs []string) (map[string]model.ID, error) {
	o := c.dao.Q().Offer
	rows, err := o.WithContext(ctx).
		Select(o.ID, o.SourceRef).
		Where(
			o.IndexerName.Eq(indexerName),
			o.Kind.Eq(model.OfferCrossSeedCandidate),
			o.SourceRef.In(refs...),
		).
		Find()
	if err != nil {
		return nil, fmt.Errorf("crossseed: offer ids: %w", err)
	}
	out := make(map[string]model.ID, len(rows))
	for _, r := range rows {
		out[r.SourceRef] = r.ID
	}
	return out, nil
}

// crossSeedOutcome maps cross-seed's verdict vocabulary.
//
// Everything that is not a match is a rejection: cross-seed looked at the
// candidate and declined it. None of its outcomes are errors in the sense the
// model means — a failed fetch is recorded by autobrr's push decision instead.
func crossSeedOutcome(decision string) string {
	if decision == "MATCH" {
		return model.OutcomeAccepted
	}
	return model.OutcomeRejected
}

// searcheeHashes maps a searchee name to the one torrent it resolves to.
//
// Deliberately one-to-one and refused when ambiguous: cross-seed keys a
// searchee by NAME, and one name routinely covers several infohashes — they are
// cross-seeds of each other — so a name with more than one cannot say which was
// searched from. A wrong edge would attribute a decision to the wrong torrent.
func (c *CrossSeed) searcheeHashes(ctx context.Context) (map[string]string, error) {
	// A service-specific route, not /changes: this is a lookup table rather
	// than a stream, and managarr wants it whole.
	var body struct {
		ClientTorrents []crossseed.ClientTorrent `json:"clientTorrents"`
	}
	if err := c.client.Get(ctx, crossseed.ClientTorrentsPath, &body); err != nil {
		return nil, err
	}
	torrents := body.ClientTorrents
	hashes := make(map[string][]string, len(torrents))
	for _, t := range torrents {
		hashes[t.Name] = append(hashes[t.Name], strings.ToLower(t.InfoHash))
	}
	out := make(map[string]string, len(hashes))
	for name, hs := range hashes {
		if len(hs) == 1 {
			out[name] = hs[0]
		}
	}
	return out, nil
}
