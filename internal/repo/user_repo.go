package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/nomad-api/internal/domain"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUserAlreadyExists = errors.New("user already exists")

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	query := `
		INSERT INTO users (email, username, password_hash, birth_date, age, account_status, privacy_mode)
		VALUES ($1, $2, $3, $4, EXTRACT(YEAR FROM AGE($4::date))::INTEGER, $5, $6)
		RETURNING user_id, age, is_protected, points_total, badge_level, created_at, updated_at
	`

	err := r.db.QueryRow(ctx, query,
		u.Email, u.Username, u.PasswordHash, u.BirthDate, u.AccountStatus, u.PrivacyMode,
	).Scan(
		&u.UserID, &u.Age, &u.IsProtected, &u.PointsTotal, &u.BadgeLevel, &u.CreatedAt, &u.UpdatedAt,
	)

	return err
}

func (r *UserRepository) FindByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	query := `SELECT user_id, email, username, password_hash, birth_date, age, is_protected, account_status, privacy_mode, created_at, updated_at FROM users WHERE user_id = $1`

	var u domain.User
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&u.UserID, &u.Email, &u.Username, &u.PasswordHash, &u.BirthDate, &u.Age,
		&u.IsProtected, &u.AccountStatus, &u.PrivacyMode, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

// GetByID returns a user including their weekly streak fields, used by the
// check-in service to compute and persist the weekly streak/multiplier.
func (r *UserRepository) GetByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	query := `
		SELECT user_id, email, username, points_total,
		       streak_weeks_current, streak_weeks_max, streak_week_checkins,
		       streak_week_start, streak_last_checkin
		FROM users WHERE user_id = $1
	`

	var u domain.User
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&u.UserID, &u.Email, &u.Username, &u.PointsTotal,
		&u.StreakWeeksCurrent, &u.StreakWeeksMax, &u.StreakWeekCheckins,
		&u.StreakWeekStart, &u.StreakLastCheckin,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

// FindByUsername returns a user and their total check-in count.
// Returns ErrUserNotFound if the user does not exist or is not ACTIVE.
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*domain.User, int, error) {
	query := `
		SELECT u.user_id, u.username, u.badge_level, u.points_total,
		       u.streak_weeks_current, u.streak_weeks_max, u.privacy_mode, u.is_protected,
		       u.account_status, u.ranking_opt_in, u.created_at,
		       COUNT(ci.checkin_id) AS checkin_count
		FROM users u
		LEFT JOIN check_ins ci ON ci.user_id = u.user_id
		WHERE u.username = $1
		  AND u.account_status = 'ACTIVE'
		GROUP BY u.user_id
	`

	var u domain.User
	var checkInCount int
	err := r.db.QueryRow(ctx, query, username).Scan(
		&u.UserID, &u.Username, &u.BadgeLevel, &u.PointsTotal,
		&u.StreakWeeksCurrent, &u.StreakWeeksMax, &u.PrivacyMode, &u.IsProtected,
		&u.AccountStatus, &u.RankingOptIn, &u.CreatedAt,
		&checkInCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, 0, ErrUserNotFound
		}
		return nil, 0, err
	}
	return &u, checkInCount, nil
}

func (r *UserRepository) UpdatePrivacyMode(ctx context.Context, userID uuid.UUID, mode string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE users SET privacy_mode = $1, updated_at = NOW() WHERE user_id = $2
	`, mode, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateRankingOptIn sets the user's ranking_opt_in flag.
func (r *UserRepository) UpdateRankingOptIn(ctx context.Context, userID uuid.UUID, optIn bool) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE users SET ranking_opt_in = $1, updated_at = NOW() WHERE user_id = $2
	`, optIn, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SoftDelete anonymizes PII and marks the account as DELETED.
// check_ins and point_transactions rows are preserved.
func (r *UserRepository) SoftDelete(ctx context.Context, userID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE users SET
			account_status   = 'DELETED',
			email            = 'deleted_' || user_id || '@candela.deleted',
			username         = 'deleted_' || user_id,
			password_hash    = '',
			guardian_contact = NULL,
			guardian_token   = NULL,
			updated_at       = NOW()
		WHERE user_id = $1
		  AND account_status != 'DELETED'
	`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// GetMyProfile returns the authenticated user's full profile with aggregated counts.
// Sensitive fields (password_hash, guardian_token, guardian_contact) are excluded.
func (r *UserRepository) GetMyProfile(ctx context.Context, userID uuid.UUID) (*domain.MyProfile, error) {
	var p domain.MyProfile
	err := r.db.QueryRow(ctx, `
		SELECT
			u.user_id, u.email, u.username, u.birth_date, u.age,
			u.is_protected, u.account_status, u.privacy_mode,
			u.points_total, u.points_season, u.badge_level,
			u.streak_weeks_current, u.streak_weeks_max,
			u.streak_week_checkins, u.streak_week_start, u.streak_last_checkin,
			u.ranking_opt_in,
			u.created_at, u.updated_at,
			(SELECT COUNT(*) FROM check_ins ci
			 WHERE ci.user_id = $1 AND ci.validation_passed = true) AS check_in_count,
			(SELECT COUNT(*) FROM friendships f
			 WHERE (f.user_id_a = $1 OR f.user_id_b = $1) AND f.status = 'accepted') AS friends_count,
			(SELECT COUNT(*) FROM plans pl
			 WHERE (pl.creator_user_id = $1 OR pl.target_user_id = $1) AND pl.status = 'COMPLETED') AS completed_plans_count
		FROM users u
		WHERE u.user_id = $1
	`, userID).Scan(
		&p.UserID, &p.Email, &p.Username, &p.BirthDate, &p.Age,
		&p.IsProtected, &p.AccountStatus, &p.PrivacyMode,
		&p.PointsTotal, &p.PointsSeason, &p.BadgeLevel,
		&p.StreakWeeksCurrent, &p.StreakWeeksMax,
		&p.StreakWeekCheckins, &p.StreakWeekStart, &p.StreakLastCheckin,
		&p.RankingOptIn,
		&p.CreatedAt, &p.UpdatedAt,
		&p.CheckInCount, &p.FriendsCount, &p.CompletedPlansCount,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT user_id, email, username, password_hash, birth_date, age, account_status, privacy_mode, created_at, updated_at FROM users WHERE email = $1`

	var u domain.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.UserID, &u.Email, &u.Username, &u.PasswordHash, &u.BirthDate, &u.Age,
		&u.AccountStatus, &u.PrivacyMode, &u.CreatedAt, &u.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &u, nil
}
