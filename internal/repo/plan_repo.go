package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/nomad-api/internal/domain"
)

var ErrPlanNotFound = errors.New("plan not found")

const planSelect = `
	SELECT
		p.plan_id, p.creator_user_id, p.target_user_id, p.place_id,
		p.status, p.message, p.deadline_hours, p.deadline_at,
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
		&d.Status, &d.Message, &d.DeadlineHours, &d.DeadlineAt,
		&d.CompletedAt, &d.CreatedAt, &d.UpdatedAt,
		&d.CreatorUsername, &d.PlaceName,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *PlanRepository) Create(ctx context.Context, p *domain.Plan) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO plans (
			plan_id, creator_user_id, target_user_id, place_id,
			deadline_hours, status, message, deadline_at
		) VALUES ($1, $2, $3, $4, $5, 'PENDING', $6, $7)
		RETURNING created_at, updated_at
	`,
		p.PlanID, p.CreatorUserID, p.TargetUserID, p.PlaceID,
		p.DeadlineHours, p.Message, p.DeadlineAt,
	).Scan(&p.CreatedAt, &p.UpdatedAt)
}

func (r *PlanRepository) GetByID(ctx context.Context, planID uuid.UUID) (*domain.PlanDetail, error) {
	row := r.db.QueryRow(ctx, planSelect+`WHERE p.plan_id = $1`, planID)
	d, err := scanDetail(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPlanNotFound
	}
	return d, err
}

// HasActivePlan returns true if a PENDING or ACCEPTED plan already
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

func (r *PlanRepository) Accept(ctx context.Context, planID, targetUserID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE plans SET status = 'ACCEPTED', updated_at = NOW()
		WHERE plan_id = $1 AND target_user_id = $2 AND status = 'PENDING'
	`, planID, targetUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPlanNotFound
	}
	return nil
}

func (r *PlanRepository) Decline(ctx context.Context, planID, targetUserID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE plans SET status = 'DECLINED', updated_at = NOW()
		WHERE plan_id = $1 AND target_user_id = $2 AND status = 'PENDING'
	`, planID, targetUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPlanNotFound
	}
	return nil
}

// MarkExpiredForUser lazily marks PENDING/ACCEPTED plans past their deadline as FAILED.
func (r *PlanRepository) MarkExpiredForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE plans SET status = 'FAILED', updated_at = NOW()
		WHERE status IN ('PENDING', 'ACCEPTED')
		  AND deadline_at < NOW()
		  AND (creator_user_id = $1 OR target_user_id = $1)
	`, userID)
	return err
}

func (r *PlanRepository) GetIncoming(ctx context.Context, userID uuid.UUID) ([]domain.PlanDetail, error) {
	rows, err := r.db.Query(ctx,
		planSelect+`WHERE p.target_user_id = $1 ORDER BY p.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDetails(rows)
}

func (r *PlanRepository) GetOutgoing(ctx context.Context, userID uuid.UUID) ([]domain.PlanDetail, error) {
	rows, err := r.db.Query(ctx,
		planSelect+`WHERE p.creator_user_id = $1 ORDER BY p.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDetails(rows)
}

func scanDetails(rows pgx.Rows) ([]domain.PlanDetail, error) {
	var results []domain.PlanDetail
	for rows.Next() {
		var d domain.PlanDetail
		if err := rows.Scan(
			&d.PlanID, &d.CreatorUserID, &d.TargetUserID, &d.PlaceID,
			&d.Status, &d.Message, &d.DeadlineHours, &d.DeadlineAt,
			&d.CompletedAt, &d.CreatedAt, &d.UpdatedAt,
			&d.CreatorUsername, &d.PlaceName,
		); err != nil {
			return nil, err
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

// GetMyPlans returns all plans where the user is creator or target.
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

	rows, err := r.db.Query(ctx, `
		SELECT
			p.plan_id, p.status, p.message,
			p.deadline_hours, p.created_at, p.deadline_at, p.completed_at,
			cu.user_id, cu.username, cu.badge_level,
			tu.user_id, tu.username, tu.badge_level,
			pl.place_id, pl.name, pl.category,
			p.creator_user_id
		FROM plans p
		INNER JOIN users  cu ON cu.user_id  = p.creator_user_id
		INNER JOIN users  tu ON tu.user_id  = p.target_user_id
		INNER JOIN places pl ON pl.place_id = p.place_id
		WHERE (p.creator_user_id = $1 OR p.target_user_id = $1)
		`+statusClause+`
		ORDER BY p.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.MyPlan
	for rows.Next() {
		var mp domain.MyPlan
		var creatorID uuid.UUID
		if err := rows.Scan(
			&mp.PlanID, &mp.Status, &mp.Message,
			&mp.DeadlineHours, &mp.CreatedAt, &mp.DeadlineAt, &mp.CompletedAt,
			&mp.Creator.UserID, &mp.Creator.Username, &mp.Creator.BadgeLevel,
			&mp.Target.UserID, &mp.Target.Username, &mp.Target.BadgeLevel,
			&mp.Place.PlaceID, &mp.Place.Name, &mp.Place.Category,
			&creatorID,
		); err != nil {
			return nil, err
		}
		if creatorID == userID {
			mp.Role = "creator"
		} else {
			mp.Role = "target"
		}
		results = append(results, mp)
	}
	return results, rows.Err()
}

// CompleteMatchingPlans finds all ACCEPTED plans for (targetUser, place) within
// deadline, marks them COMPLETED, and awards bonus points to both participants in a TX.
// The multiplier (from the target user's weekly streak) scales both bonuses.
// Returns the total (multiplied) plan-bonus points awarded to targetUserID, for
// inclusion in the check-in's points breakdown.
func (r *PlanRepository) CompleteMatchingPlans(ctx context.Context, targetUserID, placeID uuid.UUID, multiplier float64) (int, error) {
	type match struct {
		planID    uuid.UUID
		creatorID uuid.UUID
	}

	rows, err := r.db.Query(ctx, `
		SELECT plan_id, creator_user_id FROM plans
		WHERE target_user_id = $1
		  AND place_id       = $2
		  AND status         = 'ACCEPTED'
		  AND deadline_at    > NOW()
	`, targetUserID, placeID)
	if err != nil {
		return 0, err
	}

	var matches []match
	for rows.Next() {
		var m match
		if err := rows.Scan(&m.planID, &m.creatorID); err != nil {
			rows.Close()
			return 0, err
		}
		matches = append(matches, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(matches) == 0 {
		return 0, nil
	}

	ptCreator := int(float64(8) * multiplier)
	ptTarget := int(float64(10) * multiplier)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	totalTargetBonus := 0
	for _, m := range matches {
		if _, err = tx.Exec(ctx, `
			UPDATE plans SET
				status       = 'COMPLETED',
				completed_at = NOW(),
				updated_at   = NOW()
			WHERE plan_id = $1
		`, m.planID); err != nil {
			return 0, err
		}

		if _, err = tx.Exec(ctx, `
			UPDATE users SET points_total = points_total + $1, points_season = points_season + $1, updated_at = NOW()
			WHERE user_id = $2
		`, ptCreator, m.creatorID); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO point_transactions (tx_id, user_id, points, reason, reference_id)
			VALUES (uuid_generate_v4(), $1, $2, 'PLAN', $3)
		`, m.creatorID, ptCreator, m.planID); err != nil {
			return 0, err
		}

		if _, err = tx.Exec(ctx, `
			UPDATE users SET points_total = points_total + $1, points_season = points_season + $1, updated_at = NOW()
			WHERE user_id = $2
		`, ptTarget, targetUserID); err != nil {
			return 0, err
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO point_transactions (tx_id, user_id, points, reason, reference_id)
			VALUES (uuid_generate_v4(), $1, $2, 'PLAN', $3)
		`, targetUserID, ptTarget, m.planID); err != nil {
			return 0, err
		}

		totalTargetBonus += ptTarget
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return totalTargetBonus, nil
}
