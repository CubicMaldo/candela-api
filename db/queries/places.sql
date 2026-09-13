-- name: IsPlaceActive :one
SELECT is_active FROM places WHERE place_id = $1;

-- name: GetPlaceByID :one
SELECT
	place_id, name, description, category, minor_suitability,
	ST_Y(location::geometry)::float8 as lat, ST_X(location::geometry)::float8 as lon,
	geofence_meters, business_id, is_verified, is_active, created_at
FROM places
WHERE place_id = $1;
