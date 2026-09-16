package spoofing_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/web/middleware"
	"github.com/tuusuario/candela-api/tests/testhelper"
)

// TestJWT_TamperedPayload_SignatureVerificationRejected verifies that modifying
// any claims in the JWT payload (e.g. is_protected: false or sub to victim UUID)
// without the secret key results in an HMAC signature mismatch rejected with 401 Unauthorized.
func TestJWT_TamperedPayload_SignatureVerificationRejected(t *testing.T) {
	rdb := testhelper.GetTestRedis(t)
	if rdb == nil {
		return
	}

	secret := "test-secret-at-least-32-characters-long"
	minorUserID := uuid.New()
	victimUserID := uuid.New()

	// 1. Generate legitimate token for a protected minor
	tokenStr := testhelper.GenerateTestJWT(secret, minorUserID, "ACTIVE", true, time.Now().Add(15*time.Minute))

	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		t.Fatalf("malformed JWT: expected 3 parts, got %d", len(parts))
	}

	// 2. Decode the payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("failed to decode JWT payload: %v", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		t.Fatalf("failed to unmarshal claims: %v", err)
	}

	// ATTACK 1: Attacker modifies is_protected to false (trying to bypass adult venue restriction)
	claims["is_protected"] = false
	tamperedPayloadBytes, _ := json.Marshal(claims)
	tamperedPayloadB64 := base64.RawURLEncoding.EncodeToString(tamperedPayloadBytes)
	tamperedToken1 := parts[0] + "." + tamperedPayloadB64 + "." + parts[2]

	router := gin.New()
	router.Use(middleware.AuthMiddleware(secret, rdb))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req1 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req1.Header.Set("Authorization", "Bearer "+tamperedToken1)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for tampered is_protected claim, got %d: %s", w1.Code, w1.Body.String())
	}

	// ATTACK 2: Attacker modifies sub to impersonate victim user
	claims["sub"] = victimUserID.String()
	tamperedPayloadBytes2, _ := json.Marshal(claims)
	tamperedPayloadB642 := base64.RawURLEncoding.EncodeToString(tamperedPayloadBytes2)
	tamperedToken2 := parts[0] + "." + tamperedPayloadB642 + "." + parts[2]

	req2 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req2.Header.Set("Authorization", "Bearer "+tamperedToken2)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for tampered sub claim, got %d: %s", w2.Code, w2.Body.String())
	}
}

// TestJWT_AlgorithmNone_RejectedWith401 verifies that attempting to use the 'none' algorithm
// without a signature is strictly rejected by the auth middleware.
func TestJWT_AlgorithmNone_RejectedWith401(t *testing.T) {
	rdb := testhelper.GetTestRedis(t)
	if rdb == nil {
		return
	}
	secret := "test-secret-at-least-32-characters-long"

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claimsB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"` + uuid.New().String() + `","exp":` + fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()) + `,"account_status":"ACTIVE"}`))
	noneToken := headerB64 + "." + claimsB64 + "."

	router := gin.New()
	router.Use(middleware.AuthMiddleware(secret, rdb))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+noneToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for alg:none attack, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRefreshToken_ConcurrentReplay_RevokesAllSessions verifies that concurrent rotation
// requests using the same refresh token trigger theft/replay detection in PostgreSQL (RFC 6819)
// and immediately revoke all active sessions for that user.
func TestRefreshToken_ConcurrentReplay_RevokesAllSessions(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	if pool == nil {
		return
	}
	ctx := context.Background()

	refreshRepo := repo.NewRefreshTokenRepository(pool)
	userID := testhelper.CreateTestUser(t, ctx, pool, false)

	rtInitial := "test-token-replay-" + uuid.New().String()
	err := refreshRepo.Create(ctx, userID, rtInitial, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("failed to create initial refresh token: %v", err)
	}

	// Two clients concurrently attempt to rotate the SAME rtInitial:
	// Client A rotates rtInitial -> rtNewA
	// Client B rotates rtInitial -> rtNewB
	rtNewA := "test-token-a-" + uuid.New().String()
	rtNewB := "test-token-b-" + uuid.New().String()

	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	var errA, errB error
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-startBarrier
		_, errA = refreshRepo.Rotate(ctx, rtInitial, rtNewA, time.Now().Add(24*time.Hour))
	}()

	go func() {
		defer wg.Done()
		<-startBarrier
		_, errB = refreshRepo.Rotate(ctx, rtInitial, rtNewB, time.Now().Add(24*time.Hour))
	}()

	close(startBarrier)
	wg.Wait()

	// At least one MUST fail with ErrRefreshTokenInvalid due to replay detection
	hasInvalid := errors.Is(errA, repo.ErrRefreshTokenInvalid) || errors.Is(errB, repo.ErrRefreshTokenInvalid)
	if !hasInvalid {
		t.Fatalf("expected at least one rotation to fail with ErrRefreshTokenInvalid, got errA=%v, errB=%v", errA, errB)
	}

	// Now verify that ALL active sessions for this user were revoked!
	var countActive int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, userID).Scan(&countActive)
	if err != nil {
		t.Fatalf("querying active refresh tokens failed: %v", err)
	}

	// Because replay detection revoked all sessions, count must be 0
	if countActive != 0 {
		t.Fatalf("expected 0 active refresh tokens after concurrent replay detection, got %d", countActive)
	}

	// Attempting to use whichever token was minted will now also fail
	_, errA2 := refreshRepo.Rotate(ctx, rtNewA, "rt-next-a", time.Now().Add(24*time.Hour))
	_, errB2 := refreshRepo.Rotate(ctx, rtNewB, "rt-next-b", time.Now().Add(24*time.Hour))

	if !errors.Is(errA2, repo.ErrRefreshTokenInvalid) && !errors.Is(errB2, repo.ErrRefreshTokenInvalid) {
		t.Errorf("expected downstream tokens to be rejected after session revocation")
	}
}
