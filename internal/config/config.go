package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// devJWTSecret is the throwaway signing key used when JWT_SECRET is unset.
// Load refuses to start with it in production — a predictable secret lets
// anyone forge a token for any user.
const devJWTSecret = "insecure-dev-only-secret"

type Config struct {
	AppEnv      string
	Port        string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
	GoogleCID   string
	// LocalTimezone is the IANA zone that defines a "day" for check-in rules
	// (first-of-day bonus, one-per-place-per-day). Candela is a nightlife app:
	// in UTC the day would roll over at 7pm local and reset those rules
	// mid-evening.
	LocalTimezone string

	StorageEndpoint string
	// StoragePublicEndpoint is the host embedded in presigned URLs — the one
	// devices can actually reach (e.g. http://10.0.2.2:9000 for the Android
	// emulator). Falls back to StorageEndpoint when unset.
	StoragePublicEndpoint string
	StorageAccessKey      string
	StorageSecretKey      string
	StorageBucket         string
	StorageUseSSL         bool

	// Database connection pool settings
	DBMaxConns        int32
	DBMinConns        int32
	DBMaxConnLifetime time.Duration
	DBMaxConnIdleTime time.Duration

	// Rate limiting settings (requests per minute)
	RateLimitAuth    int
	RateLimitSession int
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		JWTSecret:   getEnv("JWT_SECRET", devJWTSecret),
		GoogleCID:   os.Getenv("GOOGLE_CLIENT_ID"),

		LocalTimezone: getEnv("APP_TIMEZONE", "America/Bogota"),

		StorageEndpoint:       os.Getenv("STORAGE_ENDPOINT"),
		StoragePublicEndpoint: os.Getenv("STORAGE_PUBLIC_ENDPOINT"),
		StorageAccessKey:      os.Getenv("STORAGE_ACCESS_KEY"),
		StorageSecretKey:      os.Getenv("STORAGE_SECRET_KEY"),
		StorageBucket:         getEnv("STORAGE_BUCKET", "candela-photos"),
		StorageUseSSL:         os.Getenv("STORAGE_USE_SSL") == "true",

		DBMaxConns:        int32(getEnvInt("DB_MAX_CONNS", 25)),
		DBMinConns:        int32(getEnvInt("DB_MIN_CONNS", 5)),
		DBMaxConnLifetime: getEnvDuration("DB_MAX_CONN_LIFETIME", time.Hour),
		DBMaxConnIdleTime: getEnvDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute),

		RateLimitAuth:    getEnvInt("RATE_LIMIT_AUTH", 10),
		RateLimitSession: getEnvInt("RATE_LIMIT_SESSION", 120),
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

	if cfg.AppEnv == "production" {
		if cfg.JWTSecret == devJWTSecret {
			return Config{}, fmt.Errorf("JWT_SECRET is required in production")
		}
		if len(cfg.JWTSecret) < 32 {
			return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
		}
	}

	if _, err := time.LoadLocation(cfg.LocalTimezone); err != nil {
		return Config{}, fmt.Errorf("APP_TIMEZONE %q is not a valid IANA timezone: %w", cfg.LocalTimezone, err)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
