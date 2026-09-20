package store

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"gorm.io/gorm"
)

// testDSN names a scratch database to run schema and write tests against.
//
// Empty skips, so `nix build` stays hermetic and CI needs no service. The
// homelab already runs Postgres; pointing this at a throwaway database there is
// the whole setup, with no testcontainers dependency.
//
//	MANAGARR_TEST_DSN=postgres://managarr@10.10.140.3:5432/managarr_test?sslmode=disable go test ./...
const testDSNEnv = "MANAGARR_TEST_DSN"

// openTest returns a migrated scratch database, or skips.
func openTest(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv(testDSNEnv)
	if dsn == "" {
		t.Skipf("%s not set; skipping database test", testDSNEnv)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	db, err := OpenGorm(context.Background(), dsn, logger)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := AutoMigrate(db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestAutoMigrateIsIdempotent: a second run must be a no-op rather than an
// error. AutoMigrate runs on every service start, so this is the common path.
func TestAutoMigrateIsIdempotent(t *testing.T) {
	db := openTest(t)
	logger := slog.New(slog.DiscardHandler)

	for i := range 2 {
		if err := AutoMigrate(db, logger); err != nil {
			t.Fatalf("migrate run %d: %v", i+2, err)
		}
	}
}

// TestEveryModelHasATable checks that the Models list and the structs agree.
//
// A model that AutoMigrate skipped — typically because it was added to the
// package and not to Models — would otherwise be discovered as a runtime error
// on first write.
func TestEveryModelHasATable(t *testing.T) {
	db := openTest(t)

	for _, m := range Models {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(m); err != nil {
			t.Errorf("parse %T: %v", m, err)
			continue
		}
		if !db.Migrator().HasTable(m) {
			t.Errorf("%T declares table %q, which does not exist", m, stmt.Table)
		}
	}
}
