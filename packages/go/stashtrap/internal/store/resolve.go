package store

import (
	"context"
	"strings"
)

// AnnounceNames returns every recorded release name, oldest first. It exists
// for offline analysis of the corpus against a matching change.
func (s *Store) AnnounceNames(ctx context.Context) ([]string, error) {
	return s.ids(ctx, `SELECT name FROM announces ORDER BY id`)
}

// placeholders builds an IN list of n bind parameters.
//
// The only thing interpolated into any query in this package: a count decides
// how many "?" to emit, and every value behind them is bound. Nothing derived
// from a release name or a stash-box id ever reaches the SQL text.
func placeholders(n int) string {
	if n == 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
