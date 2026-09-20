// Package store owns the database connection and the schema.
//
// The schema is the Models list in models.go rather than a migration
// directory: AutoMigrate walks it, so a model not named there does not exist
// as far as the database is concerned. That check is what replaces remembering
// to hand-write a migration for every new table.
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// slogLogger adapts GORM's logger onto slog.
//
// GORM's default writes to stdout with its own formatting, which would break
// the JSON line format the Vector sidecar parses. Slow queries and errors are
// worth seeing; every statement is not.
type slogLogger struct {
	log           *slog.Logger
	slowThreshold time.Duration
}

func (l slogLogger) LogMode(gormlogger.LogLevel) gormlogger.Interface { return l }

func (l slogLogger) Info(ctx context.Context, msg string, args ...any) {
	l.log.InfoContext(ctx, fmt.Sprintf(msg, args...))
}

func (l slogLogger) Warn(ctx context.Context, msg string, args ...any) {
	l.log.WarnContext(ctx, fmt.Sprintf(msg, args...))
}

func (l slogLogger) Error(ctx context.Context, msg string, args ...any) {
	l.log.ErrorContext(ctx, fmt.Sprintf(msg, args...))
}

// Trace reports a statement, but only when it failed or was slow.
func (l slogLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		// ErrRecordNotFound is excluded deliberately: a miss is the normal
		// outcome of most lookups here and logging it would bury real errors.
		sql, rows := fc()
		l.log.ErrorContext(ctx, "query failed",
			"err", err, "elapsed", elapsed.String(), "rows", rows, "sql", sql)

	case l.slowThreshold > 0 && elapsed > l.slowThreshold:
		sql, rows := fc()
		l.log.WarnContext(ctx, "slow query",
			"elapsed", elapsed.String(), "rows", rows, "sql", sql)
	}
}

// OpenGorm connects and returns a GORM handle.
//
// PrepareStmt is on: every writer here is the same handful of upserts executed
// tens of thousands of times per pass, which is exactly the case a statement
// cache exists for.
func OpenGorm(ctx context.Context, dsn string, logger *slog.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: slogLogger{
			log: logger,
			// A reconcile statement that takes longer than this is a missing
			// index, not a busy database.
			slowThreshold: 500 * time.Millisecond,
		},
		PrepareStmt: true,
		// The model owns its own timestamps — FirstSeenAt, AnnouncedAt, At —
		// and their meanings differ per table. GORM's automatic created_at /
		// updated_at would add columns nothing reads.
		SkipDefaultTransaction: true,
		NowFunc:                func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("store: pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(4)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return db, nil
}

// AutoMigrate reconciles the live schema against Models.
//
// Additive only, which is what makes it safe to run on every start: GORM adds
// missing tables, columns, indexes and constraints, and never drops a column or
// narrows a type. A column this model no longer declares is left in place
// rather than taken away with the data in it.
// One model at a time rather than one variadic call: GORM reports a failure
// from the batch form without naming the model that caused it, and "relation
// does not exist" across fifty tables is not a diagnosis. Migrating
// individually costs nothing and makes the first failure self-describing.
func AutoMigrate(db *gorm.DB, logger *slog.Logger) error {
	started := time.Now()
	for _, m := range Models {
		if err := db.AutoMigrate(m); err != nil {
			return fmt.Errorf("store: automigrate %T: %w", m, err)
		}
	}
	logger.Info("schema reconciled",
		"models", len(Models), "took", time.Since(started).String())
	return nil
}
