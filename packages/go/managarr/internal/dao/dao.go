// Package dao owns every write to the database.
//
// Two rules, and they are the whole point of the package existing:
//
//  1. All SQL lives here. Scattered across the reconcile and ingest packages,
//     "is every write idempotent?" could only be answered by reading a dozen
//     files, and the answer was repeatedly no.
//  2. Every write is an upsert whose conflict target names a real unique index
//     declared on the model. The arbiter and the index are the same text in the
//     same repository, so they cannot drift — which is how this schema
//     accumulated duplicate rows four separate times.
//
// GORM's clause.OnConflict is used rather than hand-written ON CONFLICT so the
// target columns are Go values that a compiler and a test can inspect.
package dao

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"gorm.io/gen/field"

	"homelab/managarr/internal/query"
)

// batchSize bounds one multi-row INSERT.
//
// The large passes write six-figure row counts; sending them one statement at a
// time is what made a full autobrr walk take thirteen minutes. Postgres caps a
// statement at 65535 parameters, and these models run to ~30 columns, so 500
// rows stays well inside that with room for wider tables.
const batchSize = 500

// DAO is the write surface.
type DAO struct {
	db  *gorm.DB
	q   *query.Query
	log *slog.Logger
}

// New builds a DAO over an open GORM handle.
func New(db *gorm.DB, log *slog.Logger) *DAO {
	return &DAO{db: db, q: query.Use(db), log: log}
}

// Q is the generated, typed query API — the read surface.
//
// Reads are not routed through this package's own functions: they are ad hoc
// and vary per caller. What they DID share was column names written as string
// literals, so a renamed field compiled and failed at runtime against whatever
// the database happened to have. That is not hypothetical here; a wrong route
// left subtitle_files empty for weeks and nothing in the build objected.
//
// Handed out from the DAO because every pass already holds one. Threading a
// second handle through fifteen constructors would have said the same thing at
// more cost.
func (d *DAO) Q() *query.Query { return d.q }

// DB exposes the raw handle.
//
// Still here for what the generated API cannot express: the aggregates in
// internal/stats, and this package's own generic upserts, which are
// clause.OnConflict over an arbitrary T and have no per-model equivalent.
// Prefer Q for anything that names a column.
func (d *DAO) DB() *gorm.DB { return d.db }

// Tx runs fn inside a transaction.
//
// Used by the ingest path so one webhook delivery — which can carry two dozen
// records — either lands whole or not at all. A partially applied batch that is
// then redelivered is how a transfer ended up with a state from the middle of
// its own history.
func (d *DAO) Tx(ctx context.Context, fn func(*DAO) error) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Q is rebuilt on tx, not carried over from d. A query handle holds the
		// connection it was built with, so reusing the outer one would run the
		// transaction's reads OUTSIDE the transaction — seeing pre-transaction
		// state and escaping the rollback.
		return fn(&DAO{db: tx, q: query.Use(tx), log: d.log})
	})
}

// Assign renders a nullable value as a typed column assignment.
//
// The generated Value(T) takes a concrete value and so cannot express absence,
// while every nullable column in this model is a pointer precisely because
// absence is meaningful — a transfer with no finished_at is running, not
// finished at the zero time. Without this, writing NULL back means dropping to
// a map[string]any and losing the compile-time column check that is the whole
// reason the query API is generated.
func Assign[T any](f interface {
	Value(T) field.AssignExpr
	Null() field.AssignExpr
}, v *T) field.AssignExpr {
	if v == nil {
		return f.Null()
	}
	return f.Value(*v)
}

// cols turns column names into a conflict target.
func cols(names ...string) []clause.Column {
	out := make([]clause.Column, len(names))
	for i, n := range names {
		out[i] = clause.Column{Name: n}
	}
	return out
}

// Upsert writes rows, updating the named columns on conflict.
//
// conflictCols must name a unique index that exists. GORM will happily build
// an ON CONFLICT against columns with no index behind them, and Postgres then
// raises "no unique or exclusion constraint matching" — which is a loud failure
// and therefore fine. The silent failure this prevents is the opposite one: an
// arbiter that matches a DIFFERENT index than the one actually violated, which
// Postgres reports as a plain unique_violation that no ON CONFLICT catches.
//
// updateCols lists what a conflicting row takes from the incoming one. Columns
// left out keep their stored value, which is how a snapshot avoids clobbering a
// field only the webhook path knows.
func Upsert[T any](ctx context.Context, d *DAO, rows []T, conflictCols, updateCols []string) error {
	if len(rows) == 0 {
		return nil
	}
	rows, err := dedupe(d, rows, conflictCols)
	if err != nil {
		return err
	}
	onConflict := clause.OnConflict{
		Columns:   cols(conflictCols...),
		DoUpdates: clause.AssignmentColumns(updateCols),
	}
	if len(updateCols) == 0 {
		onConflict.DoNothing = true
	}
	return d.db.WithContext(ctx).
		Clauses(onConflict).
		CreateInBatches(rows, batchSize).Error
}

// UpsertGuarded writes rows, updating a conflicting row only when guard holds.
//
// The guard becomes the WHERE of the DO UPDATE, so a row that fails it is left
// exactly as stored. This is how a webhook fills a column the snapshot has not
// learned yet without being able to overwrite one it already has — "set it if
// it is null" as a property of the statement rather than a read-then-write that
// two concurrent handlers could interleave.
//
// The guard must qualify its column with the table name: Postgres exposes the
// stored row under the table name and the incoming one as EXCLUDED, and an
// unqualified name in a DO UPDATE is ambiguous.
func UpsertGuarded[T any](ctx context.Context, d *DAO, rows []T, conflictCols []string, guard string, updateCols []string) error {
	if len(rows) == 0 {
		return nil
	}
	rows, err := dedupe(d, rows, conflictCols)
	if err != nil {
		return err
	}
	return d.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   cols(conflictCols...),
			DoUpdates: clause.AssignmentColumns(updateCols),
			Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: guard}}},
		}).
		CreateInBatches(rows, batchSize).Error
}

// UpsertCoalesce writes rows, keeping the stored value wherever the incoming
// one is NULL.
//
// The difference from Upsert is per-VALUE rather than per-column. Upsert decides
// once, at the call site, which columns a later write may replace; this decides
// per row, so a record that carries a field overwrites and a record that does
// not leaves what is there.
//
// That distinction matters wherever one source writes the same row repeatedly
// with varying completeness. A release title that autobrr parsed cleanly on
// Tuesday and failed to parse on Wednesday would otherwise have its resolution,
// codec and group blanked by the second announce — the newer row is not the
// better one, it is just the later one.
//
// Rendered as `col = COALESCE(EXCLUDED.col, table.col)`. The table name is
// resolved from the model rather than passed in, so it cannot drift from the
// struct the rows belong to.
func UpsertCoalesce[T any](ctx context.Context, d *DAO, rows []T, conflictCols, updateCols []string) error {
	if len(rows) == 0 {
		return nil
	}
	rows, err := dedupe(d, rows, conflictCols)
	if err != nil {
		return err
	}

	stmt := &gorm.Statement{DB: d.db}
	if err = stmt.Parse(new(T)); err != nil {
		return fmt.Errorf("dao: parse %T: %w", *new(T), err)
	}
	table := stmt.Schema.Table

	assignments := make([]clause.Assignment, 0, len(updateCols))
	for _, c := range updateCols {
		assignments = append(assignments, clause.Assignment{
			Column: clause.Column{Name: c},
			Value: gorm.Expr(
				"COALESCE(EXCLUDED." + c + ", " + table + "." + c + ")"),
		})
	}

	return d.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   cols(conflictCols...),
			DoUpdates: assignments,
		}).
		CreateInBatches(rows, batchSize).Error
}

// UpsertCounter writes rows, ADDING to a counter column instead of replacing it.
//
// For tables that count occurrences of a repeating event. Every other writer
// here records a state — the latest value of something — and replacing is
// correct for those. A counter is the opposite: each write is one more
// occurrence, and assigning would make an album that has failed five times
// indistinguishable from one that failed once, which is the only question such
// a table exists to answer.
//
// Rendered as `counter = COALESCE(table.counter, 0) + EXCLUDED.counter`, so a
// NULL starting value counts from zero rather than staying NULL.
func UpsertCounter[T any](ctx context.Context, d *DAO, rows []T, conflictCols []string, counterCol string, updateCols []string) error {
	if len(rows) == 0 {
		return nil
	}
	rows, err := dedupe(d, rows, conflictCols)
	if err != nil {
		return err
	}

	stmt := &gorm.Statement{DB: d.db}
	if err = stmt.Parse(new(T)); err != nil {
		return fmt.Errorf("dao: parse %T: %w", *new(T), err)
	}
	table := stmt.Schema.Table

	assignments := make([]clause.Assignment, 0, len(updateCols)+1)
	assignments = append(assignments, clause.Assignment{
		Column: clause.Column{Name: counterCol},
		Value: gorm.Expr("COALESCE(" + table + "." + counterCol +
			", 0) + EXCLUDED." + counterCol),
	})
	for _, c := range updateCols {
		assignments = append(assignments, clause.Assignment{
			Column: clause.Column{Name: c},
			Value:  gorm.Expr("EXCLUDED." + c),
		})
	}

	return d.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   cols(conflictCols...),
			DoUpdates: assignments,
		}).
		CreateInBatches(rows, batchSize).Error
}

// UpsertPartial writes rows against a PARTIAL unique index.
//
// A partial index only arbitrates when the ON CONFLICT clause repeats its
// predicate. Without it Postgres cannot tell which index was meant and fails
// with "no unique or exclusion constraint matching the ON CONFLICT
// specification" — loudly, which is the good case. The bad case is naming a
// total index that exists while a DIFFERENT partial one is what actually gets
// violated: that surfaces as a bare unique_violation which ON CONFLICT does not
// catch, and it took down a whole reconcile chain here.
//
// where must be the index predicate verbatim, e.g. "autobrr_id IS NOT NULL".
func UpsertPartial[T any](ctx context.Context, d *DAO, rows []T, conflictCols []string, where string, updateCols []string) error {
	if len(rows) == 0 {
		return nil
	}
	rows, err := dedupe(d, rows, conflictCols)
	if err != nil {
		return err
	}
	onConflict := clause.OnConflict{
		Columns:     cols(conflictCols...),
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: where}}},
		DoUpdates:   clause.AssignmentColumns(updateCols),
	}
	if len(updateCols) == 0 {
		onConflict.DoNothing = true
	}
	return d.db.WithContext(ctx).
		Clauses(onConflict).
		CreateInBatches(rows, batchSize).Error
}

// Create writes rows with no conflict handling.
//
// For tables whose only key is the surrogate id, where every row is genuinely
// new. Fails loudly on a constraint violation rather than swallowing it, which
// is correct: a violation here means the caller's assumption about uniqueness
// was wrong and silence would hide that.
func Create[T any](ctx context.Context, d *DAO, rows []T) error {
	if len(rows) == 0 {
		return nil
	}
	return d.db.WithContext(ctx).CreateInBatches(rows, batchSize).Error
}

// UpsertAll writes rows, replacing every non-key column on conflict.
//
// For snapshot writers, where the incoming row is by definition the current
// truth and there is no older value worth preserving.
func UpsertAll[T any](ctx context.Context, d *DAO, rows []T, conflictCols []string) error {
	if len(rows) == 0 {
		return nil
	}
	rows, err := dedupe(d, rows, conflictCols)
	if err != nil {
		return err
	}
	return d.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   cols(conflictCols...),
			UpdateAll: true,
		}).
		CreateInBatches(rows, batchSize).Error
}

// Insert writes rows, ignoring any that conflict.
//
// For append-only records where the stored row is already complete and an
// incoming duplicate carries nothing new.
func Insert[T any](ctx context.Context, d *DAO, rows []T, conflictCols []string) error {
	return Upsert(ctx, d, rows, conflictCols, nil)
}

// InsertIgnore writes rows, skipping any that violate ANY unique constraint.
//
// The difference from Insert is the absence of an arbiter. Insert names one
// index; Postgres then enforces every OTHER index normally, so a row colliding
// on a different one raises 23505 that no ON CONFLICT catches — the exact
// failure Upsert's comment warns about, arriving from the other direction.
//
// content_external_ids is the case that motivated this. It carries TWO unique
// constraints: a primary key on (content_id, provider) and a unique index on
// (provider, value). Inserting with the (provider, value) arbiter left the
// primary key unguarded, and one batch holding two values for a single
// content+provider took down the sonarr pass — and with it radarr and lidarr,
// which never got to run.
//
// Also tolerates duplicates WITHIN the batch, which DO UPDATE cannot: Postgres
// refuses to affect a row twice in one statement (21000) only when there is an
// update to apply. DO NOTHING simply keeps the first and skips the rest, so no
// dedupe pass is needed before calling this.
//
// Use it where "record this unless it is already known, by any definition of
// known" is the whole requirement. Where a conflict should UPDATE something,
// the arbiter is load-bearing and Upsert is the right call.
func InsertIgnore[T any](ctx context.Context, d *DAO, rows []T) error {
	if len(rows) == 0 {
		return nil
	}
	return d.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(rows, batchSize).Error
}

// FindOne returns the first row matching where, or ok=false if there is none.
//
// Wraps the ErrRecordNotFound check that was repeatedly written as
// `if err != nil { return 0, nil }` — which reported a connection failure as a
// miss and wrote a permanent null link during every database blip.
func FindOne[T any](ctx context.Context, d *DAO, dest *T, where string, args ...any) (bool, error) {
	err := d.db.WithContext(ctx).Where(where, args...).First(dest).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("dao: find %T: %w", dest, err)
	}
	return true, nil
}

// dedupe removes rows repeating an earlier row's conflict key, keeping the LAST
// of each.
//
// Applied by every writer that issues a DO UPDATE, because Postgres refuses one
// outright: a single INSERT ... ON CONFLICT DO UPDATE may not affect the same
// row twice, and raises 21000, "command cannot affect row a second time". Not a
// corner case — a page of autobrr releases holds one title twice whenever a
// release was announced on two trackers, which is exactly what cross-seeding
// produces, and it took a whole backfill down.
//
// Last wins, because every writer here is a snapshot writer: the later row is
// the more recent observation of the same thing. A caller needing something
// smarter — merging two partial parses, preferring the newer timestamp — does it
// before calling, and this then finds nothing left to drop.
//
// DO NOTHING is unaffected by the Postgres rule, but deduplicating anyway costs
// one pass and keeps every writer behaving the same way.
func dedupe[T any](d *DAO, rows []T, conflictCols []string) ([]T, error) {
	if len(rows) < 2 || len(conflictCols) == 0 {
		return rows, nil
	}

	stmt := &gorm.Statement{DB: d.db}
	if err := stmt.Parse(new(T)); err != nil {
		return nil, fmt.Errorf("dao: parse %T: %w", *new(T), err)
	}

	fields := make([]*schema.Field, 0, len(conflictCols))
	for _, col := range conflictCols {
		f, ok := stmt.Schema.FieldsByDBName[col]
		if !ok {
			// A conflict column the model does not declare. Left for Postgres
			// to reject — which it will — rather than silently not
			// deduplicating on a key that cannot be evaluated here.
			return rows, nil
		}
		fields = append(fields, f)
	}

	ctx := context.Background()
	seen := make(map[string]int, len(rows))
	out := make([]T, 0, len(rows))

	for i := range rows {
		rv := reflect.ValueOf(&rows[i])
		var sb strings.Builder
		for _, f := range fields {
			v, zero := f.ValueOf(ctx, rv)
			if zero {
				sb.WriteString("\x00")
				continue
			}
			fmt.Fprintf(&sb, "%v\x00", v)
		}
		key := sb.String()

		if at, dup := seen[key]; dup {
			out[at] = rows[i]
			continue
		}
		seen[key] = len(out)
		out = append(out, rows[i])
	}
	return out, nil
}
