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

// GetFeedCheckIns returns the three-layer feed for a user, newest first within each layer:
//
//	layer 1 = accepted friends with active check-ins (last 4h)
//	layer 2 = followed places with active check-ins (last 4h)
//	layer 3 = general city activity (last 4h, non-PRIVATE users)
//
// Check-ins blocked in either direction are excluded from every layer.
// A check-in present in multiple layers is deduplicated in Go, keeping the
// lowest (highest-priority) layer.
func (r *FeedRepository) GetFeedCheckIns(ctx context.Context, userID uuid.UUID, limit int, before *time.Time) ([]domain.CheckInEvent, error) {
	query := `
		SELECT * FROM (
			SELECT
				ci.checkin_id, ci.checked_in_at,
				u.user_id, u.username, u.badge_level,
				p.place_id, p.name, p.category,
				CASE WHEN ci.photo_expires_at IS NOT NULL AND ci.photo_expires_at < NOW()
				          AND ci.photo_is_permanent = false
				     THEN NULL ELSE ci.photo_url END AS photo_url,
				ci.photo_is_permanent,
				ci.points_awarded,
				1 AS priority_layer
			FROM check_ins ci
			INNER JOIN users u ON u.user_id = ci.user_id
			INNER JOIN places p ON p.place_id = ci.place_id
			INNER JOIN friendships f ON (
				(f.user_id_a = $1 AND f.user_id_b = ci.user_id) OR
				(f.user_id_b = $1 AND f.user_id_a = ci.user_id)
			)
			WHERE f.status = 'accepted'
			  AND ci.checked_in_at > NOW() - INTERVAL '4 hours'
			  AND ci.user_id NOT IN (
			      SELECT CASE WHEN user_id_a = $1 THEN user_id_b ELSE user_id_a END
			      FROM friendships
			      WHERE (user_id_a = $1 OR user_id_b = $1) AND status = 'blocked'
			  )

			UNION ALL

			SELECT
				ci.checkin_id, ci.checked_in_at,
				u.user_id, u.username, u.badge_level,
				p.place_id, p.name, p.category,
				CASE WHEN ci.photo_expires_at IS NOT NULL AND ci.photo_expires_at < NOW()
				          AND ci.photo_is_permanent = false
				     THEN NULL ELSE ci.photo_url END AS photo_url,
				ci.photo_is_permanent,
				ci.points_awarded,
				2 AS priority_layer
			FROM check_ins ci
			INNER JOIN users u ON u.user_id = ci.user_id
			INNER JOIN places p ON p.place_id = ci.place_id
			INNER JOIN follows fol ON fol.place_id = ci.place_id AND fol.follower_id = $1
			WHERE ci.checked_in_at > NOW() - INTERVAL '4 hours'
			  AND ci.user_id != $1
			  AND ci.user_id NOT IN (
			      SELECT CASE WHEN user_id_a = $1 THEN user_id_b ELSE user_id_a END
			      FROM friendships
			      WHERE (user_id_a = $1 OR user_id_b = $1) AND status = 'blocked'
			  )

			UNION ALL

			SELECT
				ci.checkin_id, ci.checked_in_at,
				u.user_id, u.username, u.badge_level,
				p.place_id, p.name, p.category,
				CASE WHEN ci.photo_expires_at IS NOT NULL AND ci.photo_expires_at < NOW()
				          AND ci.photo_is_permanent = false
				     THEN NULL ELSE ci.photo_url END AS photo_url,
				ci.photo_is_permanent,
				ci.points_awarded,
				3 AS priority_layer
			FROM check_ins ci
			INNER JOIN users u ON u.user_id = ci.user_id
			INNER JOIN places p ON p.place_id = ci.place_id
			WHERE ci.checked_in_at > NOW() - INTERVAL '4 hours'
			  AND u.privacy_mode != 'PRIVATE'
			  AND ci.user_id != $1
			  AND ci.user_id NOT IN (
			      SELECT CASE WHEN user_id_a = $1 THEN user_id_b ELSE user_id_a END
			      FROM friendships
			      WHERE (user_id_a = $1 OR user_id_b = $1) AND status = 'blocked'
			  )
		) feed
	`
	args := []interface{}{userID}

	if before != nil {
		query += ` WHERE checked_in_at < $2`
		args = append(args, *before)
	}

	query += ` ORDER BY priority_layer ASC, checked_in_at DESC LIMIT $` + itoa(len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[uuid.UUID]bool)
	var events []domain.CheckInEvent
	for rows.Next() {
		var e domain.CheckInEvent
		var priorityLayer int
		e.Type = "checkin"
		if err := rows.Scan(
			&e.EventID, &e.Timestamp,
			&e.User.UserID, &e.User.Username, &e.User.BadgeLevel,
			&e.Place.PlaceID, &e.Place.Name, &e.Place.Category,
			&e.PhotoURL, &e.PhotoIsPermanent,
			&e.PointsAwarded,
			&priorityLayer,
		); err != nil {
			return nil, err
		}
		if seen[e.EventID] {
			continue
		}
		seen[e.EventID] = true
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
