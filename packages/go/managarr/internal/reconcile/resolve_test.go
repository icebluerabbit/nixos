package reconcile

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/store"
)

// TestResolveStatementsAreValid runs every link against a real schema.
//
// A resolve link is raw SQL, so a column that was renamed in the model is not a
// compile error here — it is a runtime failure on a production pass. Executing
// each statement against an empty database costs nothing and catches exactly
// that.
func TestResolveStatementsAreValid(t *testing.T) {
	dsn := os.Getenv("MANAGARR_TEST_DSN")
	if dsn == "" {
		t.Skip("MANAGARR_TEST_DSN not set")
	}
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	db, err := store.OpenGorm(ctx, dsn, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(db, logger); err != nil {
		t.Fatal(err)
	}

	if _, err := NewResolve(dao.New(db, logger), logger).Run(ctx); err != nil {
		t.Fatalf("resolve: %v", err)
	}
}
