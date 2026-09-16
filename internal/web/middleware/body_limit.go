package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxBodySize limits the HTTP request body size to protect against JSON / payload bombs and memory exhaustion (OOM).
func MaxBodySize(limitBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Fast check: Content-Length header
		if c.Request.ContentLength > limitBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "El cuerpo de la petición excede el tamaño máximo permitido.",
			})
			return
		}

		// Wrap Request.Body with http.MaxBytesReader to handle chunked encoding or missing Content-Length
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limitBytes)
		c.Next()
	}
}
