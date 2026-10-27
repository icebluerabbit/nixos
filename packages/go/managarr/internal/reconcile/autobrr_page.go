package reconcile

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"homelab/clients/autobrr"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// A page of autobrr history is written with a fixed number of statements,
// regardless of how many releases it holds.
//
// It used to be roughly five queries PER RELEASE — resolve the release, check
// whether the offer existed, upsert it, read its id back, upsert its decisions.
// Against a cold database and autobrr's six-figure history that is over half a
// million round trips, which is why a backfill ran for half an hour at five
// percent CPU: almost all of it was waiting.
//
// The shape below is six statements per page instead. Everything that was a
// per-row lookup becomes one set-based read whose result is held in a map for
// the rest of the page.

// writePage records one page of releases: the releases themselves, the offers
// they were announced as, and every verdict taken on them.
//
// Returns how many offers were written and how many of them the store had not
// seen before, which is what stops an incremental walk.
func (a *Autobrr) writePage(ctx context.Context, page []autobrr.Release) (offers, decisions, fresh int, err error) {
	usable := make([]autobrr.Release, 0, len(page))
	for _, r := range page {
		// An indexer-less release cannot be keyed. autobrr has not been seen to
		// emit one, so this is a guard rather than a case.
		if r.Name == "" || r.Indexer.Name == "" {
			continue
		}
		if _, err := a.lookups.Indexer(ctx, r.Indexer.Name); err != nil {
			return 0, 0, 0, err
		}
		usable = append(usable, r)
	}
	if len(usable) == 0 {
		return 0, 0, 0, nil
	}

	// 1. The releases, and their ids.
	releaseIDs, err := a.writeReleasePage(ctx, usable)
	if err != nil {
		return 0, 0, 0, err
	}

	// 2. Which offers already exist — asked BEFORE the upsert, because
	//    afterwards they all do and a new page is indistinguishable from a
	//    stored one.
	existing, err := a.existingOffers(ctx, usable)
	if err != nil {
		return 0, 0, 0, err
	}

	rows := make([]model.Offer, 0, len(usable))
	for _, r := range usable {
		row := a.offerRow(r)
		if id, ok := releaseIDs[r.Name]; ok {
			row.ReleaseID = &id
		}
		rows = append(rows, row)
	}

	// 3. The offers.
	//
	// infohash is NOT in the update list: autobrr does not serialise it, so
	// writing it here would blank whatever the webhook recorded — and the
	// webhook is its only source.
	if err := dao.Upsert(ctx, a.dao, rows,
		[]string{"indexer_name", "kind", "source_ref"},
		[]string{"release_id", "announce_type", "protocol", "categories",
			"uploader", "pre_time", "info_url", "download_url",
			"normalized_hash", "size_bytes", "tracker_torrent_id"},
	); err != nil {
		return 0, 0, 0, fmt.Errorf("autobrr: offers: %w", err)
	}

	// 4. Their ids, now that every row is present.
	offerIDs, err := a.offerIDs(ctx, usable)
	if err != nil {
		return 0, 0, 0, err
	}

	// 5. Every verdict on the page, in one statement.
	var decisionRows []model.Decision
	for _, r := range usable {
		id, ok := offerIDs[offerKey(r.Indexer.Name, r.ID)]
		if !ok {
			// The offer did not come back, which means the upsert did not
			// write it. Skipping its decisions is right: a decision with no
			// offer has nothing to be about.
			continue
		}
		decisionRows = append(decisionRows, a.decisionRows(r, id)...)
	}
	if err := dao.Upsert(ctx, a.dao, dedupeDecisions(decisionRows),
		[]string{"offer_id", "decider", "stage"},
		[]string{"outcome", "status", "reason", "filter_name", "decided_at"},
	); err != nil {
		return 0, 0, 0, fmt.Errorf("autobrr: decisions: %w", err)
	}

	for _, r := range usable {
		if _, seen := existing[offerKey(r.Indexer.Name, r.ID)]; !seen {
			fresh++
		}
	}
	return len(rows), len(decisionRows), fresh, nil
}

// writeReleasePage upserts the page's releases and returns their ids by title.
//
// Two statements: the batched COALESCE upsert, then one read. That read is what
// replaces a per-release call to lookups.Release, which was an upsert and a
// RETURNING of its own for every single record.
func (a *Autobrr) writeReleasePage(ctx context.Context, page []autobrr.Release) (map[string]model.ID, error) {
	if err := a.writeReleases(ctx, page); err != nil {
		return nil, err
	}

	titles := make([]string, 0, len(page))
	seen := make(map[string]struct{}, len(page))
	for _, r := range page {
		if _, dup := seen[r.Name]; dup {
			continue
		}
		seen[r.Name] = struct{}{}
		titles = append(titles, r.Name)
	}

	rl := a.dao.Q().Release
	rows, err := rl.WithContext(ctx).
		Select(rl.ID, rl.Title).
		Where(rl.Title.In(titles...)).
		Find()
	if err != nil {
		return nil, fmt.Errorf("autobrr: release ids: %w", err)
	}

	out := make(map[string]model.ID, len(rows))
	for _, r := range rows {
		out[r.Title] = r.ID
	}
	return out, nil
}

// existingOffers is the set of the page's offers already stored.
func (a *Autobrr) existingOffers(ctx context.Context, page []autobrr.Release) (map[string]struct{}, error) {
	rows, err := a.findOffers(ctx, page)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		out[offerKey(r.IndexerName, mustParseRef(r.SourceRef))] = struct{}{}
	}
	return out, nil
}

// offerIDs maps the page's offers to their ids.
func (a *Autobrr) offerIDs(ctx context.Context, page []autobrr.Release) (map[string]model.ID, error) {
	rows, err := a.findOffers(ctx, page)
	if err != nil {
		return nil, err
	}
	out := make(map[string]model.ID, len(rows))
	for _, r := range rows {
		out[offerKey(r.IndexerName, mustParseRef(r.SourceRef))] = r.ID
	}
	return out, nil
}

// findOffers reads the page's offers in one query.
//
// Scoped by kind as well as by source_ref: a cross-seed candidate can share an
// indexer with an announce, and only announces are keyed by autobrr's id.
func (a *Autobrr) findOffers(ctx context.Context, page []autobrr.Release) ([]*model.Offer, error) {
	refs := make([]string, 0, len(page))
	indexers := make([]string, 0, len(page))
	seenRef := make(map[string]struct{}, len(page))
	seenIx := make(map[string]struct{}, len(page))

	for _, r := range page {
		ref := strconv.FormatInt(r.ID, 10)
		if _, dup := seenRef[ref]; !dup {
			seenRef[ref] = struct{}{}
			refs = append(refs, ref)
		}
		if _, dup := seenIx[r.Indexer.Name]; !dup {
			seenIx[r.Indexer.Name] = struct{}{}
			indexers = append(indexers, r.Indexer.Name)
		}
	}

	o := a.dao.Q().Offer
	rows, err := o.WithContext(ctx).
		Select(o.ID, o.IndexerName, o.SourceRef).
		Where(
			o.Kind.Eq(model.OfferAnnounce),
			o.IndexerName.In(indexers...),
			o.SourceRef.In(refs...),
		).
		Find()
	if err != nil {
		return nil, fmt.Errorf("autobrr: offer ids: %w", err)
	}
	return rows, nil
}

// offerRow converts one release into the offer it was announced as.
func (a *Autobrr) offerRow(r autobrr.Release) model.Offer {
	return model.Offer{
		Kind:         model.OfferAnnounce,
		IndexerName:  r.Indexer.Name,
		SourceRef:    strconv.FormatInt(r.ID, 10),
		DiscoveredAt: r.Timestamp.UTC(),
		AnnounceType: strPtr(r.AnnounceType),
		Protocol:     strPtr(r.Protocol),
		Categories:   strPtr(r.Category),
		Uploader:     strPtr(r.Uploader),
		PreTime:      strPtr(r.PreTime),
		// These carry per-account tracker keys; the model's BeforeSave hook
		// strips them.
		InfoURL:          strPtr(r.InfoURL),
		DownloadURL:      strPtr(r.DownloadURL),
		NormalizedHash:   strPtr(r.NormalizedHash),
		TrackerTorrentID: strPtr(r.TorrentID),
		SizeBytes:        int64Ptr(r.Size),
	}
}

// decisionRows builds every verdict recorded against one release.
func (a *Autobrr) decisionRows(r autobrr.Release, offerID model.ID) []model.Decision {
	rows := []model.Decision{{
		OfferID:    offerID,
		Decider:    a.serviceName,
		Stage:      model.StageFilter,
		Outcome:    filterOutcome(r.FilterStatus),
		Status:     strPtr(r.FilterStatus),
		Reason:     strPtr(strings.Join(r.Rejections, "; ")),
		FilterName: strPtr(r.FilterName),
		DecidedAt:  timePtr(r.Timestamp),
	}}

	// One per action target: what each downstream service did with it.
	for _, act := range r.ActionStatus {
		rows = append(rows, model.Decision{
			OfferID:   offerID,
			Decider:   act.Action,
			Stage:     model.StagePush,
			Outcome:   pushOutcome(act.Status),
			Status:    strPtr(act.Status),
			Reason:    strPtr(strings.Join(act.Rejections, "; ")),
			DecidedAt: timePtr(act.Timestamp),
		})
	}
	return rows
}

// offerKey identifies an announce by the pair both autobrr and this store agree
// on: which indexer, and autobrr's own id for the record.
func offerKey(indexer string, id int64) string {
	return indexer + "\x00" + strconv.FormatInt(id, 10)
}

// mustParseRef reads a source_ref back as the integer it was written from.
//
// Every announce's ref is autobrr's numeric release id, so a value that does
// not parse is a row written by something else and is skipped by returning a
// sentinel that cannot match any real key.
func mustParseRef(ref string) int64 {
	n, err := strconv.ParseInt(ref, 10, 64)
	if err != nil {
		return -1
	}
	return n
}
