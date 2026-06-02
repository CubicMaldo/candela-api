package config

import (
	"fmt"
	"os"







)

type Config struct {
	AppEnv      string
	Port        string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
	GoogleCID   string
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		JWTSecret:   getEnv("JWT_SECRET", "super-secret-key-1234"),
		GoogleCID:   os.Getenv("GOOGLE_CLIENT_ID"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}