-- name: GetGlobalRankingCount :one
SELECT COUNT(*)
FROM users
WHERE account_status = 'ACTIVE'
  AND privacy_mode   = 'PUBLIC'
  AND is_protected   = false
  AND ranking_opt_in = TRUE;

-- name: GetGlobalRanking :many
SELECT
	ROW_NUMBER() OVER (ORDER BY points_total DESC)::int AS rank,
	user_id,
	username,
	badge_level,
	points_total
FROM users
WHERE account_status = 'ACTIVE'
  AND privacy_mode   = 'PUBLIC'
  AND is_protected   = false
  AND ranking_opt_in = TRUE
ORDER BY points_total DESC
LIMIT $1 OFFSET $2;

-- name: GetUserGlobalRank :one
SELECT rank::int, user_id, username, badge_level, points_total
FROM (
	SELECT
		ROW_NUMBER() OVER (ORDER BY points_total DESC) AS rank,
		user_id,
		username,
		badge_level,
		points_total
	FROM users
	WHERE account_status = 'ACTIVE'
	  AND privacy_mode   = 'PUBLIC'
	  AND is_protected   = false
	  AND ranking_opt_in = TRUE
) ranked
WHERE user_id = $1;

-- name: GetFriendsRanking :many
SELECT
	ROW_NUMBER() OVER (ORDER BY u.points_total DESC)::int AS rank,
	u.user_id,
	u.username,
	u.badge_level,
	u.points_total
FROM users u
INNER JOIN friendships f
	ON (f.user_id_a = u.user_id OR f.user_id_b = u.user_id)
WHERE f.status = 'accepted'
  AND (f.user_id_a = $1 OR f.user_id_b = $1)
  AND u.user_id != $1
  AND u.account_status = 'ACTIVE'
ORDER BY u.points_total DESC;

-- name: GetUserFriendRank :one
SELECT rank::int, user_id, username, badge_level, points_total
FROM (
	SELECT
		ROW_NUMBER() OVER (ORDER BY u.points_total DESC) AS rank,
		u.user_id,
		u.username,
		u.badge_level,
		u.points_total
	FROM users u
	WHERE u.user_id = $1
	   OR u.user_id IN (
			SELECT CASE
				WHEN f.user_id_a = $1 THEN f.user_id_b
				ELSE f.user_id_a
			END
			FROM friendships f
			WHERE (f.user_id_a = $1 OR f.user_id_b = $1)
			  AND f.status = 'accepted'
		)
) ranked
WHERE user_id = $1;
