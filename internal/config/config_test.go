package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("GRPC_PORT", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NATS_URL", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("DB_MAX_OPEN_CONNS", "")
	t.Setenv("DB_MAX_IDLE_CONNS", "")
	t.Setenv("DB_CONN_MAX_LIFETIME", "")
	t.Setenv("OUTBOX_BATCH_SIZE", "")
	t.Setenv("OUTBOX_INTERVAL", "")
	t.Setenv("OUTBOX_MAX_ATTEMPTS", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("RPC_TIMEOUT", "")
	t.Setenv("RUN_MIGRATIONS", "")

	c := Load()
	if c.GRPCPort != "50051" {
		t.Errorf("default GRPCPort = %q", c.GRPCPort)
	}
	if c.HTTPAddr != ":8080" {
		t.Errorf("default HTTPAddr = %q", c.HTTPAddr)
	}
	if c.NATSUrl != "nats://localhost:4222" {
		t.Errorf("default NATSUrl = %q", c.NATSUrl)
	}
	if c.LogLevel != "info" {
		t.Errorf("default LogLevel = %q", c.LogLevel)
	}
	if c.DatabaseURL == "" {
		t.Errorf("default DatabaseURL is empty")
	}
	if c.DBMaxOpenConns != 25 {
		t.Errorf("default DBMaxOpenConns = %d", c.DBMaxOpenConns)
	}
	if c.DBMaxIdleConns != 5 {
		t.Errorf("default DBMaxIdleConns = %d", c.DBMaxIdleConns)
	}
	if c.DBConnMaxLifetime != 30*time.Minute {
		t.Errorf("default DBConnMaxLifetime = %s", c.DBConnMaxLifetime)
	}
	if c.OutboxBatchSize != 50 {
		t.Errorf("default OutboxBatchSize = %d", c.OutboxBatchSize)
	}
	if c.OutboxInterval != 500*time.Millisecond {
		t.Errorf("default OutboxInterval = %s", c.OutboxInterval)
	}
	if c.OutboxMaxAttempts != 10 {
		t.Errorf("default OutboxMaxAttempts = %d", c.OutboxMaxAttempts)
	}
	if c.ShutdownTimeout != 10*time.Second {
		t.Errorf("default ShutdownTimeout = %s", c.ShutdownTimeout)
	}
	if c.RPCTimeout != 30*time.Second {
		t.Errorf("default RPCTimeout = %s", c.RPCTimeout)
	}
	if !c.RunMigrations {
		t.Errorf("default RunMigrations = false")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("GRPC_PORT", "1234")
	t.Setenv("HTTP_ADDR", ":9000")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("NATS_URL", "nats://x")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("DB_MAX_OPEN_CONNS", "100")
	t.Setenv("DB_MAX_IDLE_CONNS", "10")
	t.Setenv("DB_CONN_MAX_LIFETIME", "1h")
	t.Setenv("OUTBOX_BATCH_SIZE", "200")
	t.Setenv("OUTBOX_INTERVAL", "2s")
	t.Setenv("OUTBOX_MAX_ATTEMPTS", "20")
	t.Setenv("SHUTDOWN_TIMEOUT", "5s")
	t.Setenv("RPC_TIMEOUT", "15s")
	t.Setenv("RUN_MIGRATIONS", "false")

	c := Load()
	if c.GRPCPort != "1234" || c.DatabaseURL != "postgres://x" || c.NATSUrl != "nats://x" || c.LogLevel != "debug" {
		t.Fatalf("env not applied: %+v", c)
	}
	if c.HTTPAddr != ":9000" {
		t.Errorf("HTTPAddr = %q", c.HTTPAddr)
	}
	if c.DBMaxOpenConns != 100 || c.DBMaxIdleConns != 10 {
		t.Errorf("pool conns: %+v", c)
	}
	if c.DBConnMaxLifetime != time.Hour {
		t.Errorf("DBConnMaxLifetime = %s", c.DBConnMaxLifetime)
	}
	if c.OutboxBatchSize != 200 || c.OutboxInterval != 2*time.Second || c.OutboxMaxAttempts != 20 {
		t.Errorf("outbox knobs: %+v", c)
	}
	if c.ShutdownTimeout != 5*time.Second || c.RPCTimeout != 15*time.Second {
		t.Errorf("timeouts: %+v", c)
	}
	if c.RunMigrations {
		t.Errorf("RunMigrations should be false")
	}
}

func TestLoad_InvalidEnvFallsBack(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "not-a-number")
	t.Setenv("DB_CONN_MAX_LIFETIME", "not-a-duration")
	t.Setenv("RUN_MIGRATIONS", "not-a-bool")

	c := Load()
	if c.DBMaxOpenConns != 25 {
		t.Errorf("want default DBMaxOpenConns, got %d", c.DBMaxOpenConns)
	}
	if c.DBConnMaxLifetime != 30*time.Minute {
		t.Errorf("want default DBConnMaxLifetime, got %s", c.DBConnMaxLifetime)
	}
	if !c.RunMigrations {
		t.Errorf("want default RunMigrations=true")
	}
}
