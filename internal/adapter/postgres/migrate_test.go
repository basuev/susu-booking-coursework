//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var coreTables = []string{
	"booking",
	"booking_offer_snapshot",
	"booking_status_history",
	"booking_projection",
	"idempotency_key",
	"outbox_message",
}

func TestRunMigrations_AppliesAllFromFS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("booking_migrate"),
		tcpostgres.WithUsername("booking"),
		tcpostgres.WithPassword("booking"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := waitForDB(ctx, db); err != nil {
		t.Fatalf("wait: %v", err)
	}

	_, file, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
	files := os.DirFS(migrationsDir)
	if err := RunMigrations(db, files); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if got := countCoreTables(t, db); got != len(coreTables) {
		t.Fatalf("expected %d core tables after up, got %d", len(coreTables), got)
	}

	if err := RunMigrations(db, files); err != nil {
		t.Fatalf("migrate (idempotent rerun): %v", err)
	}
}

func TestMigrations_DownUpRoundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("booking_roundtrip"),
		tcpostgres.WithUsername("booking"),
		tcpostgres.WithPassword("booking"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := waitForDB(ctx, db); err != nil {
		t.Fatalf("wait: %v", err)
	}

	dir := migrationsDir(t)
	m := newFileMigrator(t, db, dir)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("first up: %v", err)
	}
	if got := countCoreTables(t, db); got != len(coreTables) {
		t.Fatalf("after first up: want %d tables, got %d", len(coreTables), got)
	}

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("down: %v", err)
	}
	if got := countCoreTables(t, db); got != 0 {
		t.Fatalf("after down: want 0 tables, got %d", got)
	}

	m2 := newFileMigrator(t, db, dir)
	if err := m2.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("second up: %v", err)
	}
	if got := countCoreTables(t, db); got != len(coreTables) {
		t.Fatalf("after second up: want %d tables, got %d", len(coreTables), got)
	}

	if err := m2.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("third up (idempotent): %v", err)
	}
}

func newFileMigrator(t *testing.T, db *sql.DB, dir string) *migrate.Migrate {
	t.Helper()
	driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{})
	if err != nil {
		t.Fatalf("migrate driver: %v", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+dir, "postgres", driver)
	if err != nil {
		t.Fatalf("migrate new: %v", err)
	}
	return m
}

func countCoreTables(t *testing.T, db *sql.DB) int {
	t.Helper()
	row := db.QueryRow(`
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema='public'
		  AND table_name IN ('booking','booking_offer_snapshot','booking_status_history','booking_projection','idempotency_key','outbox_message')
	`)
	var n int
	if err := row.Scan(&n); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return n
}
