package domain

import (
	"time"

	"github.com/google/uuid"
)

type Business struct {
	BusinessID   uuid.UUID `json:"business_id" db:"business_id"`
	LegalName    string    `json:"legal_name" db:"legal_name"`
	ContactEmail string    `json:"contact_email" db:"contact_email"`
	Plan         string    `json:"plan" db:"plan"`
	IsVerified   bool      `json:"is_verified" db:"is_verified"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

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
	IsActive         bool       `json:"is_active" db:"is_active"`
	Distance         *float64   `json:"distance,omitempty"` // Para búsquedas por cercanía
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
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
