// Package config loads Identity Service configuration from environment
// variables. Kept local to the service (not shared/) since env var names
// and defaults are service-specific, per docs/03-system-architecture.md §4.
package config

import (
	"os"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	RedisAddr      string
	RabbitMQURL    string
	LogLevel       string
	JWTSigningKey  string
	AccessTokenTTL time.Duration
	RefreshTTL     time.Duration
	CoreServiceURL string // synchronous call to create/upsert the Core customer profile on registration (docs/06-database-schema.md §1.9)
}

func Load() Config {
	return Config{
		Port:           getenv("PORT", "8081"),
		DatabaseURL:    getenv("DATABASE_URL", "postgres://identity:identity@localhost:5432/identity_db?sslmode=disable"),
		RedisAddr:      getenv("REDIS_ADDR", "localhost:6379"),
		RabbitMQURL:    getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		LogLevel:       getenv("LOG_LEVEL", "info"),
		JWTSigningKey:  getenv("JWT_SIGNING_KEY", "dev-only-insecure-key-change-me"),
		AccessTokenTTL: 15 * time.Minute,
		RefreshTTL:     7 * 24 * time.Hour,
		CoreServiceURL: getenv("CORE_SERVICE_URL", "http://core:8082"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
