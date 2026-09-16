package race_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/web/middleware"
	"github.com/tuusuario/candela-api/tests/testhelper"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestRateLimiter_ConcurrentBurst_TTLNeverMissing verifies that when 50 concurrent requests
// hit the rate limiter simultaneously on a new key, the atomic Lua script ensures:
// 1. The key is NEVER created without a TTL (no immortal key bug).
// 2. The TTL is strictly > 0 and <= window.
func TestRateLimiter_ConcurrentBurst_TTLNeverMissing(t *testing.T) {
	rdb := testhelper.GetTestRedis(t)
	if rdb == nil {
		return
	}
	ctx := context.Background()

	// Use an isolated unique IP
	testIP := fmt.Sprintf("192.168.%d.%d", time.Now().UnixNano()%250, time.Now().UnixNano()%200)
	key := "rate_limit:" + testIP
	_ = rdb.Del(ctx, key).Err()
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), key).Err()
	})

	window := 10 * time.Second
	limit := 100 // high limit so we don't abort early

	router := gin.New()
	router.Use(middleware.RateLimiter(rdb, limit, window))
	router.GET("/api/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	concurrency := 50
	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier

			req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
			req.RemoteAddr = testIP + ":12345"
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}()
	}

	close(startBarrier)
	wg.Wait()

	// Check the key's TTL in Redis
	ttl, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to query TTL from Redis: %v", err)
	}

	// If the race condition existed, ttl would be negative (key has no expiration, immortal)
	if ttl <= 0 {
		t.Fatalf("RACE CONDITION BUG: Redis key %s was left immortal without a TTL (TTL = %v)", key, ttl)
	}
	if ttl > window {
		t.Fatalf("expected TTL <= window (%v), got: %v", window, ttl)
	}
}

// TestRateLimiter_SelfHealing_RepairsImmortalKeys verifies that if an orphaned key
// exists in Redis without a TTL (TTL = -1, e.g. from a past crash), the atomic Lua script
// automatically detects and repairs it by setting the TTL on the next request.
func TestRateLimiter_SelfHealing_RepairsImmortalKeys(t *testing.T) {
	rdb := testhelper.GetTestRedis(t)
	if rdb == nil {
		return
	}
	ctx := context.Background()

	testIP := fmt.Sprintf("10.0.%d.%d", uuid.New()[0], uuid.New()[1])
	key := "rate_limit:" + testIP

	// Simulate an immortal key created without TTL (expiration = 0)
	err := rdb.Set(ctx, key, 5, 0).Err()
	if err != nil {
		t.Fatalf("failed to seed test key: %v", err)
	}
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), key).Err()
	})

	// Verify it has no TTL before the test (go-redis returns -1ns for immortal key)
	preTTL, err := rdb.TTL(ctx, key).Result()
	if err != nil || preTTL >= 0 {
		t.Fatalf("expected seeded key to have negative TTL (no expiration), got: %v", preTTL)
	}

	window := 15 * time.Second
	router := gin.New()
	router.Use(middleware.RateLimiter(rdb, 10, window))
	router.GET("/api/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req.RemoteAddr = testIP + ":54321"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify that the Lua script repaired the immortal key by assigning a valid TTL
	postTTL, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to query TTL: %v", err)
	}
	if postTTL <= 0 {
		t.Fatalf("Self-healing failed: Key %s is still immortal without TTL (got %v)", key, postTTL)
	}
	if postTTL > window {
		t.Fatalf("expected TTL to be repaired to 0 < TTL <= %v, got %v", window, postTTL)
	}
}
