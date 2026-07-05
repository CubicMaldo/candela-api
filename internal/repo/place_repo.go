package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/candela-api/internal/domain"
)

type PlaceRepository struct {
	db *pgxpool.Pool
}

func NewPlaceRepository(db *pgxpool.Pool) *PlaceRepository {
	return &PlaceRepository{db: db}
}

// IsActive returns true if the place exists and is_active = true.
func (r *PlaceRepository) IsActive(ctx context.Context, placeID uuid.UUID) (bool, error) {
	var active bool
	err := r.db.QueryRow(ctx, `
		SELECT is_active FROM places WHERE place_id = $1
	`, placeID).Scan(&active)
	if err != nil {
		return false, err
	}
	return active, nil
}

// GetByID returns a single place by its ID, including its lat/lon decoded from PostGIS.
func (r *PlaceRepository) GetByID(ctx context.Context, placeID uuid.UUID) (*domain.Place, error) {
	var p domain.Place
	err := r.db.QueryRow(ctx, `
		SELECT
			place_id, name, description, category, minor_suitability,
			ST_Y(location::geometry) as lat, ST_X(location::geometry) as lon,
			geofence_meters, business_id, is_verified, is_active, created_at
		FROM places
		WHERE place_id = $1
	`, placeID).Scan(
		&p.PlaceID, &p.Name, &p.Description, &p.Category, &p.MinorSuitability,
		&p.Lat, &p.Lon, &p.GeofenceMeters, &p.BusinessID, &p.IsVerified, &p.IsActive, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// FindNearby usa PostGIS ST_DWithin para encontrar lugares y ordenarlos por distancia
func (r *PlaceRepository) FindNearby(ctx context.Context, lat, lon float64, radiusInMeters float64, category string) ([]domain.Place, error) {
	// NOTA: PostGIS ST_MakePoint toma argumentos como (longitud, latitud)
	query := `
		SELECT 
			place_id, name, description, category, minor_suitability,
			ST_Y(location::geometry) as lat, ST_X(location::geometry) as lon,
			geofence_meters, business_id, is_verified, is_active, created_at,
			ST_Distance(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography) as distance
		FROM places
		WHERE is_active = true
		  AND ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
	`

	args := []interface{}{lon, lat, radiusInMeters}

	if category != "" {
		query += ` AND category = $4`
		args = append(args, category)
	}

	query += ` ORDER BY distance ASC LIMIT 100`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var places []domain.Place
	for rows.Next() {
		var p domain.Place
		var dist float64
		err := rows.Scan(
			&p.PlaceID, &p.Name, &p.Description, &p.Category, &p.MinorSuitability,
			&p.Lat, &p.Lon, &p.GeofenceMeters, &p.BusinessID, &p.IsVerified, &p.IsActive, &p.CreatedAt,
			&dist,
		)
		if err != nil {
			return nil, err
		}
		p.Distance = &dist
		places = append(places, p)
	}

	return places, nil
}
