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
	UserID             uuid.UUID `json:"user_id"`
	Username           string    `json:"username"`
	BadgeLevel         string    `json:"badge_level"`
	PointsTotal        *int      `json:"points_total,omitempty"`
	StreakWeeksCurrent int       `json:"streak_weeks_current"`
	StreakWeeksMax     int       `json:"streak_weeks_max"`
	CheckInCount       int       `json:"check_in_count"`
	CreatedAt          time.Time `json:"created_at"`
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
