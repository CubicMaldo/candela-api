-- name: CreateUser :one
INSERT INTO users (email, username, password_hash, birth_date, account_status, privacy_mode)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING user_id, EXTRACT(YEAR FROM age(CURRENT_DATE, birth_date))::INTEGER AS age, is_protected, points_total, badge_level, created_at, updated_at;

-- name: FindUserByID :one
SELECT user_id, email, username, password_hash, birth_date,
       EXTRACT(YEAR FROM age(CURRENT_DATE, birth_date))::INTEGER AS age,
       is_protected, account_status, privacy_mode, created_at, updated_at
FROM users WHERE user_id = $1;

-- name: GetUserByID :one
SELECT user_id, email, username, points_total, ranking_opt_in,
       streak_weeks_current, streak_weeks_max, streak_week_checkins,
       streak_week_start, streak_last_checkin
FROM users WHERE user_id = $1;

-- name: FindByUsername :one
SELECT u.user_id, u.username, u.badge_level, u.points_total,
       u.streak_weeks_current, u.streak_weeks_max, u.privacy_mode, u.is_protected,
       u.account_status, u.ranking_opt_in, u.created_at,
       COUNT(ci.checkin_id)::int AS checkin_count
FROM users u
LEFT JOIN check_ins ci ON ci.user_id = u.user_id
WHERE u.username = $1
  AND u.account_status = 'ACTIVE'
GROUP BY u.user_id;

-- name: SearchByUsernamePrefix :many
SELECT u.user_id, u.username, u.badge_level,
       COALESCE(f.status, '') = 'accepted' AS is_friend
FROM users u
LEFT JOIN friendships f
       ON f.user_id_a = LEAST(u.user_id, $1)
      AND f.user_id_b = GREATEST(u.user_id, $1)
WHERE u.username ILIKE $2 || '%'
  AND u.user_id <> $1
  AND u.account_status = 'ACTIVE'
  AND COALESCE(f.status, '') <> 'blocked'
  AND (u.privacy_mode <> 'PRIVATE' OR COALESCE(f.status, '') = 'accepted')
ORDER BY u.username ASC
LIMIT $3;

-- name: UpdatePrivacyMode :execrows
UPDATE users SET privacy_mode = $1 WHERE user_id = $2;

-- name: UpdateRankingOptIn :execrows
UPDATE users SET ranking_opt_in = $1 WHERE user_id = $2;

-- name: GetMyProfile :one
SELECT
	u.user_id, u.email, u.username, u.birth_date,
	EXTRACT(YEAR FROM age(CURRENT_DATE, u.birth_date))::INTEGER AS age,
	u.is_protected, u.account_status, u.privacy_mode,
	u.points_total, u.points_season, u.badge_level,
	u.streak_weeks_current, u.streak_weeks_max,
	u.streak_week_checkins, u.streak_week_start, u.streak_last_checkin,
	u.ranking_opt_in,
	u.created_at, u.updated_at,
	(SELECT COUNT(*) FROM check_ins ci
	 WHERE ci.user_id = $1 AND ci.validation_passed = true)::int AS check_in_count,
	(SELECT COUNT(*) FROM friendships f
	 WHERE (f.user_id_a = $1 OR f.user_id_b = $1) AND f.status = 'accepted')::int AS friends_count,
	(SELECT COUNT(*) FROM plan_participants pp
	 WHERE pp.user_id = $1 AND pp.status = 'COMPLETED')::int AS completed_plans_count
FROM users u
WHERE u.user_id = $1;

-- name: FindByEmail :one
SELECT user_id, email, username, password_hash, birth_date,
       EXTRACT(YEAR FROM age(CURRENT_DATE, birth_date))::INTEGER AS age,
       is_protected, account_status, privacy_mode, created_at, updated_at
FROM users WHERE email = $1;

-- name: CheckAccountDeleted :one
SELECT account_status = 'DELETED' FROM users WHERE user_id = $1 FOR UPDATE;

-- name: GetPhotoKeysForUser :many
SELECT photo_key FROM check_ins
WHERE user_id = $1 AND photo_key IS NOT NULL;

-- name: DeleteUserPlanParticipants :exec
DELETE FROM plan_participants WHERE user_id = $1;

-- name: DeleteUserPlans :exec
DELETE FROM plans WHERE creator_user_id = $1 OR target_user_id = $1;

-- name: DeleteUserFollows :exec
DELETE FROM follows WHERE follower_id = $1;

-- name: DeleteUserFriendships :exec
DELETE FROM friendships WHERE user_id_a = $1 OR user_id_b = $1;

-- name: DeleteUserPointTransactions :exec
DELETE FROM point_transactions WHERE user_id = $1;

-- name: DeleteUserCheckIns :exec
DELETE FROM check_ins WHERE user_id = $1;

-- name: AnonymizeUser :exec
UPDATE users SET
	account_status       = 'DELETED',
	email                = 'deleted_' || user_id || '@candela.deleted',
	username             = 'deleted_' || user_id,
	password_hash        = '',
	birth_date           = '1900-01-01',
	guardian_contact     = NULL,
	guardian_token       = NULL,
	points_total         = 0,
	points_season        = 0,
	badge_level          = 'Nuevo en la vuelta',
	streak_weeks_current = 0,
	streak_weeks_max     = 0,
	streak_week_checkins = 0,
	streak_week_start    = NULL,
	streak_last_checkin  = NULL,
	ranking_opt_in       = FALSE,
	privacy_mode         = 'PRIVATE'
WHERE user_id = $1;
