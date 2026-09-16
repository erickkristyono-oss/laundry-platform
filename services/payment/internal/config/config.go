// Package config loads Payment Service configuration from environment variables.
package config

import "os"

type Config struct {
	Port            string
	DatabaseURL     string
	RedisAddr       string
	RabbitMQURL     string
	LogLevel        string
	CoreServiceURL  string // synchronous read-only call target for order-total verification (docs/04-service-boundaries.md §4)
	PaymentProvider string // online-gateway provider selector: "dummy" until a real one (Midtrans, Xendit, ...) is registered
	WebBaseURL      string // frontend origin, used to build the DummyProvider's checkout/simulate link
}

func Load() Config {
	return Config{
		Port:            getenv("PORT", "8083"),
		DatabaseURL:     getenv("DATABASE_URL", "postgres://payment:payment@localhost:5432/payment_db?sslmode=disable"),
		RedisAddr:       getenv("REDIS_ADDR", "localhost:6379"),
		RabbitMQURL:     getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		LogLevel:        getenv("LOG_LEVEL", "info"),
		CoreServiceURL:  getenv("CORE_SERVICE_URL", "http://core:8082"),
		PaymentProvider: getenv("PAYMENT_PROVIDER", "dummy"),
		WebBaseURL:      getenv("WEB_BASE_URL", "http://localhost:3000"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
