package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// RetryConfig defines the parameters for connection retry with backoff.
type RetryConfig struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

// DefaultRetryConfig specifies default retry parameters for Redis connection.
var DefaultRetryConfig = RetryConfig{
	MaxRetries:   10,
	InitialDelay: 1 * time.Second,
	MaxDelay:     2 * time.Second,
}

// NewClient initializes a redis.Client with default retry logic and backoff.
func NewClient(ctx context.Context, redisURL string) (*redis.Client, error) {
	return NewClientWithRetry(ctx, redisURL, DefaultRetryConfig)
}

// NewClientWithRetry initializes a redis.Client with custom retry configuration.
func NewClientWithRetry(ctx context.Context, redisURL string, cfg RetryConfig) (*redis.Client, error) {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 1
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = 1 * time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 2 * time.Second
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis configuration: %w", err)
	}

	delay := cfg.InitialDelay
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("redis connection aborted: %w", ctx.Err())
		}

		client := redis.NewClient(opts)
		pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
		_, err = client.Ping(pingCtx).Result()
		pingCancel()

		if err == nil {
			return client, nil
		}

		_ = client.Close()
		lastErr = err

		if attempt == cfg.MaxRetries {
			break
		}

		log.Printf("[WARN] Unable to connect to Redis (attempt %d/%d): %v. Retrying in %v...", attempt, cfg.MaxRetries, err, delay)

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("redis connection aborted while waiting to retry: %w", ctx.Err())
		case <-time.After(delay):
		}

		delay = time.Duration(float64(delay) * 1.5)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	return nil, fmt.Errorf("failed to connect to Redis after %d attempts: %w", cfg.MaxRetries, lastErr)
}
