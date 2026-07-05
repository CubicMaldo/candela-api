package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/candela-api/internal/domain"
)

var (
	ErrPlanNotFound  = errors.New("plan not found")
	ErrAlreadyJoined = errors.New("already a participant")
)

const planSelect = `
	SELECT
		p.plan_id, p.creator_user_id, p.target_user_id, p.place_id,
		p.visibility, p.status, p.message, p.deadline_hours, p.deadline_at,
		p.completed_at, p.created_at, p.updated_at,
		u.username AS creator_username,
		pl.name    AS place_name
	FROM plans p
	INNER JOIN users  u  ON u.user_id  = p.creator_user_id
	INNER JOIN places pl ON pl.place_id = p.place_id
`

type PlanRepository struct {
	db *pgxpool.Pool
}

func NewPlanRepository(db *pgxpool.Pool) *PlanRepository {
	return &PlanRepository{db: db}
}

func scanDetail(row pgx.Row) (*domain.PlanDetail, error) {
	var d domain.PlanDetail
	err := row.Scan(
		&d.PlanID, &d.CreatorUserID, &d.TargetUserID, &d.PlaceID,
		&d.Visibility, &d.Status, &d.Message, &d.DeadlineHours, &d.DeadlineAt,
		&d.CompletedAt, &d.CreatedAt, &d.UpdatedAt,
		&d.CreatorUsername, &d.PlaceName,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// Create inserts the plan and its initial participant rows in one transaction:
// always the creator (CREATOR/JOINED), plus — for INVITE plans — the invited
// target (GUEST/INVITED). PUBLIC/FRIENDS plans start with only the creator and
// gather guests later via Join.
func (r *PlanRepository) Create(ctx context.Context, p *domain.Plan) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx, `
		INSERT INTO plans (
			plan_id, creator_user_id, target_user_id, place_id,
			visibility, deadline_hours, status, message, deadline_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'PENDING', $7, $8)
		RETURNING created_at, updated_at
	`,
		p.PlanID, p.CreatorUserID, p.TargetUserID, p.PlaceID,
		p.Visibility, p.DeadlineHours, p.Message, p.DeadlineAt,
	).Scan(&p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO plan_participants (plan_id, user_id, role, status)
		VALUES ($1, $2, 'CREATOR', 'JOINED')
	`, p.PlanID, p.CreatorUserID); err != nil {
		return err
	}

	if p.TargetUserID != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO plan_participants (plan_id, user_id, role, status)
			VALUES ($1, $2, 'GUEST', 'INVITED')
		`, p.PlanID, *p.TargetUserID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// Join adds the user as a JOINED guest of an open plan. Returns ErrAlreadyJoined
// if they already participate (creator or existing guest).
func (r *PlanRepository) Join(ctx context.Context, planID, userID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		INSERT INTO plan_participants (plan_id, user_id, role, status)
		VALUES ($1, $2, 'GUEST', 'JOINED')
		ON CONFLICT (plan_id, user_id) DO NOTHING
	`, planID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAlreadyJoined
	}
	return nil
}

func (r *PlanRepository) GetByID(ctx context.Context, planID uuid.UUID) (*domain.PlanDetail, error) {
	row := r.db.QueryRow(ctx, planSelect+`WHERE p.plan_id = $1`, planID)
	d, err := scanDetail(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPlanNotFound
	}
	return d, err
}

// HasActivePlan returns true if a PENDING or ACCEPTED INVITE plan already
// exists between the two users for the given place (in either direction).
func (r *PlanRepository) HasActivePlan(ctx context.Context, userA, userB, placeID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM plans
		WHERE ((creator_user_id = $1 AND target_user_id = $2)
		    OR (creator_user_id = $2 AND target_user_id = $1))
		  AND place_id = $3
		  AND status IN ('PENDING', 'ACCEPTED')
	`, userA, userB, placeID).Scan(&count)
	return count > 0, err
}

// HasActiveOpenPlan returns true if the creator already has an active (PENDING)
// open plan at this place — the dedupe rule for PUBLIC/FRIENDS plans.
func (r *PlanRepository) HasActiveOpenPlan(ctx context.Context, creatorID, placeID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM plans
		WHERE creator_user_id = $1
		  AND place_id = $2
		  AND target_user_id IS NULL
		  AND status = 'PENDING'
	`, creatorID, placeID).Scan(&count)
	return count > 0, err
}

// Accept marks the invited guest JOINED and flips the (INVITE) plan to ACCEPTED
// so plan-completion points continue to fire on the participants' check-ins.
func (r *PlanRepository) Accept(ctx context.Context, planID, targetUserID uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE plan_participants SET status = 'JOINED'
		WHERE plan_id = $1 AND user_id = $2 AND status = 'INVITED'
	`, planID, targetUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPlanNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE plans SET status = 'ACCEPTED', updated_at = NOW()
		WHERE plan_id = $1 AND status = 'PENDING'
	`, planID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Decline marks the invited guest DECLINED and the (INVITE) plan DECLINED.
func (r *PlanRepository) Decline(ctx context.Context, planID, targetUserID uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE plan_participants SET status = 'DECLINED'
		WHERE plan_id = $1 AND user_id = $2 AND status = 'INVITED'
	`, planID, targetUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPlanNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE plans SET status = 'DECLINED', updated_at = NOW()
		WHERE plan_id = $1 AND status = 'PENDING'
	`, planID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// MarkExpiredForUser lazily marks PENDING/ACCEPTED plans past their deadline as
// FAILED, for any plan the user creates or participates in.
func (r *PlanRepository) MarkExpiredForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE plans SET status = 'FAILED', updated_at = NOW()
		WHERE status IN ('PENDING', 'ACCEPTED')
		  AND deadline_at < NOW()
		  AND (creator_user_id = $1
		    OR EXISTS (
		        SELECT 1 FROM plan_participants pp
		        WHERE pp.plan_id = plans.plan_id AND pp.user_id = $1
		    ))
	`, userID)
	return err
}

// myPlanSelect is the participant-facing plan projection. The viewer id is
// always $1 (used to resolve the viewer's own participant status); callers add
// their WHERE clause using $2+. Target is LEFT JOINed (nil for open plans) and a
// participant count + the viewer's status are included for the UI.
const myPlanSelect = `
	SELECT
		p.plan_id, p.status, p.visibility, p.message,
		p.deadline_hours, p.created_at, p.deadline_at, p.completed_at,
		cu.user_id, cu.username, cu.badge_level,
		tu.user_id, tu.username, tu.badge_level,
		pl.place_id, pl.name, pl.category,
		p.creator_user_id,
		(SELECT COUNT(*) FROM plan_participants pp
		 WHERE pp.plan_id = p.plan_id AND pp.status IN ('JOINED', 'COMPLETED')) AS participant_count,
		me.status AS my_status
	FROM plans p
	INNER JOIN users  cu ON cu.user_id  = p.creator_user_id
	LEFT  JOIN users  tu ON tu.user_id  = p.target_user_id
	INNER JOIN places pl ON pl.place_id = p.place_id
	LEFT  JOIN plan_participants me ON me.plan_id = p.plan_id AND me.user_id = $1
`

func scanMyPlan(row pgx.Row, viewerID uuid.UUID) (domain.MyPlan, error) {
	var mp domain.MyPlan
	var creatorID uuid.UUID
	var tgtID *uuid.UUID
	var tgtUsername, tgtBadge *string
	var myStatus *string
	err := row.Scan(
		&mp.PlanID, &mp.Status, &mp.Visibility, &mp.Message,
		&mp.DeadlineHours, &mp.CreatedAt, &mp.DeadlineAt, &mp.CompletedAt,
		&mp.Creator.UserID, &mp.Creator.Username, &mp.Creator.BadgeLevel,
		&tgtID, &tgtUsername, &tgtBadge,
		&mp.Place.PlaceID, &mp.Place.Name, &mp.Place.Category,
		&creatorID, &mp.ParticipantCount, &myStatus,
	)
	if err != nil {
		return mp, err
	}
	if tgtID != nil {
		mp.Target = &domain.MyPlanUser{UserID: *tgtID}
		if tgtUsername != nil {
			mp.Target.Username = *tgtUsername
		}
		if tgtBadge != nil {
			mp.Target.BadgeLevel = *tgtBadge
		}
	}
	if myStatus != nil {
		mp.MyStatus = *myStatus
	}
	if creatorID == viewerID {
		mp.Role = "creator"
	} else {
		mp.Role = "guest"
	}
	return mp, nil
}

func scanMyPlans(rows pgx.Rows, viewerID uuid.UUID) ([]domain.MyPlan, error) {
	var results []domain.MyPlan
	for rows.Next() {
		mp, err := scanMyPlan(rows, viewerID)
		if err != nil {
			return nil, err
		}
		results = append(results, mp)
	}
	return results, rows.Err()
}

// GetIncoming returns plans where the user is a guest (invited or joined).
func (r *PlanRepository) GetIncoming(ctx context.Context, userID uuid.UUID) ([]domain.MyPlan, error) {
	rows, err := r.db.Query(ctx,
		myPlanSelect+`WHERE me.role = 'GUEST' ORDER BY p.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMyPlans(rows, userID)
}

// GetOutgoing returns plans the user created.
func (r *PlanRepository) GetOutgoing(ctx context.Context, userID uuid.UUID) ([]domain.MyPlan, error) {
	rows, err := r.db.Query(ctx,
		myPlanSelect+`WHERE p.creator_user_id = $1 ORDER BY p.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMyPlans(rows, userID)
}

// DiscoverOpenPlans returns active open plans near (lat, lon) the viewer can
// join: PUBLIC plans, or FRIENDS plans whose creator is a friend. Excludes the
// viewer's own plans and any they already participate in. radiusMeters bounds
// the search; results carry place coordinates for map pins.
func (r *PlanRepository) DiscoverOpenPlans(ctx context.Context, viewerID uuid.UUID, lat, lon, radiusMeters float64) ([]domain.DiscoverPlan, error) {
	// ST_MakePoint takes (lon, lat) — order matters for PostGIS.
	rows, err := r.db.Query(ctx, `
		SELECT
			p.plan_id, p.visibility, p.status, p.message,
			p.deadline_hours, p.deadline_at, p.created_at,
			cu.user_id, cu.username, cu.badge_level,
			pl.place_id, pl.name, pl.category,
			ST_Y(pl.location::geometry) AS lat, ST_X(pl.location::geometry) AS lon,
			(SELECT COUNT(*) FROM plan_participants pp
			 WHERE pp.plan_id = p.plan_id AND pp.status IN ('JOINED', 'COMPLETED')) AS participant_count
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
		LIMIT 100
	`, viewerID, lon, lat, radiusMeters)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.DiscoverPlan
	for rows.Next() {
		var dp domain.DiscoverPlan
		if err := rows.Scan(
			&dp.PlanID, &dp.Visibility, &dp.Status, &dp.Message,
			&dp.DeadlineHours, &dp.DeadlineAt, &dp.CreatedAt,
			&dp.Creator.UserID, &dp.Creator.Username, &dp.Creator.BadgeLevel,
			&dp.Place.PlaceID, &dp.Place.Name, &dp.Place.Category,
			&dp.Place.Lat, &dp.Place.Lon,
			&dp.ParticipantCount,
		); err != nil {
			return nil, err
		}
		results = append(results, dp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.DiscoverPlan{}
	}
	return results, nil
}

// GetMyPlanByID returns a single plan in the participant-facing shape, with
// Role and MyStatus computed relative to viewerID.
func (r *PlanRepository) GetMyPlanByID(ctx context.Context, planID, viewerID uuid.UUID) (*domain.MyPlan, error) {
	row := r.db.QueryRow(ctx, myPlanSelect+`WHERE p.plan_id = $2`, viewerID, planID)
	mp, err := scanMyPlan(row, viewerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPlanNotFound
	}
	if err != nil {
		return nil, err
	}
	return &mp, nil
}

// Cancel deletes a still-pending plan created by creatorID (cascade removes
// participant rows).
func (r *PlanRepository) Cancel(ctx context.Context, planID, creatorID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM plans
		WHERE plan_id = $1 AND creator_user_id = $2 AND status = 'PENDING'
	`, planID, creatorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPlanNotFound
	}
	return nil
}

// GetMyPlans returns all plans where the user is creator or a participant.
// statusFilter: "active" | "completed" | "failed" | "" (all).
// Runs lazy expiration before querying so FAILED plans reflect current state.
func (r *PlanRepository) GetMyPlans(ctx context.Context, userID uuid.UUID, statusFilter string) ([]domain.MyPlan, error) {
	var statusClause string
	switch statusFilter {
	case "active":
		statusClause = "AND p.status IN ('PENDING', 'ACCEPTED')"
	case "completed":
		statusClause = "AND p.status = 'COMPLETED'"
	case "failed":
		statusClause = "AND p.status IN ('FAILED', 'DECLINED')"
	}

	rows, err := r.db.Query(ctx, myPlanSelect+`
		WHERE (p.creator_user_id = $1 OR me.user_id IS NOT NULL)
		`+statusClause+`
		ORDER BY p.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMyPlans(rows, userID)
}

// planParticipantBonus is the base plan bonus each participant earns, once, when
// they concretan their participation (check in at the place within the deadline).
// Scaled by the user's weekly-streak multiplier.
const planParticipantBonus = 10

// CompleteMatchingPlans concretes the checking-in user's participation in any of
// their active plans for this place: for each plan where the user is a JOINED
// participant, still active and within deadline, it marks that participant
// COMPLETED and awards them the plan bonus once (scaled by multiplier). A plan
// flips to COMPLETED once ≥2 participants have shown up ("se armó"). Returns the
// total bonus awarded to userID, for the check-in's points breakdown.
func (r *PlanRepository) CompleteMatchingPlans(ctx context.Context, userID, placeID uuid.UUID, multiplier float64) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	// Plans at this place where the user is a JOINED participant and the plan is
	// still open within its deadline.
	rows, err := tx.Query(ctx, `
		SELECT p.plan_id
		FROM plans p
		INNER JOIN plan_participants pp
			ON pp.plan_id = p.plan_id AND pp.user_id = $1 AND pp.status = 'JOINED'
		WHERE p.place_id    = $2
		  AND p.status       IN ('PENDING', 'ACCEPTED')
		  AND p.deadline_at  > NOW()
	`, userID, placeID)
	if err != nil {
		return 0, err
	}
	var planIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		planIDs = append(planIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(planIDs) == 0 {
		return 0, nil
	}

	bonus := int(float64(planParticipantBonus) * multiplier)
	total := 0

	for _, planID := range planIDs {
		// Flip this participant to COMPLETED. The guard makes the award idempotent
		// even if the same plan matches twice.
		tag, err := tx.Exec(ctx, `
			UPDATE plan_participants SET status = 'COMPLETED', completed_at = NOW()
			WHERE plan_id = $1 AND user_id = $2 AND status = 'JOINED'
		`, planID, userID)
		if err != nil {
			return 0, err
		}
		if tag.RowsAffected() == 0 {
			continue
		}

		if _, err = tx.Exec(ctx, `
			UPDATE users SET points_total = points_total + $1, points_season = points_season + $1,
				badge_level = `+badgeCaseSQL("points_total + $1")+`, updated_at = NOW()
			WHERE user_id = $2
		`, bonus, userID); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO point_transactions (tx_id, user_id, points, reason, reference_id)
			VALUES (uuid_generate_v4(), $1, $2, 'PLAN', $3)
		`, userID, bonus, planID); err != nil {
			return 0, err
		}
		total += bonus

		// The plan "se armó" once at least two participants have shown up.
		if _, err = tx.Exec(ctx, `
			UPDATE plans SET status = 'COMPLETED', completed_at = NOW(), updated_at = NOW()
			WHERE plan_id = $1
			  AND status <> 'COMPLETED'
			  AND (SELECT COUNT(*) FROM plan_participants pp
			       WHERE pp.plan_id = $1 AND pp.status = 'COMPLETED') >= 2
		`, planID); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return total, nil
}
