package repo

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/candela-api/internal/domain"
)

var ErrCheckInNotFound = errors.New("check-in not found")

// ErrDuplicateCheckIn is returned when the unique index
// idx_checkins_one_per_place_per_day rejects a second validated check-in by the
// same user, at the same place, on the same local day. The service-level check
// in Initiate catches the ordinary case; this covers the race where concurrent
// requests all pass that check before any row exists.
var ErrDuplicateCheckIn = errors.New("already checked in here today")

type CheckInRepository struct {
	db *pgxpool.Pool
	// tz is the IANA zone that defines a calendar day for the check-in rules.
	// Passed as a query parameter rather than interpolated, so it can never
	// reach the SQL text. Both day queries filter by user_id (and place_id)
	// first, so the surviving row set is tiny and no expression index is needed.
	tz string
}

func NewCheckInRepository(db *pgxpool.Pool, timezone string) *CheckInRepository {
	return &CheckInRepository{db: db, tz: timezone}
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
// at this place on the current *local* calendar day. Comparing in UTC would
// roll the day over at 7pm Colombian time — mid-evening, when the app is
// busiest — resetting the one-per-place rule in the middle of a night out.
func (r *CheckInRepository) HasCheckedInToday(ctx context.Context, userID, placeID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM check_ins
			WHERE user_id = $1
			  AND place_id = $2
			  AND validation_passed = true
			  AND (checked_in_at AT TIME ZONE $3)::date = (NOW() AT TIME ZONE $3)::date
		)
	`, userID, placeID, r.tz).Scan(&exists)
	return exists, err
}

// IsFirstCheckInToday returns true if the user has no validated check-ins
// anywhere on the current local calendar day (see HasCheckedInToday).
func (r *CheckInRepository) IsFirstCheckInToday(ctx context.Context, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM check_ins
			WHERE user_id = $1
			  AND validation_passed = true
			  AND (checked_in_at AT TIME ZONE $2)::date = (NOW() AT TIME ZONE $2)::date
		)
	`, userID, r.tz).Scan(&exists)
	return !exists, err
}

// Create inserts a new check-in and populates CheckInID and CheckedInAt from
// RETURNING. Returns ErrDuplicateCheckIn if this user already has a validated
// check-in at this place today.
func (r *CheckInRepository) Create(ctx context.Context, c *domain.CheckIn) error {
	err := r.create(ctx, c)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		pgErr.ConstraintName == "idx_checkins_one_per_place_per_day" {
		return ErrDuplicateCheckIn
	}
	return err
}

func (r *CheckInRepository) create(ctx context.Context, c *domain.CheckIn) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO check_ins (
			checkin_id, user_id, place_id,
			photo_url, photo_key, photo_expires_at, photo_is_permanent,
			photo_exif_ts, timestamp_valid,
			location,
			gps_valid, validation_passed,
			points_awarded, is_first_of_day,
			checked_in_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7,
			$8, $9,
			ST_MakePoint($10, $11)::geography,
			$12, $13,
			$14, $15,
			NOW()
		)
		RETURNING checkin_id, checked_in_at
	`,
		c.CheckInID, c.UserID, c.PlaceID,
		c.PhotoURL, c.PhotoKey, c.PhotoExpiresAt, c.PhotoIsPermanent,
		c.PhotoExifTS, c.TimestampValid,
		c.Lon, c.Lat,
		c.GpsValid, c.ValidationPassed,
		c.PointsAwarded, c.IsFirstOfDay,
	).Scan(&c.CheckInID, &c.CheckedInAt)
}

// SetPointsAwarded records the points a check-in earned. The row is inserted
// with zero so that its unique index settles same-day races before any counter
// moves; the real figure depends on the streak multiplier computed afterwards.
func (r *CheckInRepository) SetPointsAwarded(ctx context.Context, checkinID uuid.UUID, points int) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE check_ins SET points_awarded = $1 WHERE checkin_id = $2
	`, points, checkinID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCheckInNotFound
	}
	return nil
}

// Delete removes a check-in row. Used to roll back a check-in whose follow-up
// steps failed, so the user is not left with a claimed day they cannot retry.
func (r *CheckInRepository) Delete(ctx context.Context, checkinID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM check_ins WHERE checkin_id = $1`, checkinID)
	return err
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
		    badge_level   = `+badgeCaseSQL("points_total + $1")+`,
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
			ci.photo_is_permanent,
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

	query += ` ORDER BY ci.checked_in_at DESC LIMIT $` + strconv.Itoa(len(args)+1)
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
			&c.PhotoURL, &c.PhotoIsPermanent,
			&c.PointsAwarded, &c.IsFirstOfDay, &c.CheckedInAt,
		); err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

// ApplyStreak reads the user's streak counters, folds one check-in into them
// via `apply`, and persists the result — all inside a single transaction that
// holds a row lock on the user. Two concurrent check-ins by the same user
// therefore serialise instead of overwriting each other's counters.
//
// `apply` must be pure (see domain.ApplyCheckInToStreak): it runs while the
// row lock is held, so it must not perform I/O.
//
// badge_level is not touched here: it is recomputed wherever points change
// (UpdateUserPoints, CompleteMatchingPlans), the only time it can move.
func (r *CheckInRepository) ApplyStreak(
	ctx context.Context,
	userID uuid.UUID,
	apply func(domain.StreakState) (domain.StreakState, float64, int),
) (multiplier float64, weeklyBonus int, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	var current domain.StreakState
	err = tx.QueryRow(ctx, `
		SELECT streak_weeks_current, streak_weeks_max, streak_week_checkins,
		       streak_week_start, streak_last_checkin
		FROM users
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(
		&current.WeeksCurrent, &current.WeeksMax, &current.WeekCheckins,
		&current.WeekStart, &current.LastCheckin,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, 0, ErrUserNotFound
		}
		return 0, 0, err
	}

	next, multiplier, weeklyBonus := apply(current)

	if _, err = tx.Exec(ctx, `
		UPDATE users SET
		  streak_weeks_current = $2,
		  streak_weeks_max     = $3,
		  streak_week_checkins = $4,
		  streak_week_start    = $5,
		  streak_last_checkin  = $6,
		  updated_at = NOW()
		WHERE user_id = $1
	`,
		userID,
		next.WeeksCurrent,
		next.WeeksMax,
		next.WeekCheckins,
		next.WeekStart,
		next.LastCheckin,
	); err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return multiplier, weeklyBonus, nil
}

// GetEventFeed returns check-ins for a specific place within the last 4 hours, newest first.
// Cursor-based: pass before != nil to get events older than that timestamp.
func (r *CheckInRepository) GetEventFeed(ctx context.Context, placeID, requesterID uuid.UUID, limit int, before *time.Time) ([]domain.CheckInEvent, error) {
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
			EXISTS (
				SELECT 1 FROM friendships f2
				WHERE ((f2.user_id_a = $2 AND f2.user_id_b = ci.user_id)
				    OR (f2.user_id_b = $2 AND f2.user_id_a = ci.user_id))
				  AND f2.status = 'accepted'
			) AS is_friend
		FROM check_ins ci
		INNER JOIN users u ON u.user_id = ci.user_id
		INNER JOIN places p ON p.place_id = ci.place_id
		WHERE ci.place_id = $1
		  AND ci.checked_in_at > NOW() - INTERVAL '4 hours'
	`
	args := []interface{}{placeID, requesterID}

	if before != nil {
		query += ` AND ci.checked_in_at < $3`
		args = append(args, *before)
	}

	query += ` ORDER BY ci.checked_in_at DESC LIMIT $` + strconv.Itoa(len(args)+1)
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
			&e.User.IsFriend,
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

// ClaimExpiredPhotos hands the caller up to `limit` object keys belonging to
// ephemeral photos whose viewing window has closed, and clears them from the
// row in the same statement.
//
// Clearing and returning atomically is what makes the sweep safe to run from
// more than one API instance: a key is handed out exactly once, so two
// instances never race to delete the same object, and a crash mid-sweep leaks
// at most one batch instead of re-deleting forever.
func (r *CheckInRepository) ClaimExpiredPhotos(ctx context.Context, limit int) ([]string, error) {
	// The keys must be read in a CTE: `UPDATE ... RETURNING photo_key` would
	// return the value just written (NULL), not the key being retired.
	rows, err := r.db.Query(ctx, `
		WITH claimed AS (
			SELECT checkin_id, photo_key FROM check_ins
			WHERE photo_is_permanent = false
			  AND photo_key IS NOT NULL
			  AND photo_expires_at IS NOT NULL
			  AND photo_expires_at < NOW()
			ORDER BY photo_expires_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		), cleared AS (
			UPDATE check_ins c
			SET photo_key = NULL, photo_url = NULL
			FROM claimed
			WHERE c.checkin_id = claimed.checkin_id
		)
		SELECT photo_key FROM claimed
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}
