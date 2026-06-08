package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/nomad-api/internal/domain"
)

var ErrChallengeNotFound = errors.New("challenge not found")

const challengeSelect = `
	SELECT
		c.challenge_id, c.creator_user_id, c.target_user_id, c.place_id,
		c.deadline_hours, c.status, c.message, c.created_at, c.deadline_at,
		c.completed_at, c.completing_checkin_id, c.points_creator, c.points_target,
		u.username AS creator_username,
		p.name     AS place_name
	FROM challenges c
	INNER JOIN users  u ON u.user_id  = c.creator_user_id
	INNER JOIN places p ON p.place_id = c.place_id
`

type ChallengeRepository struct {
	db *pgxpool.Pool
}

func NewChallengeRepository(db *pgxpool.Pool) *ChallengeRepository {
	return &ChallengeRepository{db: db}
}

func scanDetail(row pgx.Row) (*domain.ChallengeDetail, error) {
	var d domain.ChallengeDetail
	err := row.Scan(
		&d.ChallengeID, &d.CreatorUserID, &d.TargetUserID, &d.PlaceID,
		&d.DeadlineHours, &d.Status, &d.Message, &d.CreatedAt, &d.DeadlineAt,
		&d.CompletedAt, &d.CompletingCheckinID, &d.PointsCreator, &d.PointsTarget,
		&d.CreatorUsername, &d.PlaceName,
	)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *ChallengeRepository) Create(ctx context.Context, c *domain.Challenge) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO challenges (
			challenge_id, creator_user_id, target_user_id, place_id,
			deadline_hours, status, message, deadline_at
		) VALUES ($1, $2, $3, $4, $5, 'PENDING', $6, $7)
		RETURNING created_at
	`,
		c.ChallengeID, c.CreatorUserID, c.TargetUserID, c.PlaceID,
		c.DeadlineHours, c.Message, c.DeadlineAt,
	).Scan(&c.CreatedAt)
}

func (r *ChallengeRepository) GetByID(ctx context.Context, challengeID uuid.UUID) (*domain.ChallengeDetail, error) {
	row := r.db.QueryRow(ctx, challengeSelect+`WHERE c.challenge_id = $1`, challengeID)
	d, err := scanDetail(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrChallengeNotFound
	}
	return d, err
}

// HasActiveChallenge returns true if a PENDING or ACCEPTED challenge already
// exists between the two users for the given place (in either direction).
func (r *ChallengeRepository) HasActiveChallenge(ctx context.Context, userA, userB, placeID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM challenges
		WHERE ((creator_user_id = $1 AND target_user_id = $2)
		    OR (creator_user_id = $2 AND target_user_id = $1))
		  AND place_id = $3
		  AND status IN ('PENDING', 'ACCEPTED')
	`, userA, userB, placeID).Scan(&count)
	return count > 0, err
}

func (r *ChallengeRepository) Accept(ctx context.Context, challengeID, targetUserID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE challenges SET status = 'ACCEPTED'
		WHERE challenge_id = $1 AND target_user_id = $2 AND status = 'PENDING'
	`, challengeID, targetUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrChallengeNotFound
	}
	return nil
}

func (r *ChallengeRepository) Reject(ctx context.Context, challengeID, targetUserID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE challenges SET status = 'REJECTED'
		WHERE challenge_id = $1 AND target_user_id = $2 AND status = 'PENDING'
	`, challengeID, targetUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrChallengeNotFound
	}
	return nil
}

// MarkExpiredForUser lazily marks PENDING/ACCEPTED challenges past their deadline as FAILED.
func (r *ChallengeRepository) MarkExpiredForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE challenges SET status = 'FAILED'
		WHERE status IN ('PENDING', 'ACCEPTED')
		  AND deadline_at < NOW()
		  AND (creator_user_id = $1 OR target_user_id = $1)
	`, userID)
	return err
}

func (r *ChallengeRepository) GetIncoming(ctx context.Context, userID uuid.UUID) ([]domain.ChallengeDetail, error) {
	rows, err := r.db.Query(ctx,
		challengeSelect+`WHERE c.target_user_id = $1 ORDER BY c.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDetails(rows)
}

func (r *ChallengeRepository) GetOutgoing(ctx context.Context, userID uuid.UUID) ([]domain.ChallengeDetail, error) {
	rows, err := r.db.Query(ctx,
		challengeSelect+`WHERE c.creator_user_id = $1 ORDER BY c.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDetails(rows)
}

func scanDetails(rows pgx.Rows) ([]domain.ChallengeDetail, error) {
	var results []domain.ChallengeDetail
	for rows.Next() {
		var d domain.ChallengeDetail
		if err := rows.Scan(
			&d.ChallengeID, &d.CreatorUserID, &d.TargetUserID, &d.PlaceID,
			&d.DeadlineHours, &d.Status, &d.Message, &d.CreatedAt, &d.DeadlineAt,
			&d.CompletedAt, &d.CompletingCheckinID, &d.PointsCreator, &d.PointsTarget,
			&d.CreatorUsername, &d.PlaceName,
		); err != nil {
			return nil, err
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

// GetMyChallenges returns all challenges where the user is creator or target.
// statusFilter: "active" | "completed" | "failed" | "" (all).
// Runs lazy expiration before querying so FAILED challenges reflect current state.
func (r *ChallengeRepository) GetMyChallenges(ctx context.Context, userID uuid.UUID, statusFilter string) ([]domain.MyChallenge, error) {
	var statusClause string
	switch statusFilter {
	case "active":
		statusClause = "AND c.status IN ('PENDING', 'ACCEPTED')"
	case "completed":
		statusClause = "AND c.status = 'COMPLETED'"
	case "failed":
		statusClause = "AND c.status IN ('FAILED', 'REJECTED')"
	}

	rows, err := r.db.Query(ctx, `
		SELECT
			c.challenge_id, c.status, c.message,
			c.deadline_hours, c.created_at, c.deadline_at,
			c.completed_at, c.points_creator, c.points_target,
			cu.user_id, cu.username, cu.badge_level,
			tu.user_id, tu.username, tu.badge_level,
			p.place_id, p.name, p.category,
			c.creator_user_id
		FROM challenges c
		INNER JOIN users  cu ON cu.user_id  = c.creator_user_id
		INNER JOIN users  tu ON tu.user_id  = c.target_user_id
		INNER JOIN places p  ON p.place_id  = c.place_id
		WHERE (c.creator_user_id = $1 OR c.target_user_id = $1)
		`+statusClause+`
		ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.MyChallenge
	for rows.Next() {
		var mc domain.MyChallenge
		var creatorID uuid.UUID
		if err := rows.Scan(
			&mc.ChallengeID, &mc.Status, &mc.Message,
			&mc.DeadlineHours, &mc.CreatedAt, &mc.DeadlineAt,
			&mc.CompletedAt, &mc.PointsCreator, &mc.PointsTarget,
			&mc.Creator.UserID, &mc.Creator.Username, &mc.Creator.BadgeLevel,
			&mc.Target.UserID, &mc.Target.Username, &mc.Target.BadgeLevel,
			&mc.Place.PlaceID, &mc.Place.Name, &mc.Place.Category,
			&creatorID,
		); err != nil {
			return nil, err
		}
		if creatorID == userID {
			mc.Role = "creator"
		} else {
			mc.Role = "target"
		}
		results = append(results, mc)
	}
	return results, rows.Err()
}

// CompleteMatchingChallenges finds all ACCEPTED challenges for (targetUser, place) within
// deadline, marks them COMPLETED, and awards bonus points to both participants in a TX.
func (r *ChallengeRepository) CompleteMatchingChallenges(ctx context.Context, targetUserID, placeID, checkinID uuid.UUID) error {
	type match struct {
		challengeID uuid.UUID
		creatorID   uuid.UUID
	}

	rows, err := r.db.Query(ctx, `
		SELECT challenge_id, creator_user_id FROM challenges
		WHERE target_user_id = $1
		  AND place_id       = $2
		  AND status         = 'ACCEPTED'
		  AND deadline_at    > NOW()
	`, targetUserID, placeID)
	if err != nil {
		return err
	}

	var matches []match
	for rows.Next() {
		var m match
		if err := rows.Scan(&m.challengeID, &m.creatorID); err != nil {
			rows.Close()
			return err
		}
		matches = append(matches, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(matches) == 0 {
		return nil
	}

	const ptCreator = 8
	const ptTarget = 10

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, m := range matches {
		if _, err = tx.Exec(ctx, `
			UPDATE challenges SET
				status               = 'COMPLETED',
				completed_at         = NOW(),
				completing_checkin_id = $1,
				points_creator       = $2,
				points_target        = $3
			WHERE challenge_id = $4
		`, checkinID, ptCreator, ptTarget, m.challengeID); err != nil {
			return err
		}

		if _, err = tx.Exec(ctx, `
			UPDATE users SET points_total = points_total + $1, points_season = points_season + $1, updated_at = NOW()
			WHERE user_id = $2
		`, ptCreator, m.creatorID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO point_transactions (tx_id, user_id, points, reason, reference_id)
			VALUES (uuid_generate_v4(), $1, $2, 'CHALLENGE', $3)
		`, m.creatorID, ptCreator, m.challengeID); err != nil {
			return err
		}

		if _, err = tx.Exec(ctx, `
			UPDATE users SET points_total = points_total + $1, points_season = points_season + $1, updated_at = NOW()
			WHERE user_id = $2
		`, ptTarget, targetUserID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `
			INSERT INTO point_transactions (tx_id, user_id, points, reason, reference_id)
			VALUES (uuid_generate_v4(), $1, $2, 'CHALLENGE', $3)
		`, targetUserID, ptTarget, m.challengeID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
