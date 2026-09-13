-- name: GetFriendshipStatus :one
SELECT status FROM friendships
WHERE user_id_a = $1 AND user_id_b = $2;

-- name: GetFriendshipWithRequester :one
SELECT status, requester_id FROM friendships
WHERE user_id_a = $1 AND user_id_b = $2;

-- name: BlockUser :exec
INSERT INTO friendships (user_id_a, user_id_b, status, requester_id)
VALUES ($1, $2, 'blocked', $3)
ON CONFLICT (user_id_a, user_id_b) DO UPDATE SET status = 'blocked', requester_id = $3;

-- name: UnblockUser :execrows
DELETE FROM friendships
WHERE user_id_a = $1 AND user_id_b = $2 AND status = 'blocked';

-- name: SendFriendRequest :exec
INSERT INTO friendships (user_id_a, user_id_b, status, requester_id)
VALUES ($1, $2, 'pending', $3);

-- name: AcceptFriendRequest :execrows
UPDATE friendships SET status = 'accepted'
WHERE user_id_a = $1 AND user_id_b = $2 AND status = 'pending';

-- name: RejectFriendRequest :execrows
DELETE FROM friendships
WHERE user_id_a = $1 AND user_id_b = $2 AND status = 'pending';

-- name: CancelFriendRequest :execrows
DELETE FROM friendships
WHERE user_id_a = $1 AND user_id_b = $2 AND status = 'pending' AND requester_id = $3;

-- name: RemoveFriend :execrows
DELETE FROM friendships
WHERE user_id_a = $1 AND user_id_b = $2 AND status = 'accepted';

-- name: GetIncomingRequests :many
SELECT u.user_id, u.username, u.badge_level, f.created_at
FROM friendships f
INNER JOIN users u ON u.user_id = f.requester_id
WHERE (f.user_id_a = $1 OR f.user_id_b = $1)
  AND f.status = 'pending'
  AND f.requester_id != $1
  AND u.account_status = 'ACTIVE'
ORDER BY f.created_at DESC;

-- name: GetOutgoingRequests :many
SELECT u.user_id, u.username, u.badge_level, f.created_at
FROM friendships f
INNER JOIN users u ON u.user_id = (
	CASE WHEN f.user_id_a = $1 THEN f.user_id_b ELSE f.user_id_a END
)
WHERE (f.user_id_a = $1 OR f.user_id_b = $1)
  AND f.status = 'pending'
  AND f.requester_id = $1
  AND u.account_status = 'ACTIVE'
ORDER BY f.created_at DESC;

-- name: GetFriendsList :many
SELECT u.user_id, u.username, u.badge_level, u.points_total
FROM friendships f
INNER JOIN users u ON u.user_id = (
	CASE WHEN f.user_id_a = $1 THEN f.user_id_b ELSE f.user_id_a END
)
WHERE (f.user_id_a = $1 OR f.user_id_b = $1)
  AND f.status = 'accepted'
  AND u.account_status = 'ACTIVE'
ORDER BY u.username ASC;
