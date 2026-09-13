-- name: InsertPlan :one
INSERT INTO plans (
	plan_id, creator_user_id, target_user_id, place_id,
	visibility, deadline_hours, status, message, deadline_at
) VALUES ($1, $2, $3, $4, $5, $6, 'PENDING', $7, $8)
RETURNING created_at, updated_at;

-- name: InsertPlanParticipant :exec
INSERT INTO plan_participants (plan_id, user_id, role, status)
VALUES ($1, $2, $3, $4);

-- name: JoinPlan :execrows
INSERT INTO plan_participants (plan_id, user_id, role, status)
VALUES ($1, $2, 'GUEST', 'JOINED')
ON CONFLICT (plan_id, user_id) DO NOTHING;

-- name: GetPlanByID :one
SELECT
	p.plan_id, p.creator_user_id, p.target_user_id, p.place_id,
	p.visibility, p.status, p.message, p.deadline_hours, p.deadline_at,
	p.completed_at, p.created_at, p.updated_at,
	u.username AS creator_username,
	pl.name    AS place_name
FROM plans p
INNER JOIN users  u  ON u.user_id  = p.creator_user_id
INNER JOIN places pl ON pl.place_id = p.place_id
WHERE p.plan_id = $1;

-- name: HasActivePlan :one
SELECT COUNT(*) FROM plans
WHERE ((creator_user_id = $1 AND target_user_id = $2)
    OR (creator_user_id = $2 AND target_user_id = $1))
  AND place_id = $3
  AND status IN ('PENDING', 'ACCEPTED');

-- name: HasActiveOpenPlan :one
SELECT COUNT(*) FROM plans
WHERE creator_user_id = $1
  AND place_id = $2
  AND target_user_id IS NULL
  AND status = 'PENDING';

-- name: AcceptParticipant :execrows
UPDATE plan_participants SET status = 'JOINED'
WHERE plan_id = $1 AND user_id = $2 AND status = 'INVITED';

-- name: AcceptPlan :exec
UPDATE plans SET status = 'ACCEPTED', updated_at = NOW()
WHERE plan_id = $1 AND status = 'PENDING';

-- name: DeclineParticipant :execrows
UPDATE plan_participants SET status = 'DECLINED'
WHERE plan_id = $1 AND user_id = $2 AND status = 'INVITED';

-- name: DeclinePlan :exec
UPDATE plans SET status = 'DECLINED', updated_at = NOW()
WHERE plan_id = $1 AND status = 'PENDING';

-- name: MarkExpiredForUser :exec
UPDATE plans SET status = 'FAILED', updated_at = NOW()
WHERE status IN ('PENDING', 'ACCEPTED')
  AND deadline_at < NOW()
  AND (creator_user_id = $1
    OR EXISTS (
        SELECT 1 FROM plan_participants pp
        WHERE pp.plan_id = plans.plan_id AND pp.user_id = $1
    ));

-- name: GetIncomingPlans :many
SELECT
	p.plan_id, p.status, p.visibility, p.message,
	p.deadline_hours, p.created_at, p.deadline_at, p.completed_at,
	cu.user_id AS creator_user_id_obj, cu.username AS creator_username, cu.badge_level AS creator_badge_level,
	tu.user_id AS target_user_id_obj, tu.username AS target_username, tu.badge_level AS target_badge_level,
	pl.place_id, pl.name AS place_name, pl.category AS place_category,
	p.creator_user_id,
	(SELECT COUNT(*) FROM plan_participants pp
	 WHERE pp.plan_id = p.plan_id AND pp.status IN ('JOINED', 'COMPLETED'))::int AS participant_count,
	me.status AS my_status
FROM plans p
INNER JOIN users  cu ON cu.user_id  = p.creator_user_id
LEFT  JOIN users  tu ON tu.user_id  = p.target_user_id
INNER JOIN places pl ON pl.place_id = p.place_id
LEFT  JOIN plan_participants me ON me.plan_id = p.plan_id AND me.user_id = $1
WHERE me.role = 'GUEST' ORDER BY p.created_at DESC;

-- name: GetOutgoingPlans :many
SELECT
	p.plan_id, p.status, p.visibility, p.message,
	p.deadline_hours, p.created_at, p.deadline_at, p.completed_at,
	cu.user_id AS creator_user_id_obj, cu.username AS creator_username, cu.badge_level AS creator_badge_level,
	tu.user_id AS target_user_id_obj, tu.username AS target_username, tu.badge_level AS target_badge_level,
	pl.place_id, pl.name AS place_name, pl.category AS place_category,
	p.creator_user_id,
	(SELECT COUNT(*) FROM plan_participants pp
	 WHERE pp.plan_id = p.plan_id AND pp.status IN ('JOINED', 'COMPLETED'))::int AS participant_count,
	me.status AS my_status
FROM plans p
INNER JOIN users  cu ON cu.user_id  = p.creator_user_id
LEFT  JOIN users  tu ON tu.user_id  = p.target_user_id
INNER JOIN places pl ON pl.place_id = p.place_id
LEFT  JOIN plan_participants me ON me.plan_id = p.plan_id AND me.user_id = $1
WHERE p.creator_user_id = $1 ORDER BY p.created_at DESC;

-- name: DiscoverOpenPlans :many
SELECT
	p.plan_id, p.visibility, p.status, p.message,
	p.deadline_hours, p.deadline_at, p.created_at,
	cu.user_id AS creator_user_id_obj, cu.username AS creator_username, cu.badge_level AS creator_badge_level,
	pl.place_id, pl.name AS place_name, pl.category AS place_category,
	ST_Y(pl.location::geometry)::float8 AS lat, ST_X(pl.location::geometry)::float8 AS lon,
	(SELECT COUNT(*) FROM plan_participants pp
	 WHERE pp.plan_id = p.plan_id AND pp.status IN ('JOINED', 'COMPLETED'))::int AS participant_count
FROM plans p
INNER JOIN users  cu ON cu.user_id  = p.creator_user_id
INNER JOIN places pl ON pl.place_id = p.place_id
WHERE p.status = 'PENDING'
  AND p.target_user_id IS NULL
  AND p.deadline_at > NOW()
  AND p.creator_user_id <> $1
  AND NOT EXISTS (
      SELECT 1 FROM plan_participants me
      WHERE me.plan_id = p.plan_id AND me.user_id = $1
  )
  AND (
      p.visibility = 'PUBLIC'
      OR (p.visibility = 'FRIENDS' AND EXISTS (
          SELECT 1 FROM friendships f
          WHERE f.status = 'accepted'
            AND ((f.user_id_a = p.creator_user_id AND f.user_id_b = $1)
              OR (f.user_id_a = $1 AND f.user_id_b = p.creator_user_id))
      ))
  )
  AND ST_DWithin(pl.location, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4)
ORDER BY p.created_at DESC
LIMIT 100;

-- name: GetMyPlanByID :one
SELECT
	p.plan_id, p.status, p.visibility, p.message,
	p.deadline_hours, p.created_at, p.deadline_at, p.completed_at,
	cu.user_id AS creator_user_id_obj, cu.username AS creator_username, cu.badge_level AS creator_badge_level,
	tu.user_id AS target_user_id_obj, tu.username AS target_username, tu.badge_level AS target_badge_level,
	pl.place_id, pl.name AS place_name, pl.category AS place_category,
	p.creator_user_id,
	(SELECT COUNT(*) FROM plan_participants pp
	 WHERE pp.plan_id = p.plan_id AND pp.status IN ('JOINED', 'COMPLETED'))::int AS participant_count,
	me.status AS my_status
FROM plans p
INNER JOIN users  cu ON cu.user_id  = p.creator_user_id
LEFT  JOIN users  tu ON tu.user_id  = p.target_user_id
INNER JOIN places pl ON pl.place_id = p.place_id
LEFT  JOIN plan_participants me ON me.plan_id = p.plan_id AND me.user_id = $1
WHERE p.plan_id = $2;

-- name: CancelPlan :execrows
DELETE FROM plans
WHERE plan_id = $1 AND creator_user_id = $2 AND status = 'PENDING';

-- name: GetMatchingPlansForCompletion :many
SELECT p.plan_id
FROM plans p
INNER JOIN plan_participants pp
	ON pp.plan_id = p.plan_id AND pp.user_id = $1 AND pp.status = 'JOINED'
WHERE p.place_id    = $2
  AND p.status       IN ('PENDING', 'ACCEPTED')
  AND p.deadline_at  > NOW();

-- name: CompleteParticipant :execrows
UPDATE plan_participants SET status = 'COMPLETED', completed_at = NOW()
WHERE plan_id = $1 AND user_id = $2 AND status = 'JOINED';

-- name: CompletePlanIfReady :exec
UPDATE plans SET status = 'COMPLETED', completed_at = NOW(), updated_at = NOW()
WHERE plans.plan_id = $1
  AND plans.status <> 'COMPLETED'
  AND (SELECT COUNT(*) FROM plan_participants pp
       WHERE pp.plan_id = $1 AND pp.status = 'COMPLETED') >= 2;
