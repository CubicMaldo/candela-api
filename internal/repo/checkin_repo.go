package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/nomad-api/internal/domain"
)

type CheckInRepository struct {
	db *pgxpool.Pool
}

func NewCheckInRepository(db *pgxpool.Pool) *CheckInRepository {
	return &CheckInRepository{db: db}
}

// IsWithinGeofence returns true if (lat, lon) is within the place's geofence radius.
// ST_MakePoint takes (longitude, latitude) — order matters for PostGIS.
func (r *CheckInRepository) IsWithinGeofence(ctx context.Context, placeID uuid.UUID, lat, lon float64) (bool, error) {
	var within bool
	err := r.db.QueryRow(ctx, `
		SELECT ST_DWithin(
			location,
			ST_MakePoint($1, $2)::geography,
			geofence_meters
		)
		FROM places
		WHERE place_id = $3 AND is_active = true
	`, lon, lat, placeID).Scan(&within)
	return within, err
}

// IsRestrictedForMinors returns true if the place is not suitable for minors.
// PENDING is treated as restricted (same as NO).
func (r *CheckInRepository) IsRestrictedForMinors(ctx context.Context, placeID uuid.UUID) (bool, error) {
	var suitability string
	err := r.db.QueryRow(ctx, `
		SELECT minor_suitability FROM places WHERE place_id = $1
	`, placeID).Scan(&suitability)
	if err != nil {
		return true, err
	}
	return suitability != "YES", nil
}

// HasCheckedInToday returns true if the user already has a validated check-in
// at this place on the current UTC calendar day.
func (r *CheckInRepository) HasCheckedInToday(ctx context.Context, userID, placeID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM check_ins
		WHERE user_id = $1
		  AND place_id = $2
		  AND validation_passed = true
		  AND checked_in_at::date = NOW()::date
	`, userID, placeID).Scan(&count)
	return count > 0, err
}

// IsFirstCheckInToday returns true if the user has no validated check-ins anywhere today.
func (r *CheckInRepository) IsFirstCheckInToday(ctx context.Context, userID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM check_ins
		WHERE user_id = $1
		  AND validation_passed = true
		  AND checked_in_at::date = NOW()::date
	`, userID).Scan(&count)
	return count == 0, err
}

// Create inserts a new check-in and populates CheckInID and CheckedInAt from RETURNING.
func (r *CheckInRepository) Create(ctx context.Context, c *domain.CheckIn) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO check_ins (
			checkin_id, user_id, place_id,
			photo_url, photo_expires_at,
			gps_lat, gps_lon,
			gps_valid, validation_passed,
			points_awarded, is_first_of_day,
			checked_in_at
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7,
			$8, $9,
			$10, $11,
			NOW()
		)
		RETURNING checkin_id, checked_in_at
	`,
		c.CheckInID, c.UserID, c.PlaceID,
		c.PhotoURL, c.PhotoExpiresAt,
		c.GpsLat, c.GpsLon,
		c.GpsValid, c.ValidationPassed,
		c.PointsAwarded, c.IsFirstOfDay,
	).Scan(&c.CheckInID, &c.CheckedInAt)
}

// UpdateUserPoints adds points to points_total and points_season, and records the transaction.
// Both operations run inside a single DB transaction.
func (r *CheckInRepository) UpdateUserPoints(ctx context.Context, userID uuid.UUID, points int, checkinID uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE users
		SET points_total  = points_total  + $1,
		    points_season = points_season + $1,
		    updated_at    = NOW()
		WHERE user_id = $2
	`, points, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO point_transactions (tx_id, user_id, points, reason, reference_id)
		VALUES (uuid_generate_v4(), $1, $2, 'CHECKIN', $3)
	`, userID, points, checkinID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetMyCheckIns returns the authenticated user's validated check-in history, newest first.
// Cursor-based: pass before != nil to get events older than that timestamp.
func (r *CheckInRepository) GetMyCheckIns(ctx context.Context, userID uuid.UUID, limit int, before *time.Time) ([]domain.MyCheckIn, error) {
	query := `
		SELECT
			ci.checkin_id,
			p.place_id, p.name, p.category,
			CASE WHEN ci.photo_expires_at IS NOT NULL AND ci.photo_expires_at < NOW()
			     THEN NULL ELSE ci.photo_url END AS photo_url,
			ci.points_awarded, ci.is_first_of_day, ci.checked_in_at
		FROM check_ins ci
		INNER JOIN places p ON p.place_id = ci.place_id
		WHERE ci.user_id = $1 AND ci.validation_passed = true
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

	var results []domain.MyCheckIn
	for rows.Next() {
		var c domain.MyCheckIn
		if err := rows.Scan(
			&c.CheckInID,
			&c.Place.PlaceID, &c.Place.Name, &c.Place.Category,
			&c.PhotoURL,
			&c.PointsAwarded, &c.IsFirstOfDay, &c.CheckedInAt,
		); err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

// UpdateStreakAndBadge recalculates streak counters and badge_level based on total points.
// Call after UpdateUserPoints.
func (r *CheckInRepository) UpdateStreakAndBadge(ctx context.Context, userID uuid.UUID, today time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET
		  streak_current = CASE
		    WHEN streak_last_checkin = $2::date - INTERVAL '1 day' THEN streak_current + 1
		    WHEN streak_last_checkin = $2::date                    THEN streak_current
		    ELSE 1
		  END,
		  streak_max = GREATEST(streak_max,
		    CASE
		      WHEN streak_last_checkin = $2::date - INTERVAL '1 day' THEN streak_current + 1
		      WHEN streak_last_checkin = $2::date                    THEN streak_current
		      ELSE 1
		    END
		  ),
		  streak_last_checkin = $2::date,
		  badge_level = CASE
		    WHEN points_total >= 5000 THEN 'Embajador de la Ciudad'
		    WHEN points_total >= 1000 THEN 'Referente del Barrio'
		    WHEN points_total >= 300  THEN 'Local Conocido'
		    ELSE 'Explorador Nuevo'
		  END,
		  updated_at = NOW()
		WHERE user_id = $1
	`, userID, today)
	return err
}
