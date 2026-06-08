package domain

import (
	"time"

	"github.com/google/uuid"
)

type PlanStatus string

const (
	PlanPending   PlanStatus = "PENDING"
	PlanAccepted  PlanStatus = "ACCEPTED"
	PlanCompleted PlanStatus = "COMPLETED"
	PlanFailed    PlanStatus = "FAILED"
	PlanDeclined  PlanStatus = "DECLINED"
)

type Plan struct {
	PlanID        uuid.UUID  `json:"plan_id"               db:"plan_id"`
	CreatorUserID uuid.UUID  `json:"creator_user_id"       db:"creator_user_id"`
	TargetUserID  uuid.UUID  `json:"target_user_id"        db:"target_user_id"`
	PlaceID       uuid.UUID  `json:"place_id"              db:"place_id"`
	Status        PlanStatus `json:"status"                db:"status"`
	Message       *string    `json:"message"               db:"message"`
	DeadlineHours int        `json:"deadline_hours"        db:"deadline_hours"`
	DeadlineAt    time.Time  `json:"deadline_at"           db:"deadline_at"`
	CompletedAt   *time.Time `json:"completed_at"          db:"completed_at"`
	CreatedAt     time.Time  `json:"created_at"            db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"            db:"updated_at"`
}

type PlanDetail struct {
	Plan
	CreatorUsername string `json:"creator_username"`
	PlaceName       string `json:"place_name"`
}

type CreatePlanRequest struct {
	TargetUserID  uuid.UUID `json:"target_user_id"  binding:"required"`
	PlaceID       uuid.UUID `json:"place_id"        binding:"required"`
	DeadlineHours int       `json:"deadline_hours"  binding:"required,oneof=24 48 72"`
	Message       *string   `json:"message"`
}
