package ingest

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gen/field"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// autobrrPush is the body the shared managarr filter action posts.
//
// Every field is a string because autobrr renders the action body from a Go
// template — Size, Seeders and Leechers included.
type autobrrPush struct {
	Name     string `json:"name"`
	Infohash string `json:"infohash"`
	Indexer  string `json:"indexer"`
	Filter   string `json:"filter"`
	Size     string `json:"size"`
	Category string `json:"category"`
	URL      string `json:"url"`
	// TorrentID is the TRACKER's id for the torrent, from the .TorrentID
	// macro. The key this handler updates the announce through.
	TorrentID string `json:"torrent_id"`
	Seeders   string `json:"seeders"`
	Leechers  string `json:"leechers"`
}

// autobrr records a push from autobrr's filter action.
//
// This handler exists because autobrr's two interfaces do not overlap. Its API
// serialises the release with the infohash omitted — the field is json:"-" in
// its own domain model — and never reports seeders or leechers at all. Its
// templates expose all three but cannot emit the numeric release id the API
// keys on. So each side knows something the other cannot express.
//
// torrent_id is the single field present in both, which is what makes this
// joinable: (indexer, torrent_id) identifies the announce from either side.
//
// Two writes, and they are independent on purpose. The announce is updated when
// it already exists — the hourly pass may not have run yet — while the
// release-to-torrent link is recorded regardless, because that one can be
// carried back later and the seeder counts cannot.
func (i *Ingest) autobrr(ctx context.Context, body []byte) error {
	var push autobrrPush
	if err := decode(body, &push); err != nil {
		return fmt.Errorf("autobrr: decode: %w", err)
	}
	if push.Infohash == "" || push.Name == "" {
		// A push with neither is a template that failed to render. Recording
		// it would create a nameless release with no torrent behind it.
		return nil
	}

	releaseID, err := i.lookups.Release(ctx, push.Name)
	if err != nil {
		return err
	}
	hash, err := i.lookups.Torrent(ctx, push.Infohash)
	if err != nil {
		return err
	}
	if hash == nil || releaseID == nil {
		return nil
	}
	indexer, err := i.lookups.Indexer(ctx, push.Indexer)
	if err != nil {
		return err
	}

	if err := i.updateOffer(ctx, push, indexer, *hash); err != nil {
		return err
	}

	// release_id only, and only when unset. The torrent's other columns belong
	// to whoever read the .torrent itself; this payload knows none of them.
	return dao.UpsertGuarded(ctx, i.dao, []model.Torrent{{
		Infohash:       *hash,
		ReleaseID:      releaseID,
		TotalSizeBytes: int64Ptr(parseBytes(push.Size)),
	}}, []string{"infohash"},
		"torrents.release_id IS NULL",
		[]string{"release_id", "total_size_bytes"})
}

// updateOffer writes what only the webhook knows onto the announce.
//
// Matched on (indexer, tracker_torrent_id) and scoped to announces. No row is
// created when none matches: an offer's identity is autobrr's release id, which
// this payload does not carry, so inventing one would mean the hourly pass
// later writes a second row for the same announce.
//
// The infohash is set unconditionally rather than only when NULL. autobrr is
// the authority for it — no other source has one for an announce — so a later
// push for the same tracker torrent is a correction, not a competing claim.
func (i *Ingest) updateOffer(ctx context.Context, push autobrrPush, indexer *string, hash string) error {
	if indexer == nil || push.TorrentID == "" {
		// Without both halves of the key there is nothing to match. Common
		// enough to be unremarkable: plenty of indexer definitions parse no
		// torrent id.
		return nil
	}

	o := i.dao.Q().Offer
	// Built as a list because seeders and leechers are set only when the push
	// parsed them — an unparsed count must leave the stored one alone rather
	// than write a zero over it.
	updates := []field.AssignExpr{o.Infohash.Value(hash)}
	if n, ok := parseCount(push.Seeders); ok {
		updates = append(updates, o.Seeders.Value(n))
	}
	if n, ok := parseCount(push.Leechers); ok {
		updates = append(updates, o.Leechers.Value(n))
	}

	res, err := o.WithContext(ctx).
		Where(
			o.IndexerName.Eq(*indexer),
			o.Kind.Eq(model.OfferAnnounce),
			o.TrackerTorrentID.Eq(push.TorrentID),
		).
		UpdateSimple(updates...)
	if err != nil {
		return fmt.Errorf("autobrr: update offer %s/%s: %w",
			*indexer, push.TorrentID, err)
	}
	if res.RowsAffected == 0 {
		i.logger.DebugContext(ctx, "no announce yet for push",
			"indexer", *indexer, "torrent_id", push.TorrentID)
	}
	return nil
}

// parseCount reads a swarm count the template rendered as text.
//
// Distinguishes absent from zero: a release with no seeders is a real and
// interesting measurement, while an unrendered macro is not a measurement at
// all, and storing the second as the first would invent dead releases.
func parseCount(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}
