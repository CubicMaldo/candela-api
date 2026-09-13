-- name: FollowPlace :exec
INSERT INTO follows (follower_id, place_id, created_at)
VALUES ($1, $2, NOW())
ON CONFLICT (follower_id, place_id) DO NOTHING;

-- name: UnfollowPlace :exec
DELETE FROM follows WHERE follower_id = $1 AND place_id = $2;

-- name: IsFollowingPlace :one
SELECT COUNT(*) FROM follows WHERE follower_id = $1 AND place_id = $2;

-- name: GetFollowerCount :one
SELECT COUNT(*) FROM follows WHERE place_id = $1;
