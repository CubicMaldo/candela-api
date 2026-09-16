package repo

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrRefreshTokenInvalid covers every reason a refresh token cannot be used:
// unknown, already rotated, revoked, or expired. Callers must not distinguish
// them — telling an attacker *why* a token failed is free information.
var ErrRefreshTokenInvalid = errors.New("refresh token is not valid")

type RefreshTokenRepository struct {
	db *pgxpool.Pool
}

func NewRefreshTokenRepository(db *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

// hashToken reduces a refresh token to what is safe to persist. SHA-256 without
// a salt is deliberate: the tokens are 256 bits of CSPRNG output, so they are
// not guessable and a slow KDF would only add latency to every refresh.
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// Create stores a new refresh token for a user.
func (r *RefreshTokenRepository) Create(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hashToken(token), expiresAt)
	return err
}

// Rotate atomically consumes `oldToken` and issues `newToken` in its place,
// returning the owning user. Doing both in one transaction means a token can
// never be spent twice, even if two refreshes race: the second finds the row
// already revoked and fails.
func (r *RefreshTokenRepository) Rotate(
	ctx context.Context,
	oldToken, newToken string,
	newExpiry time.Time,
) (uuid.UUID, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
		RETURNING user_id
	`, hashToken(oldToken)).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Token reuse detection: if this token existed and was already revoked,
			// an attacker might be attempting a replay attack. Revoke all active
			// sessions for this user immediately (RFC 6819).
			var reusedUserID uuid.UUID
			checkErr := tx.QueryRow(ctx, `
				SELECT user_id FROM refresh_tokens
				WHERE token_hash = $1 AND revoked_at IS NOT NULL
			`, hashToken(oldToken)).Scan(&reusedUserID)
			if checkErr == nil {
				_, _ = tx.Exec(ctx, `
					UPDATE refresh_tokens SET revoked_at = NOW()
					WHERE user_id = $1 AND revoked_at IS NULL
				`, reusedUserID)
				_ = tx.Commit(ctx)
			}
			return uuid.Nil, ErrRefreshTokenInvalid
		}
		return uuid.Nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hashToken(newToken), newExpiry); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// Revoke invalidates a single token (logout on this device). Revoking an
// already-invalid token is not an error — logout must always succeed.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, hashToken(token))
	return err
}

// RevokeAllForUser invalidates every session of a user (account deletion).
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	return err
}

// DeleteExpired drops tokens that can no longer be used, so the table does not
// grow without bound. Returns how many rows were removed.
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM refresh_tokens
		WHERE expires_at < NOW() - INTERVAL '30 days'
		   OR revoked_at < NOW() - INTERVAL '30 days'
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
