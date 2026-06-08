package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FollowRepository struct {
	db *pgxpool.Pool
}

func NewFollowRepository(db *pgxpool.Pool) *FollowRepository {
	return &FollowRepository{db: db}
}

// Follow creates a follow relationship between followerID and placeID.
// Idempotent: re-following an already-followed place is a no-op.
func (r *FollowRepository) Follow(ctx context.Context, followerID, placeID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO follows (follower_id, place_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (follower_id, place_id) DO NOTHING
	`, followerID, placeID)
	return err
}

// Unfollow removes the follow relationship, if any.
func (r *FollowRepository) Unfollow(ctx context.Context, followerID, placeID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM follows WHERE follower_id = $1 AND place_id = $2
	`, followerID, placeID)
	return err
}

// IsFollowing returns true if followerID currently follows placeID.
func (r *FollowRepository) IsFollowing(ctx context.Context, followerID, placeID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM follows WHERE follower_id = $1 AND place_id = $2
	`, followerID, placeID).Scan(&count)
	return count > 0, err
}

// GetFollowedPlaceIDs returns the IDs of every place followerID follows.
func (r *FollowRepository) GetFollowedPlaceIDs(ctx context.Context, followerID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		SELECT place_id FROM follows WHERE follower_id = $1
	`, followerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetFollowerCount returns the number of users following placeID.
func (r *FollowRepository) GetFollowerCount(ctx context.Context, placeID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM follows WHERE place_id = $1
	`, placeID).Scan(&count)
	return count, err
}
