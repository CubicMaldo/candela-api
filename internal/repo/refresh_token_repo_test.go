package repo

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/testhelper"
)

// Detección de reuso de Refresh Tokens contra PostgreSQL:
// Probar que reutilizar un refresh token ya rotado revoca inmediatamente toda la sesión del usuario.
func TestRefreshTokenRepo_ReuseDetection_RevokesAllSessions(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	if pool == nil {
		return
	}
	ctx := context.Background()

	repo := NewRefreshTokenRepository(pool)

	// Create test user
	userID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (user_id, email, username, password_hash, birth_date, is_protected, account_status, privacy_mode)
		VALUES ($1, $2, $3, 'hash', '2000-01-01', false, 'ACTIVE', 'PUBLIC')
	`, userID, fmt.Sprintf("rt-%s@test.com", userID.String()[:8]), fmt.Sprintf("rt_%s", userID.String()[:8]))
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE user_id = $1`, userID)
	})

	rt1 := "test-refresh-token-1-" + uuid.New().String()
	rt2 := "test-refresh-token-2-" + uuid.New().String()
	rt3 := "test-refresh-token-3-" + uuid.New().String()
	rt4 := "test-refresh-token-4-" + uuid.New().String()

	// 1. Create initial refresh token (RT1)
	err = repo.Create(ctx, userID, rt1, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("failed to create RT1: %v", err)
	}

	// 2. Legitimate rotation: RT1 -> RT2
	rotatedUID, err := repo.Rotate(ctx, rt1, rt2, time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Rotate RT1 -> RT2 failed: %v", err)
	}
	if rotatedUID != userID {
		t.Fatalf("expected userID %s, got %s", userID, rotatedUID)
	}

	// Verify in DB that RT2 is active (revoked_at IS NULL)
	var countActive int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM refresh_tokens
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID).Scan(&countActive)
	if err != nil || countActive != 1 {
		t.Fatalf("expected 1 active refresh token after first rotation, got %d, err: %v", countActive, err)
	}

	// 3. Replay attack: Attacker attempts to reuse the already rotated RT1 to obtain RT3
	_, err = repo.Rotate(ctx, rt1, rt3, time.Now().Add(24*time.Hour))
	if !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Fatalf("expected ErrRefreshTokenInvalid when replaying RT1, got: %v", err)
	}

	// 4. Token Reuse Detection must have revoked all active sessions for this user!
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM refresh_tokens
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID).Scan(&countActive)
	if err != nil {
		t.Fatalf("failed to query active tokens: %v", err)
	}
	if countActive != 0 {
		t.Fatalf("expected 0 active refresh tokens after reuse detected, got %d", countActive)
	}

	// 5. Legitimate user attempting to use RT2 will now be rejected because RT2 was revoked on reuse!
	_, err = repo.Rotate(ctx, rt2, rt4, time.Now().Add(24*time.Hour))
	if !errors.Is(err, ErrRefreshTokenInvalid) {
		t.Fatalf("expected ErrRefreshTokenInvalid for RT2 after session revocation, got: %v", err)
	}
}
