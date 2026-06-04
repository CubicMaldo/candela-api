package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	UserID           uuid.UUID  `json:"id"                db:"user_id"`
	Email            string     `json:"email"             db:"email"`
	Username         string     `json:"username"          db:"username"`
	PasswordHash     string     `json:"-"                 db:"password_hash"`
	BirthDate        time.Time  `json:"birth_date"        db:"birth_date"`
	Age              int        `json:"age"               db:"age"`
	IsProtected      bool       `json:"is_protected"      db:"is_protected"`
	AccountStatus    string     `json:"account_status"    db:"account_status"`
	PrivacyMode      string     `json:"privacy_mode"      db:"privacy_mode"`
	GuardianContact  *string    `json:"guardian_contact"  db:"guardian_contact"`
	GuardianVerified bool       `json:"guardian_verified" db:"guardian_verified"`
	GuardianToken    *uuid.UUID `json:"-"                 db:"guardian_token"`
	PointsTotal      int        `json:"points_total"      db:"points_total"`
	PointsSeason     int        `json:"points_season"     db:"points_season"`
	BadgeLevel       string     `json:"badge_level"       db:"badge_level"`
	StreakCurrent    int        `json:"streak_current"    db:"streak_current"`
	StreakMax        int        `json:"streak_max"        db:"streak_max"`
	StreakLastCheckin *time.Time `json:"streak_last_checkin" db:"streak_last_checkin"`
	CreatedAt        time.Time  `json:"created_at"        db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"        db:"updated_at"`
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

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type GoogleAuthRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}
