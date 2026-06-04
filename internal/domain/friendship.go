package domain

import (
	"time"

	"github.com/google/uuid"
)

type FriendRequestEntry struct {
	UserID     uuid.UUID `json:"user_id"`
	Username   string    `json:"username"`
	BadgeLevel string    `json:"badge_level"`
	CreatedAt  time.Time `json:"created_at"`
}

type FriendEntry struct {
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	BadgeLevel  string    `json:"badge_level"`
	PointsTotal int       `json:"points_total"`
}
