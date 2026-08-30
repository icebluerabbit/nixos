package store

import (
	"context"
	"fmt"
	"strings"
)

// Count1 is a one-key grouped aggregate (Key → N). N is a count or a sum depending
// on the query.
type Count1 struct {
	Key string
	N   int64
}

// Count2 is a two-key grouped aggregate.
type Count2 struct {
	A, B string
	N    int64
}

func (s *Store) scan1(ctx context.Context, query string, args ...any) ([]Count1, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Count1
	for rows.Next() {
		var c Count1
		if err := rows.Scan(&c.Key, &c.N); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) scan2(ctx context.Context, query string, args ...any) ([]Count2, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Count2
	for rows.Next() {
		var c Count2
		if err := rows.Scan(&c.A, &c.B, &c.N); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// AnnouncesTotal is the total announce count.
func (s *Store) AnnouncesTotal(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM announces`).Scan(&n)
	return n, err
}

// AnnouncesByAccept groups announces by (accept_decision, accept_reason).
func (s *Store) AnnouncesByAccept(ctx context.Context) ([]Count2, error) {
	return s.scan2(ctx, `SELECT accept_decision, accept_reason, COUNT(*) FROM announces GROUP BY accept_decision, accept_reason`)
}

// AnnouncesByGrab groups L2-evaluated announces by (grab_decision, grab_reason).
func (s *Store) AnnouncesByGrab(ctx context.Context) ([]Count2, error) {
	return s.scan2(ctx, `SELECT grab_decision, grab_reason, COUNT(*) FROM announces WHERE grab_decision IS NOT NULL GROUP BY grab_decision, grab_reason`)
}

// AnnouncesBy groups announces by a whitelisted column.
func (s *Store) AnnouncesBy(ctx context.Context, col string) ([]Count1, error) {
	switch col {
	case "media_type", "fiction", "vip", "freeleech":
	default:
		return nil, fmt.Errorf("store: bad group column %q", col)
	}
	// NULL is excluded, not grouped: scan1 reads the key into a string, so one NULL
	// row would fail the whole query. `fiction` is NULL on every pre-0006 announce.
	return s.scan1(ctx, `SELECT `+col+`, COUNT(*) FROM announces WHERE `+col+` IS NOT NULL GROUP BY `+col)
}

// AnnounceCategories counts announces by category from the normalized child table. A
// release carries several, so these counts sum above the announce total.
func (s *Store) AnnounceCategories(ctx context.Context) ([]Count1, error) {
	return s.scan1(ctx, `SELECT category, COUNT(*) FROM announce_categories GROUP BY category`)
}

// AnnounceFiletypes counts announces by filetype from the normalized child table. A
// release can announce several formats, so these counts sum above the announce total.
func (s *Store) AnnounceFiletypes(ctx context.Context) ([]Count1, error) {
	return s.scan1(ctx, `SELECT filetype, COUNT(*) FROM announce_filetypes GROUP BY filetype`)
}

// AnnounceLanguages counts announces by language from the normalized child table.
func (s *Store) AnnounceLanguages(ctx context.Context) ([]Count1, error) {
	return s.scan1(ctx, `SELECT language, COUNT(*) FROM announce_languages GROUP BY language`)
}

// AnnounceBytesByMediaType sums announced size in bytes per MAM media type.
func (s *Store) AnnounceBytesByMediaType(ctx context.Context) ([]Count1, error) {
	return s.scan1(ctx, `SELECT media_type, COALESCE(SUM(size), 0) FROM announces GROUP BY media_type`)
}

// SizeHist is a per-media-type size histogram: total count, total bytes, and
// cumulative counts per bucket upper bound (le → count with size <= le).
type SizeHist struct {
	MediaType string
	Count     uint64
	Sum       float64
	Buckets   map[float64]uint64
}

// AnnounceSizeHistogram computes the announce-size histogram per media type for the
// given bucket bounds (a Prometheus ConstHistogram source).
func (s *Store) AnnounceSizeHistogram(ctx context.Context, buckets []float64) ([]SizeHist, error) {
	var sb strings.Builder
	sb.WriteString("SELECT media_type, COUNT(*), COALESCE(SUM(size), 0)")
	for _, b := range buckets {
		fmt.Fprintf(&sb, ", SUM(CASE WHEN size <= %d THEN 1 ELSE 0 END)", int64(b))
	}
	sb.WriteString(" FROM announces GROUP BY media_type")

	rows, err := s.db.QueryContext(ctx, sb.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SizeHist
	for rows.Next() {
		h := SizeHist{Buckets: make(map[float64]uint64, len(buckets))}
		bv := make([]uint64, len(buckets))
		dest := []any{&h.MediaType, &h.Count, &h.Sum}
		for i := range bv {
			dest = append(dest, &bv[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		for i, b := range buckets {
			h.Buckets[b] = bv[i]
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// GrabsByResult groups grab actions by result.
func (s *Store) GrabsByResult(ctx context.Context) ([]Count1, error) {
	return s.scan1(ctx, `SELECT result, COUNT(*) FROM grabs GROUP BY result`)
}

// GrabsBySource groups grab actions by origin (announce|torznab).
func (s *Store) GrabsBySource(ctx context.Context) ([]Count1, error) {
	return s.scan1(ctx, `SELECT source, COUNT(*) FROM grabs GROUP BY source`)
}

// GrabBytesTotal sums the size of successfully-added grabs.
func (s *Store) GrabBytesTotal(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(size), 0) FROM grabs WHERE result IN ('added', 'added_wedge')`).Scan(&n)
	return n, err
}

// SpendsByType groups outgoing spends by type (wedge|points).
func (s *Store) SpendsByType(ctx context.Context) ([]Count1, error) {
	return s.scan1(ctx, `SELECT type, COUNT(*) FROM spends GROUP BY type`)
}

// SpendSumByType sums spent amount by type (points spent; wedges count as amount 0/1).
func (s *Store) SpendSumByType(ctx context.Context) ([]Count1, error) {
	return s.scan1(ctx, `SELECT type, COALESCE(SUM(amount), 0) FROM spends GROUP BY type`)
}

// BonusSumByType sums received-bonus amount by type (signed; negative = MAM spend).
func (s *Store) BonusSumByType(ctx context.Context) ([]Count1, error) {
	return s.scan1(ctx, `SELECT type, COALESCE(SUM(amount), 0) FROM bonuses GROUP BY type`)
}

// WedgeSavedBytes sums the size of wedge-grabbed torrents (ratio bytes saved).
func (s *Store) WedgeSavedBytes(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(size), 0) FROM grabs WHERE result = 'added_wedge'`).Scan(&n)
	return n, err
}

// SkippedWedgeBytes sums the size of announces the strict proactive wedge rule
// skipped — ratio bytes not spent because no wedge was available. Read alongside
// WedgeSavedBytes: that one is ratio avoided by spending a wedge, this one is ratio
// avoided by declining the grab. Sourced from announces, not grabs, because a skip
// never produces a grab row.
func (s *Store) SkippedWedgeBytes(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(size), 0) FROM announces WHERE grab_decision = 'skip' AND grab_reason = 'wedge_unavailable'`).Scan(&n)
	return n, err
}

// BonusesByType groups the received bonus ledger by type.
func (s *Store) BonusesByType(ctx context.Context) ([]Count1, error) {
	return s.scan1(ctx, `SELECT type, COUNT(*) FROM bonuses GROUP BY type`)
}

// BonusTotal is the total mirrored bonus row count.
func (s *Store) BonusTotal(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bonuses`).Scan(&n)
	return n, err
}

// SpendsByReason groups our outgoing spends by reason.
func (s *Store) SpendsByReason(ctx context.Context) ([]Count1, error) {
	return s.scan1(ctx, `SELECT reason, COUNT(*) FROM spends GROUP BY reason`)
}
