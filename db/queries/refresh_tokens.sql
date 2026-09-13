-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3);

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = NOW()
WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshTokens :execrows
DELETE FROM refresh_tokens
WHERE expires_at < NOW() - INTERVAL '30 days'
   OR revoked_at < NOW() - INTERVAL '30 days';
