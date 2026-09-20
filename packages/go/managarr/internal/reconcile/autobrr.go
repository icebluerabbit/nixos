package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"homelab/clients/autobrr"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
	"homelab/sidecar"
	"homelab/sidecar/autobrrsvc"
)

// Autobrr replays autobrr's own record into the store.
//
// The only source of announcements and of the verdicts taken on them. Every
// other component sees a release once somebody accepted it; autobrr keeps the
// refusals too, and those are most of them.
type Autobrr struct {
	client *autobrr.Client
	// walk reads the release history through the sidecar with a stored
	// cursor. Nil walks autobrr directly, newest-first.
	walk        *SidecarWalk
	dao         *dao.DAO
	lookups     *dao.Lookups
	logger      *slog.Logger
	serviceName string
	pageSize    int
	// full walks the whole history rather than stopping at the first page
	// holding nothing new.
	full bool
}

// NewAutobrr builds the pass.
func NewAutobrr(c *autobrr.Client, walk *SidecarWalk, d *dao.DAO, l *dao.Lookups, logger *slog.Logger, serviceName string, pageSize int, full bool) *Autobrr {
	if pageSize <= 0 {
		pageSize = 1000
	}
	return &Autobrr{
		client: c, walk: walk, dao: d, lookups: l, logger: logger,
		serviceName: serviceName, pageSize: pageSize, full: full,
	}
}

// AutobrrStats reports what a run did.
type AutobrrStats struct {
	Indexers  int
	Filters   int
	Networks  int
	Channels  int
	Offers    int
	Decisions int
	Took      time.Duration
}

// Run executes one pass.
//
// Indexers and filters first: an offer references both, and a decision
// references a filter.
func (a *Autobrr) Run(ctx context.Context) (AutobrrStats, error) {
	var st AutobrrStats
	started := time.Now()
	a.lookups.Reset()

	if _, err := a.lookups.Service(ctx, a.serviceName, "autobrr"); err != nil {
		return st, err
	}

	n, err := a.syncIndexers(ctx)
	if err != nil {
		return st, err
	}
	st.Indexers = n

	filters, err := a.syncFilters(ctx)
	if err != nil {
		return st, err
	}
	st.Filters = len(filters)

	nets, chans, err := a.syncIRC(ctx)
	if err != nil {
		return st, err
	}
	st.Networks, st.Channels = nets, chans

	offers, decisions, err := a.syncReleases(ctx)
	if err != nil {
		return st, err
	}
	st.Offers, st.Decisions = offers, decisions

	st.Took = time.Since(started)
	return st, nil
}

// syncIndexers records every indexer autobrr knows.
//
// autobrr owns identifier, implementation and enabled. Protocol and privacy
// belong to Prowlarr's view and are not written here, so the two passes cannot
// fight over one row.
func (a *Autobrr) syncIndexers(ctx context.Context) (int, error) {
	list, err := a.client.Indexers(ctx)
	if err != nil {
		return 0, err
	}
	rows := make([]model.Indexer, 0, len(list))
	for _, ix := range list {
		rows = append(rows, model.Indexer{
			Name:           ix.Name,
			Identifier:     strPtr(ix.Identifier),
			Implementation: strPtr(ix.Implementation),
			Enabled:        boolPtr(ix.Enabled),
		})
	}
	if err := dao.Upsert(ctx, a.dao, rows, []string{"name"},
		[]string{"identifier", "implementation", "enabled"}); err != nil {
		return 0, fmt.Errorf("autobrr: indexers: %w", err)
	}
	return len(rows), nil
}

// syncFilters records filters and their indexer links.
func (a *Autobrr) syncFilters(ctx context.Context) (map[string]struct{}, error) {
	list, err := a.client.Filters(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]model.Filter, 0, len(list))
	var links []model.FilterIndexer
	names := make(map[string]struct{}, len(list))

	for _, f := range list {
		rows = append(rows, model.Filter{
			Name:         f.Name,
			Enabled:      boolPtr(f.Enabled),
			Priority:     intPtr(f.Priority),
			ActionsCount: intPtr(f.ActionsCount),
			CreatedAt:    timePtr(f.CreatedAt),
			UpdatedAt:    timePtr(f.UpdatedAt),
		})
		names[f.Name] = struct{}{}
		// Many to many: a filter spans trackers and a tracker serves many
		// filters. No column on either side can hold this.
		for _, ix := range f.Indexers {
			if _, err := a.lookups.Indexer(ctx, ix.Name); err != nil {
				return nil, err
			}
			links = append(links, model.FilterIndexer{
				FilterName: f.Name, IndexerName: ix.Name,
			})
		}
	}

	if err := dao.Upsert(ctx, a.dao, rows, []string{"name"},
		[]string{"enabled", "priority", "actions_count", "updated_at"}); err != nil {
		return nil, fmt.Errorf("autobrr: filters: %w", err)
	}
	if err := dao.Insert(ctx, a.dao, links,
		[]string{"filter_name", "indexer_name"}); err != nil {
		return nil, fmt.Errorf("autobrr: filter indexers: %w", err)
	}
	return names, nil
}

// syncIRC records networks and the channels autobrr joins.
func (a *Autobrr) syncIRC(ctx context.Context) (networks, channels int, err error) {
	list, err := a.client.Networks(ctx)
	if err != nil {
		return 0, 0, err
	}

	nets := make([]model.IrcNetwork, 0, len(list))
	var chans []model.IrcChannel
	for _, n := range list {
		nets = append(nets, model.IrcNetwork{
			Host:    n.Server,
			Name:    strPtr(n.Name),
			Port:    intPtr(n.Port),
			Nick:    strPtr(n.Nick),
			TLS:     boolPtr(n.TLS),
			Enabled: boolPtr(n.Enabled),
		})
		for _, ch := range n.Channels {
			chans = append(chans, model.IrcChannel{
				NetworkHost: n.Server,
				Name:        ch.Name,
				// autobrr joins a channel without classifying it, and
				// everything it monitors is an announce channel.
				Kind:           model.ChannelAnnounce,
				Enabled:        boolPtr(ch.Enabled),
				Monitoring:     boolPtr(ch.Monitoring),
				LastAnnounceAt: timePtr(ch.LastAnnounce),
			})
		}
	}

	if err := dao.Upsert(ctx, a.dao, nets, []string{"host"},
		[]string{"name", "port", "nick", "tls", "enabled"}); err != nil {
		return 0, 0, fmt.Errorf("autobrr: irc networks: %w", err)
	}
	if err := dao.Upsert(ctx, a.dao, chans, []string{"network_host", "name"},
		[]string{"enabled", "monitoring", "last_announce_at"}); err != nil {
		return 0, 0, fmt.Errorf("autobrr: irc channels: %w", err)
	}
	return len(nets), len(chans), nil
}

// syncReleases walks the release history a page at a time.
//
// A page is written with a fixed number of statements rather than a handful per
// release — see autobrr_page.go for why that mattered enough to restructure.
// The walk resumes from a stored cursor when a sidecar is configured. Without
// one it restarts at the newest release every pass and stops on the first page
// holding nothing new — which re-reads a page every time, and cannot survive
// autobrr's own paging hazard: the list is newest-first and paged by offset, so
// a release arriving mid-walk shifts every later page down by one and a row is
// skipped outright. The sidecar counts from the oldest end instead, where an
// arrival cannot move the position.
func (a *Autobrr) syncReleases(ctx context.Context) (offers, decisions int, err error) {
	if a.walk != nil {
		_, err := a.walk.Run(ctx, func(ctx context.Context, page []sidecar.Event) error {
			releases, err := decodeReleases(page)
			if err != nil {
				return err
			}
			n, d, fresh, err := a.writePage(ctx, releases)
			if err != nil {
				return err
			}
			offers += n
			decisions += d
			a.logger.InfoContext(ctx, "autobrr progress",
				"offers", offers, "decisions", decisions, "new_this_page", fresh)
			return nil
		})
		return offers, decisions, err
	}

	var walkErr error
	err = a.client.Releases(ctx, a.pageSize, func(page []autobrr.Release) bool {
		n, d, fresh, e := a.writePage(ctx, page)
		if e != nil {
			walkErr = e
			return false
		}
		offers += n
		decisions += d

		// Progress, because the walk is the longest thing this service does and
		// a pass that only logs on completion is indistinguishable from a hung
		// one for half an hour.
		a.logger.InfoContext(ctx, "autobrr progress",
			"offers", offers, "decisions", decisions, "new_this_page", fresh)

		return a.full || fresh > 0
	})
	if walkErr != nil {
		return offers, decisions, walkErr
	}
	return offers, decisions, err
}

// writeReleases records what autobrr made of each release NAME.
//
// autobrr is the only service in this stack that parses a release name into its
// parts, and it does it on everything it sees — including the four fifths it
// rejects, which no *arr ever looked at. Every other source records a release as
// an opaque title.
//
// Written with COALESCE rather than a plain upsert. The same title is announced
// repeatedly across trackers and autobrr's parse is not always identical: a
// tracker that pads the name, or an announce the indexer definition did not
// fully match, yields a record with half the fields empty. Those must not blank
// what a cleaner announce of the same release already established.
//
// content_id is deliberately absent: it is resolved from grabs by the resolve
// pass, and writing NULL here on every pass would undo that.
func (a *Autobrr) writeReleases(ctx context.Context, page []autobrr.Release) error {
	if err := dao.UpsertCoalesce(ctx, a.dao, releaseRows(page), []string{"title"},
		[]string{"resolution", "source", "codec", "hdr", "container",
			"release_group", "edition", "proper", "repack", "year", "season",
			"episode", "size_bytes"},
	); err != nil {
		return fmt.Errorf("autobrr: releases: %w", err)
	}
	return nil
}

// filterOutcome maps autobrr's filter verdict.
func filterOutcome(status string) string {
	if status == "FILTER_APPROVED" {
		return model.OutcomeAccepted
	}
	return model.OutcomeRejected
}

// pushOutcome maps an action's verdict.
//
// PUSH_ERROR is errored rather than rejected: the target never judged the
// release, the push itself failed. Treating it as a rejection makes a broken
// integration read as a filtering policy.
func pushOutcome(status string) string {
	switch status {
	case "PUSH_APPROVED":
		return model.OutcomeAccepted
	case "PUSH_REJECTED":
		return model.OutcomeRejected
	case "PUSH_ERROR":
		return model.OutcomeErrored
	default:
		return model.OutcomeErrored
	}
}

// releaseRows converts a page, dropping records with no name.
//
// Title is the natural key and the thing every other table joins through, so a
// record without one cannot be stored at all — it would be a release nothing
// could ever reach again.
func releaseRows(page []autobrr.Release) []model.Release {
	// Deduplicated by title, and merged rather than last-wins.
	//
	// One INSERT ... ON CONFLICT DO UPDATE cannot touch the same row twice —
	// Postgres raises 21000, "command cannot affect row a second time" — and a
	// page routinely holds one title more than once, because the same release
	// announced on two trackers is two autobrr records with one name. That is
	// the cross-seeding case, so it is the normal case, not an edge one.
	//
	// Merged because the duplicates are not identical: autobrr parses each
	// announce separately and one tracker's padded name yields fewer fields
	// than another's. Taking whichever came last would let the worse parse win,
	// which is the same thing UpsertCoalesce prevents across passes.
	order := make([]string, 0, len(page))
	byTitle := make(map[string]*model.Release, len(page))

	for _, r := range page {
		if r.Name == "" {
			continue
		}
		row := releaseRow(r)
		if prev, ok := byTitle[r.Name]; ok {
			mergeRelease(prev, row)
			continue
		}
		byTitle[r.Name] = &row
		order = append(order, r.Name)
	}

	rows := make([]model.Release, 0, len(order))
	for _, title := range order {
		rows = append(rows, *byTitle[title])
	}
	return rows
}

// mergeRelease fills anything the first parse of a title left unset.
//
// Never overwrites: the first row is the one already accumulating, and a later
// duplicate can only add. Equivalent to what UpsertCoalesce does in SQL, applied
// inside the batch where SQL cannot reach.
func mergeRelease(into *model.Release, from model.Release) {
	fillStr(&into.Resolution, from.Resolution)
	fillStr(&into.Source, from.Source)
	fillStr(&into.Codec, from.Codec)
	fillStr(&into.HDR, from.HDR)
	fillStr(&into.Container, from.Container)
	fillStr(&into.ReleaseGroup, from.ReleaseGroup)
	fillStr(&into.Edition, from.Edition)
	fillInt(&into.Year, from.Year)
	fillInt(&into.Season, from.Season)
	fillInt(&into.Episode, from.Episode)

	if into.SizeBytes == nil {
		into.SizeBytes = from.SizeBytes
	}
	// Proper and Repack are never nil — boolPtr always returns a value — so a
	// later announce can only promote them to true. Two records for one title
	// disagreeing means one tracker labelled it and the other did not, and the
	// label is the fact.
	if from.Proper != nil && *from.Proper {
		into.Proper = from.Proper
	}
	if from.Repack != nil && *from.Repack {
		into.Repack = from.Repack
	}
}

// fillStr sets a nil target from a non-nil source.
func fillStr(into **string, from *string) {
	if *into == nil {
		*into = from
	}
}

// fillInt sets a nil target from a non-nil source.
func fillInt(into **int, from *int) {
	if *into == nil {
		*into = from
	}
}

// releaseRow converts one record.
//
// Kept pure and separate from the write so the mapping can be exercised without
// a database: this is the only place in the service where a release name stops
// being an opaque string, and getting a field wrong here is silent.
func releaseRow(r autobrr.Release) model.Release {
	return model.Release{
		Title:      r.Name,
		Resolution: strPtr(r.Resolution),
		Source:     strPtr(r.Source),
		// Both arrive as arrays: one release routinely carries several codecs
		// and several HDR formats — DV and HDR10 on one file is normal.
		Codec:        strPtr(strings.Join(r.Codec, ", ")),
		HDR:          strPtr(strings.Join(r.HDR, ", ")),
		Container:    strPtr(r.Container),
		ReleaseGroup: strPtr(r.Group),
		Edition:      strPtr(strings.Join(r.Edition, ", ")),
		// Not intPtr: proper and repack are the difference between two files of
		// the same quality, so false is a measurement rather than an absence.
		Proper:    boolPtr(r.Proper),
		Repack:    boolPtr(r.Repack),
		Year:      intPtr(r.Year),
		Season:    intPtr(r.Season),
		Episode:   intPtr(r.Episode),
		SizeBytes: int64Ptr(r.Size),
	}
}

// dedupeDecisions keeps one verdict per (offer, decider, stage).
//
// The latest by decision time, because a repeated action on one release is a
// retry — the second attempt is what actually happened. Records with no
// timestamp keep their arrival order, which is autobrr's own.
func dedupeDecisions(rows []model.Decision) []model.Decision {
	type key struct{ decider, stage string }

	seen := make(map[key]int, len(rows))
	out := make([]model.Decision, 0, len(rows))

	for _, row := range rows {
		k := key{row.Decider, row.Stage}
		i, dup := seen[k]
		if !dup {
			seen[k] = len(out)
			out = append(out, row)
			continue
		}
		if newerDecision(out[i], row) {
			out[i] = row
		}
	}
	return out
}

// newerDecision reports whether b supersedes a.
func newerDecision(a, b model.Decision) bool {
	if b.DecidedAt == nil {
		return false
	}
	if a.DecidedAt == nil {
		return true
	}
	return b.DecidedAt.After(*a.DecidedAt)
}

// decodeReleases unwraps a page of sidecar events.
//
// Events of another kind are skipped rather than refused: managarr-autobrr also
// relays filter actions on the same stream, and a consumer that died on the
// first kind it did not recognise would make every addition a breaking change.
func decodeReleases(page []sidecar.Event) ([]autobrr.Release, error) {
	out := make([]autobrr.Release, 0, len(page))
	for _, e := range page {
		if e.Kind != autobrrsvc.KindRelease {
			continue
		}
		r, err := sidecar.DecodePayload[autobrr.Release](e)
		if err != nil {
			return nil, fmt.Errorf("autobrr: decode release: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}
