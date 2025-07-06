package config

import (
	"os"
)

type Config struct {
	// Server configuration
	Port string

	// Database configuration
	DatabaseURL string
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8081"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:testing@localhost:5433/envio-dev?sslmode=disable"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}