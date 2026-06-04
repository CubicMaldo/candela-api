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

	StorageEndpoint  string
	StorageAccessKey string
	StorageSecretKey string
	StorageBucket    string
	StorageUseSSL    bool
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		JWTSecret:   getEnv("JWT_SECRET", "super-secret-key-1234"),
		GoogleCID:   os.Getenv("GOOGLE_CLIENT_ID"),

		StorageEndpoint:  os.Getenv("STORAGE_ENDPOINT"),
		StorageAccessKey: os.Getenv("STORAGE_ACCESS_KEY"),
		StorageSecretKey: os.Getenv("STORAGE_SECRET_KEY"),
		StorageBucket:    getEnv("STORAGE_BUCKET", "nomad-photos"),
		StorageUseSSL:    os.Getenv("STORAGE_USE_SSL") == "true",
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}
	if cfg.StorageEndpoint == "" {
		return Config{}, fmt.Errorf("STORAGE_ENDPOINT is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
