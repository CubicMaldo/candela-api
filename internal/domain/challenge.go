package domain

import (
	"time"

	"github.com/google/uuid"
)

type Challenge struct {
	ChallengeID         uuid.UUID  `json:"challenge_id"          db:"challenge_id"`
	CreatorUserID       uuid.UUID  `json:"creator_user_id"       db:"creator_user_id"`
	TargetUserID        uuid.UUID  `json:"target_user_id"        db:"target_user_id"`
	PlaceID             uuid.UUID  `json:"place_id"              db:"place_id"`
	DeadlineHours       int        `json:"deadline_hours"        db:"deadline_hours"`
	Status              string     `json:"status"                db:"status"`
	Message             *string    `json:"message"               db:"message"`
	CreatedAt           time.Time  `json:"created_at"            db:"created_at"`
	DeadlineAt          time.Time  `json:"deadline_at"           db:"deadline_at"`
	CompletedAt         *time.Time `json:"completed_at"          db:"completed_at"`
	CompletingCheckinID *uuid.UUID `json:"completing_checkin_id" db:"completing_checkin_id"`
	PointsCreator       *int       `json:"points_creator"        db:"points_creator"`
	PointsTarget        *int       `json:"points_target"         db:"points_target"`
}

type ChallengeDetail struct {
	Challenge
	CreatorUsername string `json:"creator_username"`
	PlaceName       string `json:"place_name"`
}

type CreateChallengeRequest struct {
	TargetUserID  uuid.UUID `json:"target_user_id"  binding:"required"`
	PlaceID       uuid.UUID `json:"place_id"        binding:"required"`
	DeadlineHours int       `json:"deadline_hours"  binding:"required,oneof=24 48 72"`
	Message       *string   `json:"message"`
}
