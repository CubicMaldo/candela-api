package domain

import (
	"time"

	"github.com/google/uuid"
)

type CheckIn struct {
	CheckInID        uuid.UUID  `json:"checkin_id"        db:"checkin_id"`
	UserID           uuid.UUID  `json:"user_id"           db:"user_id"`
	PlaceID          uuid.UUID  `json:"place_id"          db:"place_id"`
	PhotoURL         *string    `json:"photo_url"         db:"photo_url"`
	PhotoExpiresAt   *time.Time `json:"photo_expires_at"  db:"photo_expires_at"`
	GpsLat           float64    `json:"gps_lat"           db:"gps_lat"`
	GpsLon           float64    `json:"gps_lon"           db:"gps_lon"`
	GpsValid         bool       `json:"gps_valid"         db:"gps_valid"`
	ValidationPassed bool       `json:"validation_passed" db:"validation_passed"`
	PointsAwarded    int        `json:"points_awarded"    db:"points_awarded"`
	IsFirstOfDay     bool       `json:"is_first_of_day"   db:"is_first_of_day"`
	CheckedInAt      time.Time  `json:"checked_in_at"     db:"checked_in_at"`
}

// CheckInInitiateRequest is sent by the client to start a check-in.
type CheckInInitiateRequest struct {
	Lat float64 `json:"lat" binding:"min=-90,max=90"`
	Lon float64 `json:"lon" binding:"min=-180,max=180"`
}

// CheckInInitiateResponse tells the client where to upload the photo.
type CheckInInitiateResponse struct {
	CheckInNonce string `json:"checkin_nonce"` // stored in Redis with 90s TTL
	UploadURL    string `json:"upload_url"`    // presigned PUT URL
	PhotoKey     string `json:"photo_key"`     // reference for the confirm step
}

// CheckInConfirmRequest is sent after the client finishes uploading the photo.
type CheckInConfirmRequest struct {
	CheckInNonce string `json:"checkin_nonce" binding:"required"`
	PhotoKey     string `json:"photo_key"     binding:"required"`
}

// CheckInResult is returned after a successful confirmation.
type CheckInResult struct {
	CheckIn       CheckIn `json:"checkin"`
	PointsAwarded int     `json:"points_awarded"`
	IsFirstOfDay  bool    `json:"is_first_of_day"`
	PhotoViewURL  string  `json:"photo_view_url"` // presigned GET URL valid 24h
}
