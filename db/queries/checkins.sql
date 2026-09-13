-- name: IsWithinGeofence :one
SELECT ST_DWithin(
	location,
	ST_MakePoint($1, $2)::geography,
	geofence_meters
)
FROM places
WHERE place_id = $3 AND is_active = true;

-- name: IsRestrictedForMinors :one
SELECT minor_suitability FROM places WHERE place_id = $1;

-- name: HasCheckedInToday :one
SELECT EXISTS (
	SELECT 1 FROM check_ins
	WHERE user_id = $1
	  AND place_id = $2
	  AND validation_passed = true
	  AND (checked_in_at AT TIME ZONE sqlc.arg('tz')::text)::date = (NOW() AT TIME ZONE sqlc.arg('tz')::text)::date
);

-- name: IsFirstCheckInToday :one
SELECT EXISTS (
	SELECT 1 FROM check_ins
	WHERE user_id = $1
	  AND validation_passed = true
	  AND (checked_in_at AT TIME ZONE sqlc.arg('tz')::text)::date = (NOW() AT TIME ZONE sqlc.arg('tz')::text)::date
);

-- name: CreateCheckIn :one
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
RETURNING checkin_id, checked_in_at;

-- name: SetPointsAwarded :execrows
UPDATE check_ins SET points_awarded = $1 WHERE checkin_id = $2;

-- name: DeleteCheckIn :exec
DELETE FROM check_ins WHERE checkin_id = $1;

-- name: CheckInExists :one
SELECT COUNT(*) FROM check_ins WHERE checkin_id = $1;

-- name: GetCheckInByID :one
SELECT checkin_id, user_id, place_id, photo_is_permanent, checked_in_at
FROM check_ins
WHERE checkin_id = $1;

-- name: SetPhotoPermanent :execrows
UPDATE check_ins SET photo_is_permanent = $1 WHERE checkin_id = $2;

-- name: GetActiveCheckinsCount :one
SELECT COUNT(*) FROM check_ins
WHERE place_id = $1 AND checked_in_at > NOW() - INTERVAL '4 hours';

-- name: GetLastCheckinAt :one
SELECT MAX(checked_in_at) FROM check_ins
WHERE place_id = $1 AND validation_passed = true;

-- name: ClaimExpiredPhotos :many
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
SELECT photo_key FROM claimed;
