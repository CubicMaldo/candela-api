package domain

import (
	"time"

	"github.com/google/uuid"
)

// FeedUser is the minimal user info embedded in feed events.
type FeedUser struct {
	UserID     uuid.UUID `json:"user_id"`
	Username   string    `json:"username"`
	BadgeLevel string    `json:"badge_level"`
	IsFriend   bool      `json:"is_friend"`
}

// FeedPlace is the minimal place info embedded in feed events.
type FeedPlace struct {
	PlaceID    uuid.UUID `json:"place_id"`
	Name       string    `json:"name"`
	Category   string    `json:"category"`
	IsFollowed bool      `json:"is_followed"`
}

// CheckInEvent is a feed item representing a check-in with photo.
// Points are private (spec §14.1) and are never exposed in feeds.
type CheckInEvent struct {
	Type             string    `json:"type"`
	EventID          uuid.UUID `json:"event_id"`
	Timestamp        time.Time `json:"timestamp"`
	PriorityLayer    int       `json:"priority_layer,omitempty"`
	User             FeedUser  `json:"user"`
	Place            FeedPlace `json:"place"`
	PhotoURL         *string   `json:"photo_url"`
	PhotoIsPermanent bool      `json:"photo_is_permanent"`
}

// MyProfile is the authenticated user's own profile, including aggregated counts.
// Sensitive fields (password_hash, guardian_token, guardian_contact) are excluded.
type MyProfile struct {
	UserID              uuid.UUID  `json:"user_id"`
	Email               string     `json:"email"`
	Username            string     `json:"username"`
	BirthDate           time.Time  `json:"birth_date"`
	Age                 int        `json:"age"`
	IsProtected         bool       `json:"is_protected"`
	AccountStatus       string     `json:"account_status"`
	PrivacyMode         string     `json:"privacy_mode"`
	PointsTotal         int        `json:"points_total"`
	PointsSeason        int        `json:"points_season"`
	BadgeLevel          string     `json:"badge_level"`
	StreakWeeksCurrent  int        `json:"streak_weeks_current"`
	StreakWeeksMax      int        `json:"streak_weeks_max"`
	StreakWeekCheckins  int        `json:"streak_week_checkins"`
	StreakWeekStart     *time.Time `json:"streak_week_start"`
	StreakLastCheckin   *time.Time `json:"streak_last_checkin"`
	RankingOptIn        bool       `json:"ranking_opt_in"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	CheckInCount        int        `json:"check_in_count"`
	FriendsCount        int        `json:"friends_count"`
	CompletedPlansCount int        `json:"completed_plans_count"`
}

// MyCheckIn is a single item in the user's own check-in history.
type MyCheckIn struct {
	CheckInID        uuid.UUID `json:"checkin_id"`
	Place            FeedPlace `json:"place"`
	PhotoURL         *string   `json:"photo_url"`
	PhotoIsPermanent bool      `json:"photo_is_permanent"`
	PointsAwarded    int       `json:"points_awarded"`
	IsFirstOfDay     bool      `json:"is_first_of_day"`
	CheckedInAt      time.Time `json:"checked_in_at"`
}

// MyPlanUser is embedded in MyPlan for creator and target info.
type MyPlanUser struct {
	UserID     uuid.UUID `json:"user_id"`
	Username   string    `json:"username"`
	BadgeLevel string    `json:"badge_level"`
}

// MyPlan is an item in the user's plan history, including role and participant info.
// Target is nil for open (PUBLIC/FRIENDS) plans, which have no single invitee.
type MyPlan struct {
	PlanID           uuid.UUID      `json:"plan_id"`
	Status           PlanStatus     `json:"status"`
	Visibility       PlanVisibility `json:"visibility"`
	Message          *string        `json:"message,omitempty"`
	DeadlineHours    int            `json:"deadline_hours"`
	CreatedAt        time.Time      `json:"created_at"`
	DeadlineAt       time.Time      `json:"deadline_at"`
	CompletedAt      *time.Time     `json:"concretado_at,omitempty"`
	Creator          MyPlanUser     `json:"creator"`
	Target           *MyPlanUser    `json:"target,omitempty"`
	Place            FeedPlace      `json:"place"`
	Role             string         `json:"role"`
	ParticipantCount int            `json:"participant_count"`
	MyStatus         string         `json:"my_status,omitempty"`
}
