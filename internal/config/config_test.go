package config

import (
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("GRPC_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NATS_URL", "")
	t.Setenv("LOG_LEVEL", "")

	c := Load()
	if c.GRPCPort != "50051" {
		t.Errorf("default GRPCPort = %q", c.GRPCPort)
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
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("GRPC_PORT", "1234")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("NATS_URL", "nats://x")
	t.Setenv("LOG_LEVEL", "debug")

	c := Load()
	if c.GRPCPort != "1234" || c.DatabaseURL != "postgres://x" || c.NATSUrl != "nats://x" || c.LogLevel != "debug" {
		t.Fatalf("env not applied: %+v", c)
	}
}
