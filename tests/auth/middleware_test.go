package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/service"
	"github.com/tuusuario/candela-api/internal/web/middleware"
	"github.com/tuusuario/candela-api/tests/testhelper"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// 1. Revocación y Blacklist en Redis:
// Probar que tras invocar DeleteAccount, el token JWT activo queda en la blacklist de Redis
// y el middleware rechaza peticiones subsiguientes con 401 Unauthorized.
func TestAuthMiddleware_RedisBlacklist_DeleteAccount(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()
	secret := "test-secret-at-least-32-characters-long"

	mockStore := testhelper.NewMockStorage()
	ur := repo.NewUserRepository(pool)
	fr := repo.NewFriendshipRepository(pool)
	rr := repo.NewRefreshTokenRepository(pool)
	userSvc := service.NewUserService(ur, fr, rr, mockStore, rdb)

	userID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (user_id, email, username, password_hash, birth_date, is_protected, account_status, privacy_mode)
		VALUES ($1, $2, $3, 'hash', '2000-01-01', false, 'ACTIVE', 'PUBLIC')
	`, userID, fmt.Sprintf("del-%s@test.com", userID.String()[:8]), fmt.Sprintf("del_%s", userID.String()[:8]))
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	tokenExpiry := time.Now().Add(15 * time.Minute)
	tokenStr := testhelper.GenerateTestJWT(secret, userID, "ACTIVE", false, tokenExpiry)

	router := gin.New()
	router.Use(middleware.AuthMiddleware(secret, rdb))
	router.GET("/test-protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "authorized"})
	})

	// Before DeleteAccount: request succeeds with 200 OK
	req1 := httptest.NewRequest(http.MethodGet, "/test-protected", nil)
	req1.Header.Set("Authorization", "Bearer "+tokenStr)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK before DeleteAccount, got %d: %s", w1.Code, w1.Body.String())
	}

	// Invoke DeleteAccount
	err = userSvc.DeleteAccount(ctx, userID, tokenExpiry)
	if err != nil {
		t.Fatalf("DeleteAccount failed: %v", err)
	}

	// Verify Redis blacklist key was set
	blockKey := fmt.Sprintf(middleware.JWTBlocklistKey, userID.String())
	exists, err := rdb.Exists(ctx, blockKey).Result()
	if err != nil || exists == 0 {
		t.Fatalf("expected Redis blocklist key %s to exist, err: %v", blockKey, err)
	}

	// After DeleteAccount: subsequent requests with the same JWT must be rejected with 401 Unauthorized
	req2 := httptest.NewRequest(http.MethodGet, "/test-protected", nil)
	req2.Header.Set("Authorization", "Bearer "+tokenStr)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized after DeleteAccount, got %d: %s", w2.Code, w2.Body.String())
	}
}

// 2. Estados de cuenta suspendidos:
// Probar que un JWT emitido para una cuenta con accountStatus == "SUSPENDED" es bloqueado con 403 Forbidden.
func TestAuthMiddleware_SuspendedAccount(t *testing.T) {
	rdb := testhelper.GetTestRedis(t)
	if rdb == nil {
		return
	}
	secret := "test-secret-at-least-32-characters-long"

	router := gin.New()
	router.Use(middleware.AuthMiddleware(secret, rdb))
	router.GET("/test-protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	userID := uuid.New()
	suspendedToken := testhelper.GenerateTestJWT(secret, userID, "SUSPENDED", false, time.Now().Add(15*time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/test-protected", nil)
	req.Header.Set("Authorization", "Bearer "+suspendedToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for SUSPENDED account, got %d: %s", w.Code, w.Body.String())
	}
}

// 3. Estados de cuenta eliminados:
// Probar que un JWT emitido para una cuenta con accountStatus == "DELETED" es bloqueado con 403 Forbidden.
func TestAuthMiddleware_DeletedAccount(t *testing.T) {
	rdb := testhelper.GetTestRedis(t)
	if rdb == nil {
		return
	}
	secret := "test-secret-at-least-32-characters-long"

	router := gin.New()
	router.Use(middleware.AuthMiddleware(secret, rdb))
	router.GET("/test-protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	userID := uuid.New()
	deletedToken := testhelper.GenerateTestJWT(secret, userID, "DELETED", false, time.Now().Add(15*time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/test-protected", nil)
	req.Header.Set("Authorization", "Bearer "+deletedToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for DELETED account, got %d: %s", w.Code, w.Body.String())
	}
}
