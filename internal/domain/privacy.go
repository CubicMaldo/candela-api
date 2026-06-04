package domain

import (
	"time"

	"github.com/google/uuid"
)

type PrivacyUpdateRequest struct {
	PrivacyMode string `json:"privacy_mode" binding:"required,oneof=PUBLIC FRIENDS PRIVATE"`
}

// FullUserProfile is returned when the requester has full visibility.
type FullUserProfile struct {
	UserID        uuid.UUID `json:"user_id"`
	Username      string    `json:"username"`
	BadgeLevel    string    `json:"badge_level"`
	PointsTotal   int       `json:"points_total"`
	StreakCurrent int       `json:"streak_current"`
	StreakMax     int       `json:"streak_max"`
	CheckInCount  int       `json:"checkin_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// PartialUserProfile is returned when the requester has limited visibility.
type PartialUserProfile struct {
	Username   string `json:"username"`
	BadgeLevel string `json:"badge_level"`
}

// ProfileResult carries either a full or partial view; exactly one will be set.
type ProfileResult struct {
	Full    *FullUserProfile
	Partial *PartialUserProfile
}
