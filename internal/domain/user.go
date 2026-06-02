package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	UserID        uuid.UUID `json:"id" db:"user_id"`
	Email         string    `json:"email" db:"email"`
	Username      string    `json:"username" db:"username"`
	PasswordHash  string    `json:"-" db:"password_hash"`
	BirthDate     time.Time `json:"birth_date" db:"birth_date"`
	Age           int       `json:"age" db:"age"`
	IsProtected   bool      `json:"is_protected" db:"is_protected"`
	AccountStatus string    `json:"account_status" db:"account_status"`
	PrivacyMode   string    `json:"privacy_mode" db:"privacy_mode"`
	PointsTotal   int       `json:"points_total" db:"points_total"`
	PointsSeason  int       `json:"points_season" db:"points_season"`
	BadgeLevel    string    `json:"badge_level" db:"badge_level"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

type AuthRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Email     string    `json:"email" binding:"required,email"`
	Username  string    `json:"username" binding:"required,min=3"`
	Password  string    `json:"password" binding:"required,min=6"`
	BirthDate time.Time `json:"birth_date" binding:"required" time_format:"2006-01-02"`
}

type GoogleAuthRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
