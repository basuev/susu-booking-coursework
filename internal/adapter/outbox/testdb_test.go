//go:build integration

package outbox

import (
	"context"
	"database/sql"
	"fmt"
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

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("outbox_test"),
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
		t.Fatalf("conn string: %v", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	deadline := time.Now().Add(30 * time.Second)
	for {
		if err := db.PingContext(ctx); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("db not reachable")
		}
		time.Sleep(200 * time.Millisecond)
	}

	driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{})
	if err != nil {
		t.Fatalf("migrate driver: %v", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://"+migrationsDir(t), "postgres", driver)
	if err != nil {
		t.Fatalf("migrate new: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrate up: %v", err)
	}
	return db
}

func insertPending(t *testing.T, db *sql.DB, topic, key string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`INSERT INTO outbox_message (topic, key, payload) VALUES ($1, $2, $3) RETURNING id`,
		topic, key, []byte(fmt.Sprintf(`{"k":"%s"}`, key))).Scan(&id)
	if err != nil {
		t.Fatalf("insert pending: %v", err)
	}
	return id
}
