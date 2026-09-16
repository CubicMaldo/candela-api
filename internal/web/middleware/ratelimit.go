package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// rateLimitScript atomically increments the counter and ensures a TTL is set.
// If the counter was 1, it sets the expiration window in milliseconds.
// If the key already existed without a TTL (TTL == -1), it repairs it immediately.
var rateLimitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
    redis.call('PEXPIRE', KEYS[1], ARGV[1])
else
    local ttl = redis.call('PTTL', KEYS[1])
    if ttl == -1 then
        redis.call('PEXPIRE', KEYS[1], ARGV[1])
    end
end
return count
`)

func RateLimiter(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "rate_limit:" + c.ClientIP()
		ctx := c.Request.Context()

		windowMs := window.Milliseconds()
		count, err := rateLimitScript.Run(ctx, rdb, []string{key}, windowMs).Int64()
		if err != nil {
			c.Next() // Si falla Redis, permitimos la petición por resiliencia
			return
		}

		if count > int64(limit) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Demasiadas peticiones. Por favor, intenta de nuevo más tarde.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
