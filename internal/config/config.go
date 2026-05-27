package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	GRPCPort          string
	HTTPAddr          string
	DatabaseURL       string
	NATSUrl           string
	LogLevel          string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	OutboxBatchSize   int
	OutboxInterval    time.Duration
	OutboxMaxAttempts int
	ShutdownTimeout   time.Duration
	RPCTimeout        time.Duration
	RunMigrations     bool
}

func Load() Config {
	return Config{
		GRPCPort:          getEnv("GRPC_PORT", "50051"),
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://booking:booking@localhost:5432/booking?sslmode=disable"),
		NATSUrl:           getEnv("NATS_URL", "nats://localhost:4222"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		OutboxBatchSize:   getEnvInt("OUTBOX_BATCH_SIZE", 50),
		OutboxInterval:    getEnvDuration("OUTBOX_INTERVAL", 500*time.Millisecond),
		OutboxMaxAttempts: getEnvInt("OUTBOX_MAX_ATTEMPTS", 10),
		ShutdownTimeout:   getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		RPCTimeout:        getEnvDuration("RPC_TIMEOUT", 30*time.Second),
		RunMigrations:     getEnvBool("RUN_MIGRATIONS", true),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
