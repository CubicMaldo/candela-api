package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const JWTBlocklistKey = "jwt:blocklist:%s"

func AuthMiddleware(secret string, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header must be Bearer token"})
			return
		}

		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		userIDStr, ok := claims["sub"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token missing user identifier"})
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// Check JWT blocklist (set on account deletion).
		blockKey := fmt.Sprintf(JWTBlocklistKey, userIDStr)
		if exists, _ := rdb.Exists(c.Request.Context(), blockKey).Result(); exists > 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "account no longer active"})
			return
		}

		isProtected, _ := claims["is_protected"].(bool)
		accountStatus, _ := claims["account_status"].(string)

		if accountStatus == "SUSPENDED" || accountStatus == "DELETED" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Tu cuenta está suspendida. Escríbenos si crees que es un error."})
			return
		}

		// Parse expiry for use by DeleteAccount handler.
		var tokenExpiry time.Time
		if expFloat, ok := claims["exp"].(float64); ok {
			tokenExpiry = time.Unix(int64(expFloat), 0)
		}

		c.Set("userID", userID)
		c.Set("isProtected", isProtected)
		c.Set("accountStatus", accountStatus)
		c.Set("tokenExpiry", tokenExpiry)
		c.Next()
	}
}
