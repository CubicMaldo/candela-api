package domain

import (
	"encoding/json"
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

// planStatusES maps internal/DB status values to the Spanish values of the
// public API contract (CANDELA_API_CHANGELOG.md §6.2).
var planStatusES = map[PlanStatus]string{
	PlanPending:   "PENDIENTE",
	PlanAccepted:  "ACEPTADO",
	PlanCompleted: "CONCRETADO",
	PlanFailed:    "NO_CONCRETADO",
	PlanDeclined:  "DECLINADO",
}

func (s PlanStatus) MarshalJSON() ([]byte, error) {
	if es, ok := planStatusES[s]; ok {
		return json.Marshal(es)
	}
	return json.Marshal(string(s))
}

// PlanVisibility controls who can see and join a plan.
//
//	PUBLIC  — anyone can discover and join (no target).
//	FRIENDS — only the creator's friends can discover and join (no target).
//	INVITE  — a single friend is invited directly (target_user_id set); the
//	          classic 1:1 flow.
type PlanVisibility string

const (
	PlanVisibilityPublic  PlanVisibility = "PUBLIC"
	PlanVisibilityFriends PlanVisibility = "FRIENDS"
	PlanVisibilityInvite  PlanVisibility = "INVITE"
)

// ParticipantStatus is the per-participant state within a plan (plan_participants).
type ParticipantStatus string

const (
	ParticipantInvited   ParticipantStatus = "INVITED"
	ParticipantJoined    ParticipantStatus = "JOINED"
	ParticipantCompleted ParticipantStatus = "COMPLETED"
	ParticipantDeclined  ParticipantStatus = "DECLINED"
)

// ParticipantRole distinguishes the plan creator from guests who join.
type ParticipantRole string

const (
	ParticipantRoleCreator ParticipantRole = "CREATOR"
	ParticipantRoleGuest   ParticipantRole = "GUEST"
)

type Plan struct {
	PlanID        uuid.UUID      `json:"plan_id"               db:"plan_id"`
	CreatorUserID uuid.UUID      `json:"creator_user_id"       db:"creator_user_id"`
	TargetUserID  *uuid.UUID     `json:"target_user_id"        db:"target_user_id"`
	PlaceID       uuid.UUID      `json:"place_id"              db:"place_id"`
	Visibility    PlanVisibility `json:"visibility"            db:"visibility"`
	Status        PlanStatus     `json:"status"                db:"status"`
	Message       *string        `json:"message"               db:"message"`
	DeadlineHours int            `json:"deadline_hours"        db:"deadline_hours"`
	DeadlineAt    time.Time      `json:"deadline_at"           db:"deadline_at"`
	CompletedAt   *time.Time     `json:"concretado_at"         db:"completed_at"`
	CreatedAt     time.Time      `json:"created_at"            db:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"            db:"updated_at"`
}

type PlanDetail struct {
	Plan
	CreatorUsername string `json:"creator_username"`
	PlaceName       string `json:"place_name"`
}

// DiscoverPlansRequest carries the map's center coordinates and search radius.
type DiscoverPlansRequest struct {
	Lat      float64 `form:"lat" binding:"required"`
	Lon      float64 `form:"lon" binding:"required"`
	RadiusKM float64 `form:"radius_km,default=5.0"`
}

// DiscoverPlace is a place with coordinates, for rendering plan pins on the map.
type DiscoverPlace struct {
	PlaceID  uuid.UUID `json:"place_id"`
	Name     string    `json:"name"`
	Category string    `json:"category"`
	Lat      float64   `json:"lat"`
	Lon      float64   `json:"lon"`
}

// DiscoverPlan is an open plan the viewer can join, surfaced on the map. Carries
// the place coordinates and a participant count; no target (open plans have none).
type DiscoverPlan struct {
	PlanID           uuid.UUID      `json:"plan_id"`
	Visibility       PlanVisibility `json:"visibility"`
	Status           PlanStatus     `json:"status"`
	Message          *string        `json:"message,omitempty"`
	DeadlineHours    int            `json:"deadline_hours"`
	DeadlineAt       time.Time      `json:"deadline_at"`
	CreatedAt        time.Time      `json:"created_at"`
	Creator          MyPlanUser     `json:"creator"`
	Place            DiscoverPlace  `json:"place"`
	ParticipantCount int            `json:"participant_count"`
}

// CreatePlanRequest carries a plan proposal. TargetUserID is required only for
// INVITE visibility; PUBLIC/FRIENDS plans are open and leave it nil. An empty
// Visibility defaults to INVITE for backward compatibility.
type CreatePlanRequest struct {
	TargetUserID  *uuid.UUID     `json:"target_user_id"`
	PlaceID       uuid.UUID      `json:"place_id"        binding:"required"`
	Visibility    PlanVisibility `json:"visibility"      binding:"omitempty,oneof=PUBLIC FRIENDS INVITE"`
	DeadlineHours int            `json:"deadline_hours"  binding:"required,oneof=24 48 72"`
	Message       *string        `json:"message"`
}
