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
}

// FeedPlace is the minimal place info embedded in feed events.
type FeedPlace struct {
	PlaceID  uuid.UUID `json:"place_id"`
	Name     string    `json:"name"`
	Category string    `json:"category"`
}

// CheckInEvent is a feed item representing a friend's check-in.
type CheckInEvent struct {
	Type          string    `json:"type"`
	EventID       uuid.UUID `json:"event_id"`
	Timestamp     time.Time `json:"timestamp"`
	User          FeedUser  `json:"user"`
	Place         FeedPlace `json:"place"`
	PhotoURL      *string   `json:"photo_url"`
	PointsAwarded int       `json:"points_awarded"`
}

// ChallengeCompletedEvent is a feed item representing a completed challenge involving friends.
type ChallengeCompletedEvent struct {
	Type      string    `json:"type"`
	EventID   uuid.UUID `json:"event_id"`
	Timestamp time.Time `json:"timestamp"`
	Creator   FeedUser  `json:"creator"`
	Target    FeedUser  `json:"target"`
	Place     FeedPlace `json:"place"`
}

// MyProfile is the authenticated user's own profile, including aggregated counts.
// Sensitive fields (password_hash, guardian_token, guardian_contact) are excluded.
type MyProfile struct {
	UserID                   uuid.UUID  `json:"user_id"`
	Email                    string     `json:"email"`
	Username                 string     `json:"username"`
	BirthDate                time.Time  `json:"birth_date"`
	Age                      int        `json:"age"`
	IsProtected              bool       `json:"is_protected"`
	AccountStatus            string     `json:"account_status"`
	PrivacyMode              string     `json:"privacy_mode"`
	GuardianVerified         bool       `json:"guardian_verified"`
	PointsTotal              int        `json:"points_total"`
	PointsSeason             int        `json:"points_season"`
	BadgeLevel               string     `json:"badge_level"`
	StreakCurrent            int        `json:"streak_current"`
	StreakMax                int        `json:"streak_max"`
	StreakLastCheckin         *time.Time `json:"streak_last_checkin"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
	CheckInCount             int        `json:"check_in_count"`
	FriendsCount             int        `json:"friends_count"`
	CompletedChallengesCount int        `json:"completed_challenges_count"`
}

// MyCheckIn is a single item in the user's own check-in history.
type MyCheckIn struct {
	CheckInID     uuid.UUID `json:"checkin_id"`
	Place         FeedPlace `json:"place"`
	PhotoURL      *string   `json:"photo_url"`
	PointsAwarded int       `json:"points_awarded"`
	IsFirstOfDay  bool      `json:"is_first_of_day"`
	CheckedInAt   time.Time `json:"checked_in_at"`
}

// MyChallengeUser is embedded in MyChallenge for creator and target info.
type MyChallengeUser struct {
	UserID     uuid.UUID `json:"user_id"`
	Username   string    `json:"username"`
	BadgeLevel string    `json:"badge_level"`
}

// MyChallenge is an item in the user's challenge history, including role and participant info.
type MyChallenge struct {
	ChallengeID   uuid.UUID       `json:"challenge_id"`
	Status        string          `json:"status"`
	Message       *string         `json:"message,omitempty"`
	DeadlineHours int             `json:"deadline_hours"`
	CreatedAt     time.Time       `json:"created_at"`
	DeadlineAt    time.Time       `json:"deadline_at"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
	PointsCreator *int            `json:"points_creator,omitempty"`
	PointsTarget  *int            `json:"points_target,omitempty"`
	Creator       MyChallengeUser `json:"creator"`
	Target        MyChallengeUser `json:"target"`
	Place         FeedPlace       `json:"place"`
	Role          string          `json:"role"`
}
