package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"homelab/clients/prowlarr"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
	"homelab/sidecar"
	"homelab/sidecar/prowlarrsvc"
)

// Prowlarr replays Prowlarr's history.
//
// Prowlarr proxies every indexer query in this stack, which makes it the only
// record of what was looked FOR as opposed to what was taken. A search that
// returned nothing is recorded exactly like one that returned a hundred
// results, and the empty ones are the interesting half.
//
// RSS polls are deliberately not recorded: a scheduled feed fetch is not a
// question anyone asked, and at 44k records they would outnumber real searches
// two to one.
type Prowlarr struct {
	client *prowlarr.Client
	// walk reads both history streams through the sidecar behind one
	// cursor. Nil walks Prowlarr directly, once per event type.
	walk        *SidecarWalk
	dao         *dao.DAO
	lookups     *dao.Lookups
	logger      *slog.Logger
	serviceName string
	pageSize    int
	full        bool
}

// NewProwlarr builds the pass.
func NewProwlarr(c *prowlarr.Client, walk *SidecarWalk, d *dao.DAO, l *dao.Lookups, logger *slog.Logger, serviceName string, pageSize int, full bool) *Prowlarr {
	if pageSize <= 0 {
		pageSize = 500
	}
	return &Prowlarr{
		client: c, walk: walk, dao: d, lookups: l, logger: logger,
		serviceName: serviceName, pageSize: pageSize, full: full,
	}
}

// ProwlarrStats reports what a run did.
type ProwlarrStats struct {
	Indexers int
	Searches int
	Grabs    int
	Took     time.Duration
}

// Run executes one pass.
func (p *Prowlarr) Run(ctx context.Context) (ProwlarrStats, error) {
	var st ProwlarrStats
	started := time.Now()
	p.lookups.Reset()

	if _, err := p.lookups.Service(ctx, p.serviceName, "prowlarr"); err != nil {
		return st, err
	}

	n, err := p.syncIndexers(ctx)
	if err != nil {
		return st, err
	}
	st.Indexers = n

	byID, err := p.indexerNames(ctx)
	if err != nil {
		return st, err
	}

	if p.walk != nil {
		// One walk covering both event types, because the sidecar serves them
		// as a single cursored stream. Prowlarr's own API cannot do that: it
		// filters by one eventType per request, which is why the direct path
		// below is two separate walks.
		if st.Searches, st.Grabs, err = p.syncViaSidecar(ctx, byID); err != nil {
			return st, err
		}
	} else {
		if st.Searches, err = p.syncSearches(ctx, byID); err != nil {
			return st, err
		}
		if st.Grabs, err = p.syncGrabs(ctx, byID); err != nil {
			return st, err
		}
	}

	st.Took = time.Since(started)
	return st, nil
}

// syncIndexers records indexers as Prowlarr defines them.
//
// Prowlarr owns protocol, privacy and priority — it is where those settings
// live. enabled is autobrr's to write: the two services enable indexers
// independently, and whichever pass ran last would otherwise decide.
func (p *Prowlarr) syncIndexers(ctx context.Context) (int, error) {
	list, err := p.client.Indexers(ctx)
	if err != nil {
		return 0, err
	}
	rows := make([]model.Indexer, 0, len(list))
	for _, ix := range list {
		rows = append(rows, model.Indexer{
			Name:     ix.Name,
			Protocol: strPtr(ix.Protocol),
			Privacy:  strPtr(ix.Privacy),
			Priority: intPtr(ix.Priority),
		})
	}
	if err := dao.Upsert(ctx, p.dao, rows, []string{"name"},
		[]string{"protocol", "privacy", "priority"}); err != nil {
		return 0, fmt.Errorf("prowlarr: indexers: %w", err)
	}
	return len(rows), nil
}

// indexerNames maps Prowlarr's numeric indexer ids to names, which is what the
// model keys on.
func (p *Prowlarr) indexerNames(ctx context.Context) (map[int]string, error) {
	list, err := p.client.Indexers(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int]string, len(list))
	for _, ix := range list {
		out[ix.ID] = ix.Name
	}
	return out, nil
}

// syncSearches records indexerQuery history.
//
// content_id is NOT resolved here. It is derived from the external ids this row
// carries, and this pass often cannot resolve it because the *arr pass may not
// have created the content yet. Resolve fills it afterwards, NULL-only.
func (p *Prowlarr) syncSearches(ctx context.Context, byID map[int]string) (int, error) {
	var count int
	var walkErr error

	err := p.client.History(ctx, prowlarr.EventIndexerQuery, p.pageSize, func(page []prowlarr.HistoryRecord) bool {
		n, fresh, err := p.writeSearchPage(ctx, page, byID)
		count += n
		if err != nil {
			walkErr = err
			return false
		}
		p.logger.InfoContext(ctx, "prowlarr progress",
			"kind", "searches", "records", count, "new_this_page", fresh)
		return p.full || fresh > 0
	})
	if walkErr != nil {
		return count, walkErr
	}
	return count, err
}

// writeSearchPage records one page of indexerQuery history.
//
// Shared by the direct walk and the sidecar walk so the row-building rules
// exist once. fresh counts records that were new to the store, which only the
// direct walk needs — it has no cursor and infers its stopping point from it.
func (p *Prowlarr) writeSearchPage(
	ctx context.Context,
	page []prowlarr.HistoryRecord,
	byID map[int]string,
) (count, fresh int, err error) {
	{
		rows := make([]model.Search, 0, len(page))
		refs := make([]string, 0, len(page))
		for _, r := range page {
			name := byID[r.IndexerID]
			if name != "" {
				if _, err := p.lookups.Indexer(ctx, name); err != nil {
					return count, fresh, err
				}
			}
			elapsed, _ := r.Int("elapsedTime")
			results, _ := r.Int("queryResults")
			season, _ := r.Int("season")
			episode, _ := r.Int("episode")
			cached, cachedOK := r.Bool("cached")

			var cachedVal *bool
			if cachedOK {
				cachedVal = &cached
			}
			initiator := r.Str("source")
			if initiator == "" {
				// Prowlarr's own UI issues searches with no source.
				initiator = "manual"
			}

			ref := strconv.FormatInt(r.ID, 10)
			refs = append(refs, ref)
			rows = append(rows, model.Search{
				ServiceName: p.serviceName,
				SourceRef:   ref,
				StartedAt:   r.Date.UTC(),
				IndexerName: strPtr(name),
				Initiator:   strPtr(initiator),
				Query:       strPtr(r.Str("query")),
				QueryType:   strPtr(r.Str("queryType")),
				ResultCount: intPtr(results),
				ElapsedMs:   intPtr(elapsed),
				Cached:      cachedVal,
				// Carries the account passkey; stripped by the model hook.
				URL:        strPtr(r.Str("url")),
				ImdbID:     strPtr(r.Str("imdbId")),
				TmdbID:     strPtr(r.Str("tmdbId")),
				TvdbID:     strPtr(r.Str("tvdbId")),
				Season:     intPtr(season),
				Episode:    intPtr(episode),
				Categories: strPtr(r.Str("categories")),
			})
		}

		// Counted BEFORE the upsert: afterwards every row exists, so there is
		// nothing left to tell a page of new records from one already stored.
		fresh, err = p.countNewSearches(ctx, refs)
		if err != nil {
			return count, fresh, err
		}

		if err := dao.Upsert(ctx, p.dao, rows,
			[]string{"service_name", "source_ref"},
			[]string{"result_count", "indexer_name", "elapsed_ms", "cached"},
		); err != nil {
			return count, fresh, fmt.Errorf("prowlarr: searches: %w", err)
		}
		count = len(rows)
	}
	return count, fresh, nil
}

// syncGrabs records releaseGrabbed history.
//
// A Prowlarr grab carries a title and no infohash, so it cannot name the
// torrent it produced — it answers which indexer a release was pulled through
// and which client asked, which nothing else does.
func (p *Prowlarr) syncGrabs(ctx context.Context, byID map[int]string) (int, error) {
	var count int
	var walkErr error

	err := p.client.History(ctx, prowlarr.EventReleaseGrabbed, p.pageSize, func(page []prowlarr.HistoryRecord) bool {
		n, fresh, err := p.writeGrabPage(ctx, page, byID)
		count += n
		if err != nil {
			walkErr = err
			return false
		}
		p.logger.InfoContext(ctx, "prowlarr progress",
			"kind", "grabs", "records", count, "new_this_page", fresh)
		return p.full || fresh > 0
	})
	if walkErr != nil {
		return count, walkErr
	}
	return count, err
}

// writeGrabPage records one page of releaseGrabbed history.
//
// Shared by the direct walk and the sidecar walk, for the same reason as
// writeSearchPage: two copies of these rules would drift silently, since both
// write the same table.
func (p *Prowlarr) writeGrabPage(
	ctx context.Context,
	page []prowlarr.HistoryRecord,
	byID map[int]string,
) (count, fresh int, err error) {
	{
		rows := make([]model.IndexerGrab, 0, len(page))
		refs := make([]string, 0, len(page))
		for _, r := range page {
			name := byID[r.IndexerID]
			if name != "" {
				if _, err := p.lookups.Indexer(ctx, name); err != nil {
					return count, fresh, err
				}
			}
			releaseID, err := p.lookups.Release(ctx, r.Str("grabTitle"))
			if err != nil {
				return count, fresh, err
			}
			elapsed, _ := r.Int("elapsedTime")

			ref := strconv.FormatInt(r.ID, 10)
			refs = append(refs, ref)
			rows = append(rows, model.IndexerGrab{
				ServiceName: p.serviceName,
				SourceRef:   ref,
				At:          r.Date.UTC(),
				IndexerName: strPtr(name),
				ReleaseID:   releaseID,
				Source:      strPtr(r.Str("source")),
				SourceHost:  strPtr(r.Str("host")),
				GrabMethod:  strPtr(r.Str("grabMethod")),
				ElapsedMs:   intPtr(elapsed),
				URL:         strPtr(r.Str("url")),
			})
		}

		fresh, err = p.countNewGrabs(ctx, refs)
		if err != nil {
			return count, fresh, err
		}

		if err := dao.Upsert(ctx, p.dao, rows,
			[]string{"service_name", "source_ref"},
			[]string{"indexer_name", "release_id"},
		); err != nil {
			return count, fresh, fmt.Errorf("prowlarr: grabs: %w", err)
		}
		count = len(rows)
	}
	return count, fresh, nil
}

// countNew reports how many of a page's records are not already stored.
//
// The stop condition for an incremental walk, and it has to be asked BEFORE the
// upsert: afterwards every row exists, so there is nothing left to tell a page
// of new records from one that was already complete. Getting that backwards is
// how this walk quietly became a full re-read of the entire history on every
// scheduled pass.
//
// One query per page rather than one per record — the page is the unit the walk
// makes its decision on.
//
// Split per table rather than taking a table name. The two tables are different
// models with different generated accessors, and passing "searches" as a string
// is precisely the drift the generated API exists to prevent: it compiles
// whatever is typed and finds out at runtime.
func (p *Prowlarr) countNewSearches(ctx context.Context, refs []string) (int, error) {
	if len(refs) == 0 {
		return 0, nil
	}
	s := p.dao.Q().Search
	var existing []string
	err := s.WithContext(ctx).
		Where(s.ServiceName.Eq(p.serviceName), s.SourceRef.In(refs...)).
		Pluck(s.SourceRef, &existing)
	if err != nil {
		return 0, fmt.Errorf("prowlarr: count new searches: %w", err)
	}
	return len(refs) - len(existing), nil
}

// countNewGrabs is countNewSearches over indexer_grabs.
func (p *Prowlarr) countNewGrabs(ctx context.Context, refs []string) (int, error) {
	if len(refs) == 0 {
		return 0, nil
	}
	g := p.dao.Q().IndexerGrab
	var existing []string
	err := g.WithContext(ctx).
		Where(g.ServiceName.Eq(p.serviceName), g.SourceRef.In(refs...)).
		Pluck(g.SourceRef, &existing)
	if err != nil {
		return 0, fmt.Errorf("prowlarr: count new grabs: %w", err)
	}
	return len(refs) - len(existing), nil
}

// syncViaSidecar walks searches and grabs as one cursored stream.
//
// The sidecar serves both event types in sequence behind a single cursor, so a
// page can hold either kind and is split before it is written. RSS polls never
// arrive: managarr records none of them, and the sidecar drops that stage
// rather than page 44,000 rows across the VLAN for a consumer that discards
// every one.
func (p *Prowlarr) syncViaSidecar(ctx context.Context, byID map[int]string) (searches, grabs int, err error) {
	_, err = p.walk.Run(ctx, func(ctx context.Context, page []sidecar.Event) error {
		queries, grabbed, err := splitProwlarrPage(page)
		if err != nil {
			return err
		}

		if len(queries) > 0 {
			n, fresh, err := p.writeSearchPage(ctx, queries, byID)
			searches += n
			if err != nil {
				return err
			}
			p.logger.InfoContext(ctx, "prowlarr progress",
				"kind", "searches", "records", searches, "new_this_page", fresh)
		}
		if len(grabbed) > 0 {
			n, fresh, err := p.writeGrabPage(ctx, grabbed, byID)
			grabs += n
			if err != nil {
				return err
			}
			p.logger.InfoContext(ctx, "prowlarr progress",
				"kind", "grabs", "records", grabs, "new_this_page", fresh)
		}
		return nil
	})
	return searches, grabs, err
}

// splitProwlarrPage sorts one page into the two record sets that are written.
//
// A kind this pass does not handle is skipped rather than refused: the sidecar
// is free to grow one, and a consumer that died on the first unrecognised kind
// would make every addition a breaking change.
func splitProwlarrPage(page []sidecar.Event) (queries, grabbed []prowlarr.HistoryRecord, err error) {
	for _, e := range page {
		switch e.Kind {
		case prowlarrsvc.KindQuery, prowlarrsvc.KindGrabbed:
		default:
			continue
		}
		r, err := sidecar.DecodePayload[prowlarr.HistoryRecord](e)
		if err != nil {
			return nil, nil, fmt.Errorf("prowlarr: decode %s event: %w", e.Kind, err)
		}
		if e.Kind == prowlarrsvc.KindQuery {
			queries = append(queries, r)
		} else {
			grabbed = append(grabbed, r)
		}
	}
	return queries, grabbed, nil
}
