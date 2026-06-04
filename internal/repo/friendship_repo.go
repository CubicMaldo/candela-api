package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/nomad-api/internal/domain"
)

var (
	ErrNotBlocked            = errors.New("no block relationship found")
	ErrFriendRequestNotFound = errors.New("friend request not found")
	ErrNotFriends            = errors.New("not friends")
)

type FriendshipRepository struct {
	db *pgxpool.Pool
}

func NewFriendshipRepository(db *pgxpool.Pool) *FriendshipRepository {
	return &FriendshipRepository{db: db}
}

// orderedPair returns (smaller, larger) UUIDs respecting the user_id_a < user_id_b PK constraint.
func orderedPair(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if a.String() < b.String() {
		return a, b
	}
	return b, a
}

func (r *FriendshipRepository) AreFriends(ctx context.Context, a, b uuid.UUID) (bool, error) {
	lo, hi := orderedPair(a, b)
	var status string
	err := r.db.QueryRow(ctx, `
		SELECT status FROM friendships
		WHERE user_id_a = $1 AND user_id_b = $2
	`, lo, hi).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return status == "accepted", nil
}

func (r *FriendshipRepository) IsBlocked(ctx context.Context, a, b uuid.UUID) (bool, error) {
	lo, hi := orderedPair(a, b)
	var status string
	err := r.db.QueryRow(ctx, `
		SELECT status FROM friendships
		WHERE user_id_a = $1 AND user_id_b = $2
	`, lo, hi).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return status == "blocked", nil
}

// GetFriendship returns the current status and requester_id for the pair.
// Returns ErrFriendRequestNotFound if no row exists.
func (r *FriendshipRepository) GetFriendship(ctx context.Context, a, b uuid.UUID) (string, uuid.UUID, error) {
	lo, hi := orderedPair(a, b)
	var status string
	var requesterID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT status, requester_id FROM friendships
		WHERE user_id_a = $1 AND user_id_b = $2
	`, lo, hi).Scan(&status, &requesterID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", uuid.Nil, ErrFriendRequestNotFound
		}
		return "", uuid.Nil, err
	}
	return status, requesterID, nil
}

func (r *FriendshipRepository) Block(ctx context.Context, requesterID, targetID uuid.UUID) error {
	lo, hi := orderedPair(requesterID, targetID)
	_, err := r.db.Exec(ctx, `
		INSERT INTO friendships (user_id_a, user_id_b, status, requester_id)
		VALUES ($1, $2, 'blocked', $3)
		ON CONFLICT (user_id_a, user_id_b) DO UPDATE SET status = 'blocked', requester_id = $3
	`, lo, hi, requesterID)
	return err
}

func (r *FriendshipRepository) Unblock(ctx context.Context, requesterID, targetID uuid.UUID) error {
	lo, hi := orderedPair(requesterID, targetID)
	tag, err := r.db.Exec(ctx, `
		DELETE FROM friendships
		WHERE user_id_a = $1 AND user_id_b = $2 AND status = 'blocked'
	`, lo, hi)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotBlocked
	}
	return nil
}

func (r *FriendshipRepository) SendRequest(ctx context.Context, senderID, recipientID uuid.UUID) error {
	lo, hi := orderedPair(senderID, recipientID)
	_, err := r.db.Exec(ctx, `
		INSERT INTO friendships (user_id_a, user_id_b, status, requester_id)
		VALUES ($1, $2, 'pending', $3)
	`, lo, hi, senderID)
	return err
}

func (r *FriendshipRepository) AcceptRequest(ctx context.Context, a, b uuid.UUID) error {
	lo, hi := orderedPair(a, b)
	tag, err := r.db.Exec(ctx, `
		UPDATE friendships SET status = 'accepted'
		WHERE user_id_a = $1 AND user_id_b = $2 AND status = 'pending'
	`, lo, hi)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrFriendRequestNotFound
	}
	return nil
}

func (r *FriendshipRepository) RejectRequest(ctx context.Context, a, b uuid.UUID) error {
	lo, hi := orderedPair(a, b)
	tag, err := r.db.Exec(ctx, `
		DELETE FROM friendships
		WHERE user_id_a = $1 AND user_id_b = $2 AND status = 'pending'
	`, lo, hi)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrFriendRequestNotFound
	}
	return nil
}

func (r *FriendshipRepository) CancelRequest(ctx context.Context, senderID, recipientID uuid.UUID) error {
	lo, hi := orderedPair(senderID, recipientID)
	tag, err := r.db.Exec(ctx, `
		DELETE FROM friendships
		WHERE user_id_a = $1 AND user_id_b = $2 AND status = 'pending' AND requester_id = $3
	`, lo, hi, senderID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrFriendRequestNotFound
	}
	return nil
}

func (r *FriendshipRepository) RemoveFriend(ctx context.Context, a, b uuid.UUID) error {
	lo, hi := orderedPair(a, b)
	tag, err := r.db.Exec(ctx, `
		DELETE FROM friendships
		WHERE user_id_a = $1 AND user_id_b = $2 AND status = 'accepted'
	`, lo, hi)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFriends
	}
	return nil
}

// GetIncomingRequests returns pending requests where the authenticated user is the recipient.
func (r *FriendshipRepository) GetIncomingRequests(ctx context.Context, userID uuid.UUID) ([]domain.FriendRequestEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.user_id, u.username, u.badge_level, f.created_at
		FROM friendships f
		INNER JOIN users u ON u.user_id = f.requester_id
		WHERE (f.user_id_a = $1 OR f.user_id_b = $1)
		  AND f.status = 'pending'
		  AND f.requester_id != $1
		  AND u.account_status = 'ACTIVE'
		ORDER BY f.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.FriendRequestEntry
	for rows.Next() {
		var e domain.FriendRequestEntry
		if err := rows.Scan(&e.UserID, &e.Username, &e.BadgeLevel, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetOutgoingRequests returns pending requests sent by the authenticated user.
func (r *FriendshipRepository) GetOutgoingRequests(ctx context.Context, userID uuid.UUID) ([]domain.FriendRequestEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.user_id, u.username, u.badge_level, f.created_at
		FROM friendships f
		INNER JOIN users u ON u.user_id = (
			CASE WHEN f.user_id_a = $1 THEN f.user_id_b ELSE f.user_id_a END
		)
		WHERE (f.user_id_a = $1 OR f.user_id_b = $1)
		  AND f.status = 'pending'
		  AND f.requester_id = $1
		  AND u.account_status = 'ACTIVE'
		ORDER BY f.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.FriendRequestEntry
	for rows.Next() {
		var e domain.FriendRequestEntry
		if err := rows.Scan(&e.UserID, &e.Username, &e.BadgeLevel, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetFriendsList returns all accepted friends ordered alphabetically.
func (r *FriendshipRepository) GetFriendsList(ctx context.Context, userID uuid.UUID) ([]domain.FriendEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.user_id, u.username, u.badge_level, u.points_total
		FROM friendships f
		INNER JOIN users u ON u.user_id = (
			CASE WHEN f.user_id_a = $1 THEN f.user_id_b ELSE f.user_id_a END
		)
		WHERE (f.user_id_a = $1 OR f.user_id_b = $1)
		  AND f.status = 'accepted'
		  AND u.account_status = 'ACTIVE'
		ORDER BY u.username ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.FriendEntry
	for rows.Next() {
		var e domain.FriendEntry
		if err := rows.Scan(&e.UserID, &e.Username, &e.BadgeLevel, &e.PointsTotal); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
