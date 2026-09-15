// Package config loads Identity Service configuration from environment
// variables. Kept local to the service (not shared/) since env var names
// and defaults are service-specific, per docs/03-system-architecture.md §4.
package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	RedisAddr   string
	RabbitMQURL string
	LogLevel    string
}

func Load() Config {
	return Config{
		Port:        getenv("PORT", "8081"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://identity:identity@localhost:5432/identity_db?sslmode=disable"),
		RedisAddr:   getenv("REDIS_ADDR", "localhost:6379"),
		RabbitMQURL: getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		LogLevel:    getenv("LOG_LEVEL", "info"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
