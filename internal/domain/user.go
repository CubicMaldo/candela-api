package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	UserID             uuid.UUID  `json:"id"                db:"user_id"`
	Email              string     `json:"email"             db:"email"`
	Username           string     `json:"username"          db:"username"`
	PasswordHash       string     `json:"-"                 db:"password_hash"`
	BirthDate          time.Time  `json:"birth_date"        db:"birth_date"`
	IsProtected        bool       `json:"is_protected"      db:"is_protected"`
	AccountStatus      string     `json:"account_status"    db:"account_status"`
	PrivacyMode        string     `json:"privacy_mode"      db:"privacy_mode"`
	GuardianContact    *string    `json:"guardian_contact"  db:"guardian_contact"`
	GuardianVerified   bool       `json:"guardian_verified" db:"guardian_verified"`
	GuardianToken      *uuid.UUID `json:"-"                 db:"guardian_token"`
	PointsTotal        int        `json:"points_total"      db:"points_total"`
	PointsSeason       int        `json:"points_season"     db:"points_season"`
	BadgeLevel         string     `json:"badge_level"       db:"badge_level"`
	StreakWeeksCurrent int        `json:"streak_weeks_current" db:"streak_weeks_current"`
	StreakWeeksMax     int        `json:"streak_weeks_max"     db:"streak_weeks_max"`
	StreakWeekCheckins int        `json:"streak_week_checkins" db:"streak_week_checkins"`
	StreakWeekStart    *time.Time `json:"streak_week_start,omitempty"   db:"streak_week_start"`
	StreakLastCheckin  *time.Time `json:"streak_last_checkin,omitempty" db:"streak_last_checkin"`
	RankingOptIn       bool       `json:"ranking_opt_in"    db:"ranking_opt_in"`
	CreatedAt          time.Time  `json:"created_at"        db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"        db:"updated_at"`
}

// UserSearchEntry is one result of the username prefix search
// (GET /api/users?q=). Only ever exposes public, non-sensitive fields.
type UserSearchEntry struct {
	UserID     uuid.UUID `json:"user_id"`
	Username   string    `json:"username"`
	BadgeLevel string    `json:"badge_level"`
	IsFriend   bool      `json:"is_friend"`
}

// CalculateAge returns the user's current age in years based on BirthDate.
// It accounts for whether the birthday has occurred yet this calendar year.
func (u *User) CalculateAge(now time.Time) int {
	years := now.Year() - u.BirthDate.Year()
	// Subtract 1 if the birthday hasn't happened yet this year
	birthday := u.BirthDate.AddDate(years, 0, 0)
	if now.Before(birthday) {
		years--
	}
	return years
}

// IsProtectedMode reports whether the user requires parental-consent restrictions.
// A user is in protected mode when explicitly flagged OR when under 18.
func (u *User) IsProtectedMode(now time.Time) bool {
	return u.IsProtected || u.CalculateAge(now) < 18
}

type RegisterRequest struct {
	Email     string `json:"email"      binding:"required,email"`
	Username  string `json:"username"   binding:"required,min=3,max=50"`
	Password  string `json:"password"   binding:"required,min=8"`
	BirthDate string `json:"birth_date" binding:"required"` // YYYY-MM-DD
}

type AuthRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse is returned by every flow that establishes a session.
// Token is the short-lived access JWT; RefreshToken exchanges for a new pair
// via POST /api/auth/refresh once ExpiresIn seconds have passed.
type AuthResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	User         User   `json:"user"`
}

// RefreshRequest is the body of POST /api/auth/refresh and /api/auth/logout.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type GoogleAuthRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

// GoogleRegisterRequest completes registration for a Google account whose
// email has no Candela user yet. Email comes from the verified ID token,
// never from the client.
type GoogleRegisterRequest struct {
	IDToken   string `json:"id_token"   binding:"required"`
	Username  string `json:"username"   binding:"required,min=3,max=50"`
	BirthDate string `json:"birth_date" binding:"required"` // YYYY-MM-DD
}

// GoogleNeedsRegistration is returned by POST /api/auth/google when the
// Google email is valid but not registered: the client must collect the
// remaining profile fields and call POST /api/auth/google/register.
type GoogleNeedsRegistration struct {
	NeedsRegistration bool   `json:"needs_registration"`
	Email             string `json:"email"`
}
