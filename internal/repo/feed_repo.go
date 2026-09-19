package repo

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/candela-api/internal/domain"
)

type FeedRepository struct {
	db *pgxpool.Pool
}

func NewFeedRepository(db *pgxpool.Pool) *FeedRepository {
	return &FeedRepository{db: db}
}

// GetFeedCheckIns returns the three-layer feed for a user, newest first within each layer:
//
//	layer 1 = own active check-ins + accepted friends with active check-ins (last 4h)
//	layer 2 = followed places with active check-ins (last 4h)
//	layer 3 = general city activity (last 4h, non-PRIVATE users)
//
// Check-ins blocked in either direction are excluded from every layer.
//
// A check-in can qualify for several layers (a friend at a place you follow).
// DISTINCT ON collapses those to the lowest — highest-priority — layer inside
// the query. Deduplicating in Go instead would only work within a page: the
// same check-in would reappear under a later layer on a later page, and the
// cursor would have no stable position to resume from.
func (r *FeedRepository) GetFeedCheckIns(ctx context.Context, userID uuid.UUID, limit int, cursor *domain.FeedCursor) ([]domain.CheckInEvent, error) {
	query := `
		WITH blocked_users AS (
			SELECT CASE WHEN user_id_a = $1 THEN user_id_b ELSE user_id_a END AS blocked_id
			FROM friendships
			WHERE (user_id_a = $1 OR user_id_b = $1) AND status = 'blocked'
		), user_follows AS (
			SELECT place_id FROM follows WHERE follower_id = $1
		), user_friends AS (
			SELECT CASE WHEN user_id_a = $1 THEN user_id_b ELSE user_id_a END AS friend_id
			FROM friendships
			WHERE (user_id_a = $1 OR user_id_b = $1) AND status = 'accepted'
		)
		SELECT * FROM (
		SELECT DISTINCT ON (checkin_id) * FROM (
			SELECT
				ci.checkin_id, ci.checked_in_at,
				u.user_id, u.username, u.badge_level,
				p.place_id, p.name, p.category,
				CASE WHEN ci.photo_expires_at IS NOT NULL AND ci.photo_expires_at < NOW()
				          AND ci.photo_is_permanent = false
				     THEN NULL ELSE ci.photo_url END AS photo_url,
				ci.photo_is_permanent,
				TRUE AS is_friend,
				fol.place_id IS NOT NULL AS is_followed,
				1 AS priority_layer
			FROM check_ins ci
			INNER JOIN users u ON u.user_id = ci.user_id
			INNER JOIN places p ON p.place_id = ci.place_id
			INNER JOIN user_friends uf ON uf.friend_id = ci.user_id
			LEFT JOIN user_follows fol ON fol.place_id = ci.place_id
			WHERE ci.checked_in_at > NOW() - INTERVAL '4 hours'
			  AND NOT EXISTS (SELECT 1 FROM blocked_users bu WHERE bu.blocked_id = ci.user_id)

			UNION ALL

			-- The viewer's own active check-ins: part of "Tu gente" so people
			-- see their own presence reflected in the city scroll.
			SELECT
				ci.checkin_id, ci.checked_in_at,
				u.user_id, u.username, u.badge_level,
				p.place_id, p.name, p.category,
				CASE WHEN ci.photo_expires_at IS NOT NULL AND ci.photo_expires_at < NOW()
				          AND ci.photo_is_permanent = false
				     THEN NULL ELSE ci.photo_url END AS photo_url,
				ci.photo_is_permanent,
				FALSE AS is_friend,
				fol.place_id IS NOT NULL AS is_followed,
				1 AS priority_layer
			FROM check_ins ci
			INNER JOIN users u ON u.user_id = ci.user_id
			INNER JOIN places p ON p.place_id = ci.place_id
			LEFT JOIN user_follows fol ON fol.place_id = ci.place_id
			WHERE ci.user_id = $1
			  AND ci.checked_in_at > NOW() - INTERVAL '4 hours'

			UNION ALL

			SELECT
				ci.checkin_id, ci.checked_in_at,
				u.user_id, u.username, u.badge_level,
				p.place_id, p.name, p.category,
				CASE WHEN ci.photo_expires_at IS NOT NULL AND ci.photo_expires_at < NOW()
				          AND ci.photo_is_permanent = false
				     THEN NULL ELSE ci.photo_url END AS photo_url,
				ci.photo_is_permanent,
				uf.friend_id IS NOT NULL AS is_friend,
				TRUE AS is_followed,
				2 AS priority_layer
			FROM check_ins ci
			INNER JOIN users u ON u.user_id = ci.user_id
			INNER JOIN places p ON p.place_id = ci.place_id
			INNER JOIN user_follows fol ON fol.place_id = ci.place_id
			LEFT JOIN user_friends uf ON uf.friend_id = ci.user_id
			WHERE ci.checked_in_at > NOW() - INTERVAL '4 hours'
			  AND ci.user_id != $1
			  AND NOT EXISTS (SELECT 1 FROM blocked_users bu WHERE bu.blocked_id = ci.user_id)

			UNION ALL

			SELECT
				ci.checkin_id, ci.checked_in_at,
				u.user_id, u.username, u.badge_level,
				p.place_id, p.name, p.category,
				CASE WHEN ci.photo_expires_at IS NOT NULL AND ci.photo_expires_at < NOW()
				          AND ci.photo_is_permanent = false
				     THEN NULL ELSE ci.photo_url END AS photo_url,
				ci.photo_is_permanent,
				uf.friend_id IS NOT NULL AS is_friend,
				fol.place_id IS NOT NULL AS is_followed,
				3 AS priority_layer
			FROM check_ins ci
			INNER JOIN users u ON u.user_id = ci.user_id
			INNER JOIN places p ON p.place_id = ci.place_id
			LEFT JOIN user_friends uf ON uf.friend_id = ci.user_id
			LEFT JOIN user_follows fol ON fol.place_id = ci.place_id
			WHERE ci.checked_in_at > NOW() - INTERVAL '4 hours'
			  AND u.privacy_mode != 'PRIVATE'
			  AND ci.user_id != $1
			  AND NOT EXISTS (SELECT 1 FROM blocked_users bu WHERE bu.blocked_id = ci.user_id)
		) all_layers
		ORDER BY checkin_id, priority_layer ASC
		) feed
	`
	args := []interface{}{userID}

	// Resume strictly after the cursor position in the feed's own ordering
	// (layer ascending, then time descending, then id descending as a
	// tie-break so events sharing a timestamp are never skipped or repeated).
	if cursor != nil {
		query += `
			WHERE priority_layer > $2
			   OR (priority_layer = $2
			       AND (checked_in_at < $3
			            OR (checked_in_at = $3 AND checkin_id < $4)))
		`
		args = append(args, cursor.Layer, cursor.CheckedInAt, cursor.EventID)
	}

	query += ` ORDER BY priority_layer ASC, checked_in_at DESC, checkin_id DESC
	           LIMIT $` + strconv.Itoa(len(args)+1)
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
			&e.PhotoURL, &e.PhotoIsPermanent,
			&e.User.IsFriend, &e.Place.IsFollowed,
			&e.PriorityLayer,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
