package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func RateLimiter(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "rate_limit:" + c.ClientIP()

		ctx := context.Background()

		// Incrementar el contador para esta IP
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next() // Si falla Redis, permitimos la petición por resiliencia
			return
		}

		// Si es la primera vez, establecer el tiempo de vida de la ventana
		if count == 1 {
			rdb.Expire(ctx, key, window)
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
