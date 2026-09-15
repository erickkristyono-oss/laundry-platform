// Package config loads Gateway configuration from environment variables.
package config

import "os"

type Config struct {
	Port              string
	RedisAddr         string
	LogLevel          string
	JWTSigningKey     string
	IdentityUpstream  string
	CoreUpstream      string
	PaymentUpstream   string
	ReportingUpstream string
	AllowedOrigin     string
}

func Load() Config {
	return Config{
		Port:              getenv("PORT", "8080"),
		RedisAddr:         getenv("REDIS_ADDR", "localhost:6379"),
		LogLevel:          getenv("LOG_LEVEL", "info"),
		JWTSigningKey:     getenv("JWT_SIGNING_KEY", "dev-only-insecure-key-change-me"),
		IdentityUpstream:  getenv("IDENTITY_UPSTREAM", "http://identity:8081"),
		CoreUpstream:      getenv("CORE_UPSTREAM", "http://core:8082"),
		PaymentUpstream:   getenv("PAYMENT_UPSTREAM", "http://payment:8083"),
		ReportingUpstream: getenv("REPORTING_UPSTREAM", "http://reporting:8085"),
		AllowedOrigin:     getenv("ALLOWED_ORIGIN", "http://localhost:3000"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
