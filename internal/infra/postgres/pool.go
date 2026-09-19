package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RetryConfig defines the parameters for connection retry with backoff.
type RetryConfig struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

// PoolOptions defines parameters for connection pool sizing and lifecycle.
type PoolOptions struct {
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// DefaultRetryConfig specifies default retry parameters for PostgreSQL connection.
var DefaultRetryConfig = RetryConfig{
	MaxRetries:   10,
	InitialDelay: 1 * time.Second,
	MaxDelay:     2 * time.Second,
}

// NewPool initializes a pgxpool.Pool with default retry logic and backoff.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return NewPoolWithRetry(ctx, databaseURL, DefaultRetryConfig)
}

// NewPoolWithRetry initializes a pgxpool.Pool with custom retry configuration.
func NewPoolWithRetry(ctx context.Context, databaseURL string, cfg RetryConfig) (*pgxpool.Pool, error) {
	return NewPoolWithOptions(ctx, databaseURL, cfg, PoolOptions{})
}

// NewPoolFromConfig initializes a pgxpool.Pool with retry logic and connection pool settings.
func NewPoolFromConfig(ctx context.Context, databaseURL string, maxConns, minConns int32, maxConnLifetime, maxConnIdleTime time.Duration) (*pgxpool.Pool, error) {
	return NewPoolWithOptions(ctx, databaseURL, DefaultRetryConfig, PoolOptions{
		MaxConns:        maxConns,
		MinConns:        minConns,
		MaxConnLifetime: maxConnLifetime,
		MaxConnIdleTime: maxConnIdleTime,
	})
}

// NewPoolWithOptions initializes a pgxpool.Pool with custom retry and pool tuning options.
func NewPoolWithOptions(ctx context.Context, databaseURL string, cfg RetryConfig, opts PoolOptions) (*pgxpool.Pool, error) {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 1
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = 1 * time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 2 * time.Second
	}

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid database configuration: %w", err)
	}

	if opts.MaxConns > 0 {
		poolConfig.MaxConns = opts.MaxConns
	}
	if opts.MinConns > 0 {
		poolConfig.MinConns = opts.MinConns
	}
	if opts.MaxConnLifetime > 0 {
		poolConfig.MaxConnLifetime = opts.MaxConnLifetime
	}
	if opts.MaxConnIdleTime > 0 {
		poolConfig.MaxConnIdleTime = opts.MaxConnIdleTime
	}

	delay := cfg.InitialDelay
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("database connection aborted: %w", ctx.Err())
		}

		pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
		if err == nil {
			pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
			err = pool.Ping(pingCtx)
			pingCancel()

			if err == nil {
				return pool, nil
			}
			pool.Close()
		}

		lastErr = err

		if attempt == cfg.MaxRetries {
			break
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "57P03" {
			log.Printf("[WARN] PostgreSQL is starting up (SQLSTATE 57P03) (attempt %d/%d). Retrying in %v...", attempt, cfg.MaxRetries, delay)
		} else {
			log.Printf("[WARN] Unable to connect to PostgreSQL (attempt %d/%d): %v. Retrying in %v...", attempt, cfg.MaxRetries, err, delay)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("database connection aborted while waiting to retry: %w", ctx.Err())
		case <-time.After(delay):
		}

		delay = time.Duration(float64(delay) * 1.5)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	return nil, fmt.Errorf("failed to connect to PostgreSQL after %d attempts: %w", cfg.MaxRetries, lastErr)
}
