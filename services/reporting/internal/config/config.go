// Package config loads Reporting Service configuration from environment variables.
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
		Port:        getenv("PORT", "8085"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://reporting:reporting@localhost:5432/reporting_db?sslmode=disable"),
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
