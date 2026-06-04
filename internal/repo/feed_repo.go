package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/nomad-api/internal/domain"
)

type FeedRepository struct {
	db *pgxpool.Pool
}

func NewFeedRepository(db *pgxpool.Pool) *FeedRepository {
	return &FeedRepository{db: db}
}

// GetFeedCheckIns returns check-ins from accepted friends, newest first.
// Photos past their expiry are included in the event but photo_url is set to null.
// Excludes friends with privacy_mode='PRIVATE' or account_status!='ACTIVE'.
func (r *FeedRepository) GetFeedCheckIns(ctx context.Context, userID uuid.UUID, limit int, before *time.Time) ([]domain.CheckInEvent, error) {
	query := `
		SELECT
			ci.checkin_id,
			ci.checked_in_at,
			u.user_id, u.username, u.badge_level,
			p.place_id, p.name, p.category,
			CASE WHEN ci.photo_expires_at IS NOT NULL AND ci.photo_expires_at < NOW()
			     THEN NULL ELSE ci.photo_url END AS photo_url,
			ci.points_awarded
		FROM check_ins ci
		INNER JOIN users u ON u.user_id = ci.user_id
		INNER JOIN places p ON p.place_id = ci.place_id
		INNER JOIN friendships f ON (
			(f.user_id_a = $1 AND f.user_id_b = ci.user_id)
			OR (f.user_id_b = $1 AND f.user_id_a = ci.user_id)
		)
		WHERE f.status = 'accepted'
		  AND u.privacy_mode != 'PRIVATE'
		  AND u.account_status = 'ACTIVE'
		  AND ci.validation_passed = true
	`
	args := []interface{}{userID}

	if before != nil {
		query += ` AND ci.checked_in_at < $2`
		args = append(args, *before)
	}

	query += ` ORDER BY ci.checked_in_at DESC LIMIT $` + itoa(len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.CheckInEvent
	for rows.Next() {
		var e domain.CheckInEvent
		e.Type = "checkin"
		if err := rows.Scan(
			&e.EventID, &e.Timestamp,
			&e.User.UserID, &e.User.Username, &e.User.BadgeLevel,
			&e.Place.PlaceID, &e.Place.Name, &e.Place.Category,
			&e.PhotoURL,
			&e.PointsAwarded,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetFeedChallenges returns completed challenges involving at least one accepted friend,
// where both participants are ACTIVE and not PRIVATE. Newest first.
func (r *FeedRepository) GetFeedChallenges(ctx context.Context, userID uuid.UUID, limit int, before *time.Time) ([]domain.ChallengeCompletedEvent, error) {
	query := `
		SELECT
			c.challenge_id,
			c.completed_at,
			cu.user_id, cu.username, cu.badge_level,
			tu.user_id, tu.username, tu.badge_level,
			p.place_id, p.name, p.category
		FROM challenges c
		INNER JOIN users cu ON cu.user_id = c.creator_user_id
		INNER JOIN users tu ON tu.user_id = c.target_user_id
		INNER JOIN places p  ON p.place_id  = c.place_id
		WHERE c.status = 'COMPLETED'
		  AND cu.privacy_mode != 'PRIVATE' AND cu.account_status = 'ACTIVE'
		  AND tu.privacy_mode != 'PRIVATE' AND tu.account_status = 'ACTIVE'
		  AND (
			c.creator_user_id IN (
				SELECT CASE WHEN f.user_id_a = $1 THEN f.user_id_b ELSE f.user_id_a END
				FROM friendships f
				WHERE (f.user_id_a = $1 OR f.user_id_b = $1) AND f.status = 'accepted'
			)
			OR
			c.target_user_id IN (
				SELECT CASE WHEN f.user_id_a = $1 THEN f.user_id_b ELSE f.user_id_a END
				FROM friendships f
				WHERE (f.user_id_a = $1 OR f.user_id_b = $1) AND f.status = 'accepted'
			)
		  )
	`
	args := []interface{}{userID}

	if before != nil {
		query += ` AND c.completed_at < $2`
		args = append(args, *before)
	}

	query += ` ORDER BY c.completed_at DESC LIMIT $` + itoa(len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.ChallengeCompletedEvent
	for rows.Next() {
		var e domain.ChallengeCompletedEvent
		e.Type = "challenge_completed"
		if err := rows.Scan(
			&e.EventID, &e.Timestamp,
			&e.Creator.UserID, &e.Creator.Username, &e.Creator.BadgeLevel,
			&e.Target.UserID, &e.Target.Username, &e.Target.BadgeLevel,
			&e.Place.PlaceID, &e.Place.Name, &e.Place.Category,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// itoa converts an int to its decimal string representation without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
