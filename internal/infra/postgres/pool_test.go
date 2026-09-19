package postgres

import (
	"context"
	"testing"
	"time"
)

func TestNewPoolWithRetry_InvalidURL(t *testing.T) {
	ctx := context.Background()
	_, err := NewPoolWithRetry(ctx, "invalid-url", RetryConfig{
		MaxRetries:   2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     20 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected error for invalid database URL, got nil")
	}
}

func TestNewPoolWithRetry_UnreachableHost(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	_, err := NewPoolWithRetry(ctx, "postgres://user:pass@127.0.0.1:54329/nonexistent?sslmode=disable", RetryConfig{
		MaxRetries:   3,
		InitialDelay: 20 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}

	// 3 attempts means waiting 2 times (at least 20ms + 30ms = 50ms)
	if elapsed < 40*time.Millisecond {
		t.Fatalf("expected at least 40ms of backoff, got %v", elapsed)
	}
}

func TestNewPoolWithRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := NewPoolWithRetry(ctx, "postgres://user:pass@127.0.0.1:54329/nonexistent?sslmode=disable", RetryConfig{
		MaxRetries:   5,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     1 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}
