package spoofing_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tuusuario/candela-api/internal/web/middleware"
	"github.com/tuusuario/candela-api/tests/testhelper"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestIPSpoofing_RateLimitBypassBlocked verifies that an attacker attempting to bypass
// the rate limiter by spoofing X-Forwarded-For or X-Real-IP headers is STILL rate-limited
// because SetTrustedProxies(nil) forces Gin to use the real connection socket address (RemoteAddr).
func TestIPSpoofing_RateLimitBypassBlocked(t *testing.T) {
	rdb := testhelper.GetTestRedis(t)
	if rdb == nil {
		return
	}
	ctx := context.Background()

	realClientIP := fmt.Sprintf("198.51.100.%d", time.Now().UnixNano()%250)
	key := "rate_limit:" + realClientIP
	_ = rdb.Del(ctx, key).Err()
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), key).Err()
	})

	limit := 5
	window := 1 * time.Minute

	router := gin.New()
	// Disable trusting upstream proxies: Gin will ignore spoofed X-Forwarded-For headers
	_ = router.SetTrustedProxies(nil)

	router.Use(middleware.RateLimiter(rdb, limit, window))
	router.GET("/api/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Attacker sends 10 requests from the same physical socket/RemoteAddr,
	// but changes X-Forwarded-For on EVERY request to simulate 10 different fake clients.
	for i := 1; i <= 10; i++ {
		fakeIP := fmt.Sprintf("203.0.113.%d", i)
		req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
		req.RemoteAddr = realClientIP + ":45678" // The real connection IP
		req.Header.Set("X-Forwarded-For", fakeIP)
		req.Header.Set("X-Real-IP", fakeIP)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i <= limit {
			if w.Code != http.StatusOK {
				t.Fatalf("request %d expected 200 OK, got %d", i, w.Code)
			}
		} else {
			// Requests beyond the limit MUST be rejected with 429,
			// despite having fake/spoofed X-Forwarded-For IP headers!
			if w.Code != http.StatusTooManyRequests {
				t.Fatalf("IP SPOOFING VULNERABILITY: Request %d with fake IP %s was NOT rate-limited! Got HTTP %d",
					i, fakeIP, w.Code)
			}
		}
	}
}
