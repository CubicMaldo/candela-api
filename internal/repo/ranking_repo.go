package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/nomad-api/internal/domain"
)

type RankingRepository struct {
	db *pgxpool.Pool
}

func NewRankingRepository(db *pgxpool.Pool) *RankingRepository {
	return &RankingRepository{db: db}
}

func (r *RankingRepository) GetGlobal(ctx context.Context, limit, offset int) ([]domain.RankingEntry, int, error) {
	var total int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM users
		WHERE account_status = 'ACTIVE'
		  AND privacy_mode   = 'PUBLIC'
		  AND is_protected   = false
		  AND ranking_opt_in = TRUE
	`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
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
		ORDER BY points_total DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []domain.RankingEntry
	for rows.Next() {
		var e domain.RankingEntry
		if err := rows.Scan(&e.Rank, &e.UserID, &e.Username, &e.BadgeLevel, &e.PointsTotal); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}

	return entries, total, rows.Err()
}

func (r *RankingRepository) GetUserGlobalRank(ctx context.Context, userID uuid.UUID) (*domain.RankingEntry, error) {
	var e domain.RankingEntry
	err := r.db.QueryRow(ctx, `
		SELECT rank, user_id, username, badge_level, points_total
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
		WHERE user_id = $1
	`, userID).Scan(&e.Rank, &e.UserID, &e.Username, &e.BadgeLevel, &e.PointsTotal)
	if err != nil {
		return nil, nil
	}
	return &e, nil
}

func (r *RankingRepository) GetFriends(ctx context.Context, userID uuid.UUID) ([]domain.RankingEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			ROW_NUMBER() OVER (ORDER BY u.points_total DESC) AS rank,
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
		ORDER BY u.points_total DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.RankingEntry
	for rows.Next() {
		var e domain.RankingEntry
		if err := rows.Scan(&e.Rank, &e.UserID, &e.Username, &e.BadgeLevel, &e.PointsTotal); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}

func (r *RankingRepository) GetUserFriendRank(ctx context.Context, userID uuid.UUID) (*domain.RankingEntry, error) {
	var e domain.RankingEntry
	err := r.db.QueryRow(ctx, `
		SELECT rank, user_id, username, badge_level, points_total
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
		WHERE user_id = $1
	`, userID).Scan(&e.Rank, &e.UserID, &e.Username, &e.BadgeLevel, &e.PointsTotal)
	if err != nil {
		return nil, nil
	}
	return &e, nil
}
