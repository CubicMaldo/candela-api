package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Place struct {
	PlaceID          uuid.UUID  `json:"place_id" db:"place_id"`
	Name             string     `json:"name" db:"name"`
	Description      *string    `json:"description" db:"description"`
	Category         string     `json:"category" db:"category"`
	MinorSuitability string     `json:"minor_suitability" db:"minor_suitability"`
	Lat              float64    `json:"lat"`
	Lon              float64    `json:"lon"`
	GeofenceMeters   int        `json:"geofence_meters" db:"geofence_meters"`
	BusinessID       *uuid.UUID `json:"business_id" db:"business_id"`
	IsVerified       bool       `json:"is_verified" db:"is_verified"`
	// IsActive in API responses means "en candela ahora" (recent check-in
	// activity). The places.is_active DB column is an admin enable/disable
	// flag used only in WHERE clauses — it is never exposed as-is by nearby.
	IsActive            bool      `json:"is_active" db:"is_active"`
	ActiveCheckinsCount int       `json:"active_checkins_count"`
	Distance            *float64  `json:"distance,omitempty"` // Para búsquedas por cercanía
	CreatedAt           time.Time `json:"created_at" db:"created_at"`
}

// PlaceDetail is the response for GET /api/places/:place_id.
type PlaceDetail struct {
	PlaceID             uuid.UUID  `json:"place_id"`
	Name                string     `json:"name"`
	Category            string     `json:"category"`
	Lat                 float64    `json:"lat"`
	Lon                 float64    `json:"lon"`
	GeofenceMeters      int        `json:"geofence_meters"`
	IsVerified          bool       `json:"is_verified"`
	IsActive            bool       `json:"is_active"`
	IsFollowed          bool       `json:"is_followed"`
	FollowerCount       int        `json:"follower_count"`
	ActiveCheckinsCount int        `json:"active_checkins_count"`
	LastCheckinAt       *time.Time `json:"last_checkin_at"`
}

// PlaceEventFeed is the response for GET /api/places/:place_id/feed.
type PlaceEventFeed struct {
	PlaceID    uuid.UUID      `json:"place_id"`
	PlaceName  string         `json:"place_name"`
	IsActive   bool           `json:"is_active"`
	Events     []CheckInEvent `json:"events"`
	Total      int            `json:"total"`
	NextCursor *time.Time     `json:"next_cursor,omitempty"`
}

// Payload para que el mapa (frontend) pueda enviar sus coordenadas centrales
type SearchPlacesRequest struct {
	Lat      float64 `form:"lat" binding:"required"`
	Lon      float64 `form:"lon" binding:"required"`
	RadiusKM float64 `form:"radius_km,default=5.0"`
	Category string  `form:"category"`
}

const (
	MinGeofenceMeters = 20
	MaxGeofenceMeters = 150

	DefaultUserGeofenceMeters     = 50
	MaxUserGeofenceMeters         = 80
	DefaultBusinessGeofenceMeters = 80
	MaxBusinessGeofenceMeters     = 120
	DefaultEventGeofenceMeters    = 100
	MaxEventGeofenceMeters        = 150
)

var (
	ErrGeofenceOutOfRange   = errors.New("El radio de geocerca debe estar entre 20 y 150 metros.")
	ErrGeofenceExceedsLimit = errors.New("El radio de geocerca supera el límite permitido para este tipo de organizador.")
)

// ValidateGeofence valida que el radio de geocerca cumpla con los límites globales (20-150m)
// y los topes diferenciados según el tipo de organizador (usuarios hasta 80m, empresas hasta 120m, eventos hasta 150m).
func ValidateGeofence(meters int, isBusiness bool, category string) error {
	if meters < MinGeofenceMeters || meters > MaxGeofenceMeters {
		return ErrGeofenceOutOfRange
	}
	if category == "EVENT" {
		if meters > MaxEventGeofenceMeters {
			return ErrGeofenceExceedsLimit
		}
		return nil
	}
	if isBusiness {
		if meters > MaxBusinessGeofenceMeters {
			return ErrGeofenceExceedsLimit
		}
		return nil
	}
	if meters > MaxUserGeofenceMeters {
		return ErrGeofenceExceedsLimit
	}
	return nil
}
