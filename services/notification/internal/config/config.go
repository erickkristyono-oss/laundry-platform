// Package config loads Notification Service configuration from environment variables.
package config

import "os"

type Config struct {
	Port           string
	DatabaseURL    string
	RedisAddr      string
	RabbitMQURL    string
	LogLevel       string
	CoreServiceURL string // to resolve order_code/customer phone for a message (docs/04-service-boundaries.md §4)
	FonnteToken    string // WhatsApp Business API token (fonnte.com); empty = NoopSender (logs only, no real send)
	FonnteAPIURL   string
}

func Load() Config {
	return Config{
		Port:           getenv("PORT", "8084"),
		DatabaseURL:    getenv("DATABASE_URL", "postgres://notification:notification@localhost:5432/notification_db?sslmode=disable"),
		RedisAddr:      getenv("REDIS_ADDR", "localhost:6379"),
		RabbitMQURL:    getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		LogLevel:       getenv("LOG_LEVEL", "info"),
		CoreServiceURL: getenv("CORE_SERVICE_URL", "http://core:8082"),
		FonnteToken:    getenv("FONNTE_TOKEN", ""),
		FonnteAPIURL:   getenv("FONNTE_API_URL", "https://api.fonnte.com/send"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
