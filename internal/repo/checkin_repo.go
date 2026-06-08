package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/nomad-api/internal/domain"
)

var ErrCheckInNotFound = errors.New("check-in not found")

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
			photo_url, photo_expires_at, photo_is_permanent,
			gps_lat, gps_lon,
			gps_valid, validation_passed,
			points_awarded, is_first_of_day,
			checked_in_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6,
			$7, $8,
			$9, $10,
			$11, $12,
			NOW()
		)
		RETURNING checkin_id, checked_in_at
	`,
		c.CheckInID, c.UserID, c.PlaceID,
		c.PhotoURL, c.PhotoExpiresAt, c.PhotoIsPermanent,
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
			          AND ci.photo_is_permanent = false
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

// UpdateStreakFields persists the user's weekly streak counters and badge_level.
// Call after updateWeeklyStreak has computed the new values on the in-memory user.
func (r *CheckInRepository) UpdateStreakFields(ctx context.Context, u *domain.User) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET
		  streak_weeks_current = $2,
		  streak_weeks_max     = $3,
		  streak_week_checkins = $4,
		  streak_week_start    = $5,
		  streak_last_checkin  = $6,
		  badge_level = CASE
		    WHEN points_total >= 5000 THEN 'Embajador de la Ciudad'
		    WHEN points_total >= 1000 THEN 'Referente del Barrio'
		    WHEN points_total >= 300  THEN 'Local Conocido'
		    ELSE 'Explorador Nuevo'
		  END,
		  updated_at = NOW()
		WHERE user_id = $1
	`,
		u.UserID,
		u.StreakWeeksCurrent,
		u.StreakWeeksMax,
		u.StreakWeekCheckins,
		u.StreakWeekStart,
		u.StreakLastCheckin,
	)
	return err
}

// GetEventFeed returns check-ins for a specific place within the last 4 hours, newest first.
// Cursor-based: pass before != nil to get events older than that timestamp.
func (r *CheckInRepository) GetEventFeed(ctx context.Context, placeID uuid.UUID, limit int, before *time.Time) ([]domain.CheckInEvent, error) {
	query := `
		SELECT
			ci.checkin_id,
			ci.checked_in_at,
			u.user_id, u.username, u.badge_level,
			p.place_id, p.name, p.category,
			CASE WHEN ci.photo_expires_at IS NOT NULL AND ci.photo_expires_at < NOW()
			          AND ci.photo_is_permanent = false
			     THEN NULL ELSE ci.photo_url END AS photo_url,
			ci.photo_is_permanent,
			ci.points_awarded
		FROM check_ins ci
		INNER JOIN users u ON u.user_id = ci.user_id
		INNER JOIN places p ON p.place_id = ci.place_id
		WHERE ci.place_id = $1
		  AND ci.checked_in_at > NOW() - INTERVAL '4 hours'
	`
	args := []interface{}{placeID}

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
			&e.PhotoURL, &e.PhotoIsPermanent,
			&e.PointsAwarded,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// Exists returns true if a check-in with the given ID exists.
// Used to validate PHOTO report targets (a photo is identified by its check-in).
func (r *CheckInRepository) Exists(ctx context.Context, checkinID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM check_ins WHERE checkin_id = $1
	`, checkinID).Scan(&count)
	return count > 0, err
}

// GetByID returns a check-in by its ID, including ownership and permanence info.
func (r *CheckInRepository) GetByID(ctx context.Context, checkinID uuid.UUID) (*domain.CheckIn, error) {
	var c domain.CheckIn
	err := r.db.QueryRow(ctx, `
		SELECT checkin_id, user_id, place_id, photo_is_permanent, checked_in_at
		FROM check_ins
		WHERE checkin_id = $1
	`, checkinID).Scan(&c.CheckInID, &c.UserID, &c.PlaceID, &c.PhotoIsPermanent, &c.CheckedInAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrCheckInNotFound
		}
		return nil, err
	}
	return &c, nil
}

// SetPhotoPermanent updates the photo_is_permanent flag for a check-in.
func (r *CheckInRepository) SetPhotoPermanent(ctx context.Context, checkinID uuid.UUID, permanent bool) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE check_ins SET photo_is_permanent = $1 WHERE checkin_id = $2
	`, permanent, checkinID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCheckInNotFound
	}
	return nil
}

// GetActiveCheckinsCount returns the number of check-ins at placeID in the last 4 hours.
func (r *CheckInRepository) GetActiveCheckinsCount(ctx context.Context, placeID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM check_ins
		WHERE place_id = $1 AND checked_in_at > NOW() - INTERVAL '4 hours'
	`, placeID).Scan(&count)
	return count, err
}

// GetLastCheckinAt returns the timestamp of the most recent validated check-in at placeID, if any.
func (r *CheckInRepository) GetLastCheckinAt(ctx context.Context, placeID uuid.UUID) (*time.Time, error) {
	var t *time.Time
	err := r.db.QueryRow(ctx, `
		SELECT MAX(checked_in_at) FROM check_ins
		WHERE place_id = $1 AND validation_passed = true
	`, placeID).Scan(&t)
	return t, err
}

// GetActivePlaceIDs returns place IDs that have at least one check-in in the last 4 hours.
func (r *CheckInRepository) GetActivePlaceIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT place_id
		FROM check_ins
		WHERE checked_in_at > NOW() - INTERVAL '4 hours'
	`)
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
