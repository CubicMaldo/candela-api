package domain

import (
	"time"

	"github.com/google/uuid"
)

type CheckIn struct {
	CheckInID uuid.UUID `json:"checkin_id"        db:"checkin_id"`
	UserID    uuid.UUID `json:"user_id"           db:"user_id"`
	PlaceID   uuid.UUID `json:"place_id"          db:"place_id"`
	PhotoURL  *string   `json:"photo_url"         db:"photo_url"`
	// PhotoKey is the object-storage key behind PhotoURL. Kept so the photo can
	// be deleted (account deletion, expiry sweep) without parsing it back out
	// of a signed URL. Never exposed to clients.
	PhotoKey         *string    `json:"-"                 db:"photo_key"`
	PhotoExpiresAt   *time.Time `json:"photo_expires_at"  db:"photo_expires_at"`
	PhotoIsPermanent bool       `json:"photo_is_permanent" db:"photo_is_permanent"`
	// PhotoExifTS is the capture time the client reported. It is a fraud signal
	// recorded for review, never a gate — see TimestampValid.
	PhotoExifTS      *time.Time `json:"-"                 db:"photo_exif_ts"`
	TimestampValid   bool       `json:"-"                 db:"timestamp_valid"`
	// Lat/Lon are the coordinates the server validated at Initiate time.
	// Stored as GEOGRAPHY(POINT, 4326) in the database; the repo converts.
	Lat              float64    `json:"lat"               db:"lat"`
	Lon              float64    `json:"lon"               db:"lon"`
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
	CheckInNonce string `json:"checkin_nonce"    binding:"required"`
	PhotoKey     string `json:"photo_key"        binding:"required"`
	// ExifTimestamp is the capture time as the device reported it. Optional and
	// advisory: liveness is proven by the 90-second server-side nonce, not by
	// the client's clock. Recorded as a fraud signal only.
	ExifTimestamp    time.Time `json:"exif_timestamp"`
	PhotoIsPermanent bool      `json:"photo_is_permanent"`
}

// PermanentUpdateRequest is sent to mark or unmark a check-in photo as permanent.
type PermanentUpdateRequest struct {
	Permanent bool `json:"permanent"`
}

// PointsBreakdown explains how the total points awarded for a check-in were calculated.
type PointsBreakdown struct {
	Base            int     `json:"base"`
	Multiplier      float64 `json:"multiplier"`
	SubtotalCheckin int     `json:"subtotal_checkin"`
	WeeklyBonus     int     `json:"weekly_bonus"`
	PlanBonus       int     `json:"plan_bonus"`
	Total           int     `json:"total"`
}

// CheckInResult is returned after a successful confirmation.
type CheckInResult struct {
	CheckIn         CheckIn         `json:"checkin"`
	PointsAwarded   int             `json:"points_awarded"`
	IsFirstOfDay    bool            `json:"is_first_of_day"`
	PhotoViewURL    string          `json:"photo_view_url"` // presigned GET URL valid 24h
	PointsBreakdown PointsBreakdown `json:"points_breakdown"`
}
