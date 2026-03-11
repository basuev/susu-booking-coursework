package config

import "os"

type Config struct {
	GRPCPort    string
	DatabaseURL string
	NATSUrl     string
	LogLevel    string
}

func Load() Config {
	return Config{
		GRPCPort:    getEnv("GRPC_PORT", "50051"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://booking:booking@localhost:5432/booking?sslmode=disable"),
		NATSUrl:     getEnv("NATS_URL", "nats://localhost:4222"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
